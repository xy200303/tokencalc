package provider

import (
	"strings"

	"github.com/xy200303/tokencalc/internal/text"
)

type openAIResponsesEstimator struct {
	policy text.PlaceholderPolicy
}

func NewOpenAIResponses(policy PlaceholderPolicy) Estimator {
	return openAIResponsesEstimator{policy: buildPolicy(policy)}
}

func (e openAIResponsesEstimator) PrepareRequest(body []byte) (RequestPayload, error) {
	return PrepareRequestObject(body)
}

func (e openAIResponsesEstimator) PrepareResponse(body []byte, isStream bool) (ResponsePayload, error) {
	return PrepareResponseObject(body, isStream)
}

func (e openAIResponsesEstimator) ExtractPrompt(body []byte) (ExtractResult, error) {
	payload, err := e.PrepareRequest(body)
	if err != nil {
		return ExtractResult{}, err
	}
	return e.ExtractPromptPrepared(payload)
}

func (e openAIResponsesEstimator) ExtractPromptPrepared(payload RequestPayload) (ExtractResult, error) {
	if len(payload.Object) == 0 {
		return ExtractResult{Supported: true, Note: "request body missing"}, nil
	}

	builder := text.NewBuilder()
	if instructions, ok := payload.Object["instructions"]; ok {
		appendGenericContent(builder, instructions, e.policy)
	}
	if input, ok := payload.Object["input"]; ok {
		appendGenericContent(builder, input, e.policy)
	} else {
		builder.AddNote("input missing")
	}
	if tools, ok := payload.Object["tools"]; ok {
		builder.AddJSON(tools)
	}
	if textConfig, ok := payload.Object["text"]; ok {
		builder.AddJSON(textConfig)
	}

	return resultFromBuilder(builder), nil
}

func (e openAIResponsesEstimator) ExtractCompletion(body []byte, isStream bool) (ExtractResult, error) {
	payload, err := e.PrepareResponse(body, isStream)
	if err != nil {
		return ExtractResult{}, err
	}
	return e.ExtractCompletionPrepared(payload, isStream)
}

func (e openAIResponsesEstimator) ExtractCompletionPrepared(payload ResponsePayload, isStream bool) (ExtractResult, error) {
	if len(payload.Object) == 0 && len(payload.Events) == 0 {
		return ExtractResult{Supported: true, Note: "response body missing, only prompt estimated"}, nil
	}

	if isStream {
		return e.extractStreamPrepared(payload)
	}

	builder := text.NewBuilder()
	if output, ok := payload.Object["output"]; ok {
		appendGenericContent(builder, output, e.policy)
	}
	if outputText, ok := payload.Object["output_text"]; ok {
		appendGenericContent(builder, outputText, e.policy)
	}
	if response, ok := payload.Object["response"]; ok {
		appendGenericContent(builder, response, e.policy)
	}
	if len(builder.ResultText()) == 0 {
		builder.AddNote("output missing")
	}

	return resultFromBuilder(builder), nil
}

func (e openAIResponsesEstimator) ExtractReportedUsage(body []byte, isStream bool) (ReportedUsageResult, error) {
	payload, err := e.PrepareResponse(body, isStream)
	if err != nil {
		return ReportedUsageResult{}, err
	}
	return e.ExtractReportedUsagePrepared(payload, isStream)
}

func (e openAIResponsesEstimator) ExtractReportedUsagePrepared(payload ResponsePayload, isStream bool) (ReportedUsageResult, error) {
	if len(payload.Object) == 0 && len(payload.Events) == 0 {
		return ReportedUsageResult{}, nil
	}

	if !isStream {
		usage := normalizeReportedUsage(extractOpenAIResponsesUsage(payload.Object))
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
		usage = mergeReportedUsage(usage, extractOpenAIResponsesUsage(event))
		if response, ok := text.AsMap(event["response"]); ok {
			usage = mergeReportedUsage(usage, extractOpenAIResponsesUsage(response))
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

func (e openAIResponsesEstimator) extractStreamPrepared(payload ResponsePayload) (ExtractResult, error) {
	builder := text.NewBuilder()
	for _, event := range payload.Events {
		eventType := strings.ToLower(text.StringValue(event["type"]))
		switch eventType {
		case "response.output_text.delta":
			builder.AddText(text.FirstString(event, "delta", "text"))
		case "response.output_text.done":
			builder.AddText(text.FirstString(event, "text"))
		case "response.completed":
			if response, ok := event["response"]; ok {
				appendGenericContent(builder, response, e.policy)
			}
		default:
			appendGenericContent(builder, event, e.policy)
		}
	}

	result := resultFromBuilder(builder)
	if result.Text == "" && result.ExtraTokens == 0 {
		result.Note = joinProviderNotes(result.Note, "stream contained no extractable responses deltas")
	}
	return result, nil
}

func (e openAIResponsesEstimator) ExtractRequestModelPrepared(payload RequestPayload) string {
	return findModelInObject(payload.Object, [][]string{{"model"}})
}

func (e openAIResponsesEstimator) ExtractResponseModelPrepared(payload ResponsePayload, isStream bool) string {
	paths := [][]string{{"model"}, {"response", "model"}}
	if !isStream {
		return findModelInObject(payload.Object, paths)
	}
	return findModelInEvents(payload.Events, paths)
}

func extractOpenAIResponsesUsage(mapped map[string]any) ReportedUsage {
	usageMap, ok := text.AsMap(mapped["usage"])
	if !ok {
		return ReportedUsage{}
	}

	usage := ReportedUsage{
		PromptTokens:     intValue(usageMap["input_tokens"]),
		CompletionTokens: intValue(usageMap["output_tokens"]),
		TotalTokens:      intValue(usageMap["total_tokens"]),
	}

	if usage.PromptTokens == 0 {
		usage.PromptTokens = intValue(usageMap["prompt_tokens"])
	}
	if usage.CompletionTokens == 0 {
		usage.CompletionTokens = intValue(usageMap["completion_tokens"])
	}
	return usage
}
