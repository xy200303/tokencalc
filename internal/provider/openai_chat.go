package provider

import (
	"bytes"

	"github.com/xy200303/tokencalc/internal/text"
)

type openAIChatEstimator struct {
	policy text.PlaceholderPolicy
}

func NewOpenAIChat(policy PlaceholderPolicy) Estimator {
	return openAIChatEstimator{policy: buildPolicy(policy)}
}

func (e openAIChatEstimator) ExtractPrompt(body []byte) (ExtractResult, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return ExtractResult{Supported: true, Note: "request body missing"}, nil
	}

	payload, err := decodeJSONObject(body)
	if err != nil {
		return ExtractResult{}, err
	}

	builder := text.NewBuilder()
	if messages, ok := payload["messages"]; ok {
		appendGenericContent(builder, messages, e.policy)
	} else {
		builder.AddNote("messages missing")
	}

	if system, ok := payload["system"]; ok {
		appendGenericContent(builder, system, e.policy)
	}
	if tools, ok := payload["tools"]; ok {
		builder.AddJSON(tools)
	}
	if responseFormat, ok := payload["response_format"]; ok {
		builder.AddJSON(responseFormat)
	}

	return resultFromBuilder(builder), nil
}

func (e openAIChatEstimator) ExtractCompletion(body []byte, isStream bool) (ExtractResult, error) {
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
	if choices, ok := payload["choices"]; ok {
		appendGenericContent(builder, choices, e.policy)
	} else if message, ok := payload["message"]; ok {
		appendGenericContent(builder, message, e.policy)
	} else {
		builder.AddNote("choices missing")
	}

	return resultFromBuilder(builder), nil
}

func (e openAIChatEstimator) extractStream(body []byte) (ExtractResult, error) {
	events, err := extractEventObjects(body)
	if err != nil {
		return ExtractResult{}, err
	}

	builder := text.NewBuilder()
	for _, event := range events {
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
