package provider

import (
	"bytes"
	"strings"

	"github.com/xy200303/tokencalc/internal/text"
)

type anthropicEstimator struct {
	policy text.PlaceholderPolicy
}

func NewAnthropic(policy PlaceholderPolicy) Estimator {
	return anthropicEstimator{policy: buildPolicy(policy)}
}

func (e anthropicEstimator) ExtractPrompt(body []byte) (ExtractResult, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return ExtractResult{Supported: true, Note: "request body missing"}, nil
	}

	payload, err := decodeJSONObject(body)
	if err != nil {
		return ExtractResult{}, err
	}

	builder := text.NewBuilder()
	if system, ok := payload["system"]; ok {
		appendGenericContent(builder, system, e.policy)
	}
	if messages, ok := payload["messages"]; ok {
		appendGenericContent(builder, messages, e.policy)
	} else {
		builder.AddNote("messages missing")
	}
	if tools, ok := payload["tools"]; ok {
		builder.AddJSON(tools)
	}

	return resultFromBuilder(builder), nil
}

func (e anthropicEstimator) ExtractCompletion(body []byte, isStream bool) (ExtractResult, error) {
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
	if content, ok := payload["content"]; ok {
		appendGenericContent(builder, content, e.policy)
	} else {
		builder.AddNote("content missing")
	}

	return resultFromBuilder(builder), nil
}

func (e anthropicEstimator) extractStream(body []byte) (ExtractResult, error) {
	events, err := extractEventObjects(body)
	if err != nil {
		return ExtractResult{}, err
	}

	builder := text.NewBuilder()
	for _, event := range events {
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
