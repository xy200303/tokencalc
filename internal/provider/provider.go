package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	internalstream "github.com/xy200303/tokencalc/internal/stream"
	"github.com/xy200303/tokencalc/internal/text"
)

type PlaceholderPolicy struct {
	ImageTokenCost int
	AudioTokenCost int
	FileTokenCost  int
}

type ExtractResult struct {
	Text        string
	ExtraTokens int
	Supported   bool
	Note        string
}

type Estimator interface {
	ExtractPrompt(body []byte) (ExtractResult, error)
	ExtractCompletion(body []byte, isStream bool) (ExtractResult, error)
}

func decodeJSONObject(body []byte) (map[string]any, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return map[string]any{}, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()

	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func resultFromBuilder(builder *text.Builder) ExtractResult {
	content, extraTokens, note := builder.Result()
	return ExtractResult{
		Text:        content,
		ExtraTokens: extraTokens,
		Supported:   true,
		Note:        note,
	}
}

func extractEventObjects(body []byte) ([]map[string]any, error) {
	events := internalstream.ExtractEvents(body)
	objects := make([]map[string]any, 0, len(events))
	for _, event := range events {
		if bytes.Equal(bytes.TrimSpace(event), []byte("[DONE]")) {
			continue
		}

		decoder := json.NewDecoder(bytes.NewReader(event))
		decoder.UseNumber()

		var payload map[string]any
		if err := decoder.Decode(&payload); err != nil {
			return nil, fmt.Errorf("decode stream event: %w", err)
		}
		objects = append(objects, payload)
	}
	return objects, nil
}

func appendGenericContent(builder *text.Builder, value any, policy text.PlaceholderPolicy) {
	switch typed := value.(type) {
	case nil:
		return
	case string:
		builder.AddText(typed)
	case []any:
		for _, item := range typed {
			appendGenericContent(builder, item, policy)
		}
	case map[string]any:
		appendGenericMap(builder, typed, policy)
	default:
		builder.AddJSON(typed)
	}
}

func appendGenericMap(builder *text.Builder, mapped map[string]any, policy text.PlaceholderPolicy) {
	kind := strings.ToLower(strings.TrimSpace(text.StringValue(mapped["type"])))
	switch kind {
	case "image", "input_image", "image_url":
		builder.AddPlaceholder("image", policy.ImageTokenCost)
	case "audio", "input_audio":
		builder.AddPlaceholder("audio", policy.AudioTokenCost)
	case "file", "input_file":
		builder.AddPlaceholder("file", policy.FileTokenCost)
	}

	if textValue := text.FirstString(mapped, "text", "input_text", "output_text", "value", "delta"); textValue != "" {
		builder.AddText(textValue)
	}

	for _, key := range []string{
		"instructions",
		"message",
		"messages",
		"delta",
		"content",
		"content_block",
		"parts",
		"contents",
		"input",
		"output",
		"output_text",
		"response",
		"choices",
		"candidates",
		"candidate",
		"item",
		"tool_calls",
		"function_call",
		"function",
		"tool_call",
		"tool_result",
		"refusal",
		"arguments",
	} {
		value, ok := mapped[key]
		if !ok {
			continue
		}
		if key == "arguments" {
			if raw := text.StringValue(value); raw != "" {
				builder.AddText(raw)
			} else {
				builder.AddJSON(value)
			}
			continue
		}
		appendGenericContent(builder, value, policy)
	}

	if name := text.FirstString(mapped, "name"); name != "" {
		builder.AddText(name)
	}

	switch {
	case mapped["image_url"] != nil || mapped["inlineData"] != nil || mapped["inline_data"] != nil:
		builder.AddPlaceholder("image", policy.ImageTokenCost)
	case mapped["audio"] != nil:
		builder.AddPlaceholder("audio", policy.AudioTokenCost)
	case mapped["fileData"] != nil || mapped["file_data"] != nil:
		builder.AddPlaceholder("file", policy.FileTokenCost)
	}
}

func buildPolicy(policy PlaceholderPolicy) text.PlaceholderPolicy {
	return text.PlaceholderPolicy{
		ImageTokenCost: policy.ImageTokenCost,
		AudioTokenCost: policy.AudioTokenCost,
		FileTokenCost:  policy.FileTokenCost,
	}
}

func joinProviderNotes(notes ...string) string {
	filtered := make([]string, 0, len(notes))
	seen := map[string]struct{}{}
	for _, note := range notes {
		note = strings.TrimSpace(note)
		if note == "" {
			continue
		}
		if _, ok := seen[note]; ok {
			continue
		}
		seen[note] = struct{}{}
		filtered = append(filtered, note)
	}
	return strings.Join(filtered, "; ")
}
