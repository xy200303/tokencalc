package provider

import (
	"bytes"

	"github.com/xy200303/tokencalc/internal/text"
)

type geminiEstimator struct {
	policy text.PlaceholderPolicy
}

func NewGemini(policy PlaceholderPolicy) Estimator {
	return geminiEstimator{policy: buildPolicy(policy)}
}

func (e geminiEstimator) ExtractPrompt(body []byte) (ExtractResult, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return ExtractResult{Supported: true, Note: "request body missing"}, nil
	}

	payload, err := decodeJSONObject(body)
	if err != nil {
		return ExtractResult{}, err
	}

	builder := text.NewBuilder()
	if system, ok := payload["systemInstruction"]; ok {
		appendGenericContent(builder, system, e.policy)
	}
	if contents, ok := payload["contents"]; ok {
		appendGenericContent(builder, contents, e.policy)
	} else {
		builder.AddNote("contents missing")
	}
	if tools, ok := payload["tools"]; ok {
		builder.AddJSON(tools)
	}

	return resultFromBuilder(builder), nil
}

func (e geminiEstimator) ExtractCompletion(body []byte, isStream bool) (ExtractResult, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return ExtractResult{Supported: true, Note: "response body missing, only prompt estimated"}, nil
	}

	if isStream {
		return e.extractStream(body)
	}

	payload, err := decodeJSONObject(body)
	if err != nil {
		return ExtractResult{}, err
	}

	builder := text.NewBuilder()
	if candidates, ok := payload["candidates"]; ok {
		appendGenericContent(builder, candidates, e.policy)
	} else {
		builder.AddNote("candidates missing")
	}

	return resultFromBuilder(builder), nil
}

func (e geminiEstimator) extractStream(body []byte) (ExtractResult, error) {
	events, err := extractEventObjects(body)
	if err != nil {
		return ExtractResult{}, err
	}

	builder := text.NewBuilder()
	for _, event := range events {
		appendGenericContent(builder, event, e.policy)
	}

	result := resultFromBuilder(builder)
	if result.Text == "" && result.ExtraTokens == 0 {
		result.Note = joinProviderNotes(result.Note, "stream contained no extractable gemini deltas")
	}
	return result, nil
}
