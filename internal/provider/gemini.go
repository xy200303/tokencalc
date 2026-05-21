package provider

import "github.com/xy200303/tokencalc/internal/text"

type geminiEstimator struct {
	policy text.PlaceholderPolicy
}

func NewGemini(policy PlaceholderPolicy) Estimator {
	return geminiEstimator{policy: buildPolicy(policy)}
}

func (e geminiEstimator) PrepareRequest(body []byte) (RequestPayload, error) {
	return PrepareRequestObject(body)
}

func (e geminiEstimator) PrepareResponse(body []byte, isStream bool) (ResponsePayload, error) {
	return PrepareResponseObject(body, isStream)
}

func (e geminiEstimator) ExtractPrompt(body []byte) (ExtractResult, error) {
	payload, err := e.PrepareRequest(body)
	if err != nil {
		return ExtractResult{}, err
	}
	return e.ExtractPromptPrepared(payload)
}

func (e geminiEstimator) ExtractPromptPrepared(payload RequestPayload) (ExtractResult, error) {
	if len(payload.Object) == 0 {
		return ExtractResult{Supported: true, Note: "request body missing"}, nil
	}

	builder := text.NewBuilder()
	if system, ok := payload.Object["systemInstruction"]; ok {
		appendGenericContent(builder, system, e.policy)
	}
	if contents, ok := payload.Object["contents"]; ok {
		appendGenericContent(builder, contents, e.policy)
	} else {
		builder.AddNote("contents missing")
	}
	if tools, ok := payload.Object["tools"]; ok {
		builder.AddJSON(tools)
	}

	return resultFromBuilder(builder), nil
}

func (e geminiEstimator) ExtractCompletion(body []byte, isStream bool) (ExtractResult, error) {
	payload, err := e.PrepareResponse(body, isStream)
	if err != nil {
		return ExtractResult{}, err
	}
	return e.ExtractCompletionPrepared(payload, isStream)
}

func (e geminiEstimator) ExtractCompletionPrepared(payload ResponsePayload, isStream bool) (ExtractResult, error) {
	if len(payload.Object) == 0 && len(payload.Events) == 0 {
		return ExtractResult{Supported: true, Note: "response body missing, only prompt estimated"}, nil
	}

	if isStream {
		return e.extractStreamPrepared(payload)
	}

	builder := text.NewBuilder()
	if candidates, ok := payload.Object["candidates"]; ok {
		appendGenericContent(builder, candidates, e.policy)
	} else {
		builder.AddNote("candidates missing")
	}

	return resultFromBuilder(builder), nil
}

func (e geminiEstimator) ExtractReportedUsage(body []byte, isStream bool) (ReportedUsageResult, error) {
	payload, err := e.PrepareResponse(body, isStream)
	if err != nil {
		return ReportedUsageResult{}, err
	}
	return e.ExtractReportedUsagePrepared(payload, isStream)
}

func (e geminiEstimator) ExtractReportedUsagePrepared(payload ResponsePayload, isStream bool) (ReportedUsageResult, error) {
	if len(payload.Object) == 0 && len(payload.Events) == 0 {
		return ReportedUsageResult{}, nil
	}

	if !isStream {
		usage := normalizeReportedUsage(extractGeminiUsage(payload.Object))
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
		usage = mergeReportedUsage(usage, extractGeminiUsage(event))
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

func (e geminiEstimator) extractStreamPrepared(payload ResponsePayload) (ExtractResult, error) {
	builder := text.NewBuilder()
	for _, event := range payload.Events {
		appendGenericContent(builder, event, e.policy)
	}

	result := resultFromBuilder(builder)
	if result.Text == "" && result.ExtraTokens == 0 {
		result.Note = joinProviderNotes(result.Note, "stream contained no extractable gemini deltas")
	}
	return result, nil
}

func (e geminiEstimator) ExtractRequestModelPrepared(payload RequestPayload) string {
	return findModelInObject(payload.Object, [][]string{{"model"}})
}

func (e geminiEstimator) ExtractResponseModelPrepared(payload ResponsePayload, isStream bool) string {
	paths := [][]string{{"model"}, {"modelVersion"}, {"model_version"}, {"response", "model"}, {"response", "modelVersion"}}
	if !isStream {
		return findModelInObject(payload.Object, paths)
	}
	return findModelInEvents(payload.Events, paths)
}

func extractGeminiUsage(mapped map[string]any) ReportedUsage {
	usageMap, ok := text.AsMap(mapped["usageMetadata"])
	if !ok {
		return ReportedUsage{}
	}

	return ReportedUsage{
		PromptTokens:     intValue(usageMap["promptTokenCount"]),
		CompletionTokens: intValue(usageMap["candidatesTokenCount"]),
		TotalTokens:      intValue(usageMap["totalTokenCount"]),
	}
}
