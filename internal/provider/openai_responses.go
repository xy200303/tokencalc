package provider

import (
	"bytes"
	"strings"

	"github.com/xy200303/tokencalc/internal/text"
)

type openAIResponsesEstimator struct {
	policy text.PlaceholderPolicy
}

func NewOpenAIResponses(policy PlaceholderPolicy) Estimator {
	return openAIResponsesEstimator{policy: buildPolicy(policy)}
}

func (e openAIResponsesEstimator) ExtractPrompt(body []byte) (ExtractResult, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return ExtractResult{Supported: true, Note: "request body missing"}, nil
	}

	payload, err := decodeJSONObject(body)
	if err != nil {
		return ExtractResult{}, err
	}

	builder := text.NewBuilder()
	if instructions, ok := payload["instructions"]; ok {
		appendGenericContent(builder, instructions, e.policy)
	}
	if input, ok := payload["input"]; ok {
		appendGenericContent(builder, input, e.policy)
	} else {
		builder.AddNote("input missing")
	}
	if tools, ok := payload["tools"]; ok {
		builder.AddJSON(tools)
	}
	if textConfig, ok := payload["text"]; ok {
		builder.AddJSON(textConfig)
	}

	return resultFromBuilder(builder), nil
}

func (e openAIResponsesEstimator) ExtractCompletion(body []byte, isStream bool) (ExtractResult, error) {
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
	if output, ok := payload["output"]; ok {
		appendGenericContent(builder, output, e.policy)
	}
	if outputText, ok := payload["output_text"]; ok {
		appendGenericContent(builder, outputText, e.policy)
	}
	if response, ok := payload["response"]; ok {
		appendGenericContent(builder, response, e.policy)
	}
	if len(builder.ResultText()) == 0 {
		builder.AddNote("output missing")
	}

	return resultFromBuilder(builder), nil
}

func (e openAIResponsesEstimator) extractStream(body []byte) (ExtractResult, error) {
	events, err := extractEventObjects(body)
	if err != nil {
		return ExtractResult{}, err
	}

	builder := text.NewBuilder()
	for _, event := range events {
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
