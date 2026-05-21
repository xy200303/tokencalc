package provider

import (
	"strings"

	"github.com/xy200303/tokencalc/internal/text"
)

type anthropicEstimator struct {
	policy text.PlaceholderPolicy
}

func NewAnthropic(policy PlaceholderPolicy) Estimator {
	return anthropicEstimator{policy: buildPolicy(policy)}
}

func (e anthropicEstimator) NewStreamAccumulator() StreamAccumulator {
	return &anthropicStreamAccumulator{
		baseStreamAccumulator: newBaseStreamAccumulator(
			"reported usage extracted from stream events",
			"stream contained no extractable anthropic deltas",
		),
		policy: e.policy,
	}
}

func (e anthropicEstimator) PrepareRequest(body []byte) (RequestPayload, error) {
	return PrepareRequestObject(body)
}

func (e anthropicEstimator) PrepareResponse(body []byte, isStream bool) (ResponsePayload, error) {
	return PrepareResponseObject(body, isStream)
}

func (e anthropicEstimator) ExtractPrompt(body []byte) (ExtractResult, error) {
	payload, err := e.PrepareRequest(body)
	if err != nil {
		return ExtractResult{}, err
	}
	return e.ExtractPromptPrepared(payload)
}

func (e anthropicEstimator) ExtractPromptPrepared(payload RequestPayload) (ExtractResult, error) {
	if len(payload.Object) == 0 {
		return ExtractResult{Supported: true, Note: "request body missing"}, nil
	}

	builder := text.NewBuilder()
	if system, ok := payload.Object["system"]; ok {
		appendGenericContent(builder, system, e.policy)
	}
	if messages, ok := payload.Object["messages"]; ok {
		appendGenericContent(builder, messages, e.policy)
	} else {
		builder.AddNote("messages missing")
	}
	if tools, ok := payload.Object["tools"]; ok {
		builder.AddJSON(tools)
	}

	return resultFromBuilder(builder), nil
}

func (e anthropicEstimator) ExtractCompletion(body []byte, isStream bool) (ExtractResult, error) {
	payload, err := e.PrepareResponse(body, isStream)
	if err != nil {
		return ExtractResult{}, err
	}
	return e.ExtractCompletionPrepared(payload, isStream)
}

func (e anthropicEstimator) ExtractCompletionPrepared(payload ResponsePayload, isStream bool) (ExtractResult, error) {
	if len(payload.Object) == 0 && len(payload.Events) == 0 {
		return ExtractResult{Supported: true, Note: "response body missing, only prompt estimated"}, nil
	}

	if isStream {
		return e.extractStreamPrepared(payload)
	}

	builder := text.NewBuilder()
	if content, ok := payload.Object["content"]; ok {
		appendGenericContent(builder, content, e.policy)
	} else {
		builder.AddNote("content missing")
	}

	return resultFromBuilder(builder), nil
}

func (e anthropicEstimator) ExtractReportedUsage(body []byte, isStream bool) (ReportedUsageResult, error) {
	payload, err := e.PrepareResponse(body, isStream)
	if err != nil {
		return ReportedUsageResult{}, err
	}
	return e.ExtractReportedUsagePrepared(payload, isStream)
}

func (e anthropicEstimator) ExtractReportedUsagePrepared(payload ResponsePayload, isStream bool) (ReportedUsageResult, error) {
	if len(payload.Object) == 0 && len(payload.Events) == 0 {
		return ReportedUsageResult{}, nil
	}

	if !isStream {
		usage := normalizeReportedUsage(extractAnthropicUsage(payload.Object))
		if !usage.HasAny() {
			return ReportedUsageResult{}, nil
		}
		return ReportedUsageResult{
			Usage: usage,
			Note:  "reported usage extracted from response body",
		}, nil
	}

	usage := ReportedUsage{}
	for _, event := range payload.Events {
		usage = mergeReportedUsage(usage, extractAnthropicUsage(event))
		if message, ok := text.AsMap(event["message"]); ok {
			usage = mergeReportedUsage(usage, extractAnthropicUsage(message))
		}
	}
	usage = normalizeReportedUsage(usage)
	if !usage.HasAny() {
		return ReportedUsageResult{}, nil
	}
	return ReportedUsageResult{
		Usage: usage,
		Note:  "reported usage extracted from stream events",
	}, nil
}

func (e anthropicEstimator) extractStreamPrepared(payload ResponsePayload) (ExtractResult, error) {
	builder := text.NewBuilder()
	for _, event := range payload.Events {
		eventType := strings.ToLower(text.StringValue(event["type"]))
		switch eventType {
		case "content_block_delta":
			if delta, ok := text.AsMap(event["delta"]); ok {
				appendGenericContent(builder, delta, e.policy)
			}
		case "content_block_start":
			if block, ok := text.AsMap(event["content_block"]); ok {
				appendGenericContent(builder, block, e.policy)
			}
		default:
			appendGenericContent(builder, event, e.policy)
		}
	}

	result := resultFromBuilder(builder)
	if result.Text == "" && result.ExtraTokens == 0 {
		result.Note = joinProviderNotes(result.Note, "stream contained no extractable anthropic deltas")
	}
	return result, nil
}

func (e anthropicEstimator) ExtractRequestModelPrepared(payload RequestPayload) string {
	return findModelInObject(payload.Object, [][]string{{"model"}})
}

func (e anthropicEstimator) ExtractResponseModelPrepared(payload ResponsePayload, isStream bool) string {
	paths := [][]string{{"model"}, {"message", "model"}}
	if !isStream {
		return findModelInObject(payload.Object, paths)
	}
	return findModelInEvents(payload.Events, paths)
}

func extractAnthropicUsage(mapped map[string]any) ReportedUsage {
	usageMap, ok := text.AsMap(mapped["usage"])
	if !ok {
		return ReportedUsage{}
	}

	return ReportedUsage{
		PromptTokens:     intValue(usageMap["input_tokens"]),
		CompletionTokens: intValue(usageMap["output_tokens"]),
		TotalTokens:      intValue(usageMap["total_tokens"]),
	}
}

type anthropicStreamAccumulator struct {
	baseStreamAccumulator
	policy text.PlaceholderPolicy
}

func (a *anthropicStreamAccumulator) AddEvent(event map[string]any) {
	a.setModelIfEmpty(findModelInObject(event, [][]string{{"model"}, {"message", "model"}}))
	a.mergeUsage(extractAnthropicUsage(event))
	if message, ok := text.AsMap(event["message"]); ok {
		a.mergeUsage(extractAnthropicUsage(message))
	}

	eventType := strings.ToLower(text.StringValue(event["type"]))
	switch eventType {
	case "content_block_delta":
		if delta, ok := text.AsMap(event["delta"]); ok {
			appendGenericContent(a.builder, delta, a.policy)
		}
	case "content_block_start":
		if block, ok := text.AsMap(event["content_block"]); ok {
			appendGenericContent(a.builder, block, a.policy)
		}
	default:
		appendGenericContent(a.builder, event, a.policy)
	}
}
