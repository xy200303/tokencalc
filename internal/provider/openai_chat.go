package provider

import "github.com/xy200303/tokencalc/internal/text"

type openAIChatEstimator struct {
	policy text.PlaceholderPolicy
}

func NewOpenAIChat(policy PlaceholderPolicy) Estimator {
	return openAIChatEstimator{policy: buildPolicy(policy)}
}

func (e openAIChatEstimator) PrepareRequest(body []byte) (RequestPayload, error) {
	return PrepareRequestObject(body)
}

func (e openAIChatEstimator) PrepareResponse(body []byte, isStream bool) (ResponsePayload, error) {
	return PrepareResponseObject(body, isStream)
}

func (e openAIChatEstimator) ExtractPrompt(body []byte) (ExtractResult, error) {
	payload, err := e.PrepareRequest(body)
	if err != nil {
		return ExtractResult{}, err
	}
	return e.ExtractPromptPrepared(payload)
}

func (e openAIChatEstimator) ExtractPromptPrepared(payload RequestPayload) (ExtractResult, error) {
	if len(payload.Object) == 0 {
		return ExtractResult{Supported: true, Note: "request body missing"}, nil
	}

	builder := text.NewBuilder()
	if messages, ok := payload.Object["messages"]; ok {
		appendGenericContent(builder, messages, e.policy)
	} else {
		builder.AddNote("messages missing")
	}

	if system, ok := payload.Object["system"]; ok {
		appendGenericContent(builder, system, e.policy)
	}
	if tools, ok := payload.Object["tools"]; ok {
		builder.AddJSON(tools)
	}
	if responseFormat, ok := payload.Object["response_format"]; ok {
		builder.AddJSON(responseFormat)
	}

	return resultFromBuilder(builder), nil
}

func (e openAIChatEstimator) ExtractCompletion(body []byte, isStream bool) (ExtractResult, error) {
	payload, err := e.PrepareResponse(body, isStream)
	if err != nil {
		return ExtractResult{}, err
	}
	return e.ExtractCompletionPrepared(payload, isStream)
}

func (e openAIChatEstimator) ExtractCompletionPrepared(payload ResponsePayload, isStream bool) (ExtractResult, error) {
	if len(payload.Object) == 0 && len(payload.Events) == 0 {
		return ExtractResult{Supported: true, Note: "response body missing, only prompt estimated"}, nil
	}

	if isStream {
		return e.extractStreamPrepared(payload)
	}

	builder := text.NewBuilder()
	if choices, ok := payload.Object["choices"]; ok {
		appendGenericContent(builder, choices, e.policy)
	} else if message, ok := payload.Object["message"]; ok {
		appendGenericContent(builder, message, e.policy)
	} else {
		builder.AddNote("choices missing")
	}

	return resultFromBuilder(builder), nil
}

func (e openAIChatEstimator) ExtractReportedUsage(body []byte, isStream bool) (ReportedUsageResult, error) {
	payload, err := e.PrepareResponse(body, isStream)
	if err != nil {
		return ReportedUsageResult{}, err
	}
	return e.ExtractReportedUsagePrepared(payload, isStream)
}

func (e openAIChatEstimator) ExtractReportedUsagePrepared(payload ResponsePayload, isStream bool) (ReportedUsageResult, error) {
	if len(payload.Object) == 0 && len(payload.Events) == 0 {
		return ReportedUsageResult{}, nil
	}

	if !isStream {
		usage := normalizeReportedUsage(extractOpenAIUsage(payload.Object))
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
		usage = mergeReportedUsage(usage, extractOpenAIUsage(event))
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

func (e openAIChatEstimator) extractStreamPrepared(payload ResponsePayload) (ExtractResult, error) {
	builder := text.NewBuilder()
	for _, event := range payload.Events {
		if choices, ok := event["choices"]; ok {
			appendGenericContent(builder, choices, e.policy)
		}
	}

	result := resultFromBuilder(builder)
	if result.Text == "" && result.ExtraTokens == 0 {
		result.Note = joinProviderNotes(result.Note, "stream contained no extractable chat deltas")
	}
	return result, nil
}

func (e openAIChatEstimator) ExtractRequestModelPrepared(payload RequestPayload) string {
	return findModelInObject(payload.Object, [][]string{{"model"}})
}

func (e openAIChatEstimator) ExtractResponseModelPrepared(payload ResponsePayload, isStream bool) string {
	if !isStream {
		return findModelInObject(payload.Object, [][]string{{"model"}})
	}
	return findModelInEvents(payload.Events, [][]string{{"model"}})
}

func extractOpenAIUsage(mapped map[string]any) ReportedUsage {
	usageMap, ok := text.AsMap(mapped["usage"])
	if !ok {
		return ReportedUsage{}
	}

	return ReportedUsage{
		PromptTokens:     intValue(usageMap["prompt_tokens"]),
		CompletionTokens: intValue(usageMap["completion_tokens"]),
		TotalTokens:      intValue(usageMap["total_tokens"]),
	}
}
