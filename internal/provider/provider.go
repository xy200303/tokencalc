package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
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

type ReportedUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

func (u ReportedUsage) HasAny() bool {
	return u.PromptTokens > 0 || u.CompletionTokens > 0 || u.TotalTokens > 0
}

type ReportedUsageResult struct {
	Usage ReportedUsage
	Note  string
}

type RequestPayload struct {
	Object map[string]any
}

type ResponsePayload struct {
	Object map[string]any
	Events []map[string]any
}

type Estimator interface {
	ExtractPrompt(body []byte) (ExtractResult, error)
	ExtractCompletion(body []byte, isStream bool) (ExtractResult, error)
	ExtractReportedUsage(body []byte, isStream bool) (ReportedUsageResult, error)
}

type PreparedEstimator interface {
	Estimator
	PrepareRequest(body []byte) (RequestPayload, error)
	PrepareResponse(body []byte, isStream bool) (ResponsePayload, error)
	ExtractPromptPrepared(payload RequestPayload) (ExtractResult, error)
	ExtractCompletionPrepared(payload ResponsePayload, isStream bool) (ExtractResult, error)
	ExtractReportedUsagePrepared(payload ResponsePayload, isStream bool) (ReportedUsageResult, error)
	ExtractRequestModelPrepared(payload RequestPayload) string
	ExtractResponseModelPrepared(payload ResponsePayload, isStream bool) string
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

func PrepareRequestObject(body []byte) (RequestPayload, error) {
	object, err := decodeJSONObject(body)
	if err != nil {
		return RequestPayload{}, err
	}
	return RequestPayload{Object: object}, nil
}

func PrepareResponseObject(body []byte, isStream bool) (ResponsePayload, error) {
	if !isStream {
		object, err := decodeJSONObject(body)
		if err != nil {
			return ResponsePayload{}, err
		}
		return ResponsePayload{Object: object}, nil
	}

	events, err := extractEventObjects(body)
	if err != nil {
		return ResponsePayload{}, err
	}
	return ResponsePayload{Events: events}, nil
}

func mergeReportedUsage(current ReportedUsage, next ReportedUsage) ReportedUsage {
	if next.PromptTokens > current.PromptTokens {
		current.PromptTokens = next.PromptTokens
	}
	if next.CompletionTokens > current.CompletionTokens {
		current.CompletionTokens = next.CompletionTokens
	}
	if next.TotalTokens > current.TotalTokens {
		current.TotalTokens = next.TotalTokens
	}
	return current
}

func intValue(value any) int {
	switch typed := value.(type) {
	case nil:
		return 0
	case json.Number:
		if whole, err := typed.Int64(); err == nil {
			return int(whole)
		}
		if decimal, err := typed.Float64(); err == nil {
			return int(decimal)
		}
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case int:
		return typed
	case int8:
		return int(typed)
	case int16:
		return int(typed)
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case string:
		if typed == "" {
			return 0
		}
		if whole, err := strconv.Atoi(typed); err == nil {
			return whole
		}
	}
	return 0
}

func nonNegative(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func normalizeReportedUsage(usage ReportedUsage) ReportedUsage {
	usage.PromptTokens = nonNegative(usage.PromptTokens)
	usage.CompletionTokens = nonNegative(usage.CompletionTokens)
	usage.TotalTokens = nonNegative(usage.TotalTokens)

	sum := usage.PromptTokens + usage.CompletionTokens
	if usage.TotalTokens == 0 && sum > 0 {
		usage.TotalTokens = sum
	}
	if usage.TotalTokens > 0 && usage.TotalTokens < sum {
		usage.TotalTokens = sum
	}
	return usage
}

func findModelInObject(mapped map[string]any, paths [][]string) string {
	return findModelInValue(mapped, paths)
}

func findModelInEvents(events []map[string]any, paths [][]string) string {
	for _, event := range events {
		if model := findModelInValue(event, paths); model != "" {
			return model
		}
	}
	return ""
}

func findModelInValue(current any, paths [][]string) string {
	for _, path := range paths {
		if model := stringAtPath(current, path); model != "" {
			return model
		}
	}
	return ""
}

func stringAtPath(current any, path []string) string {
	if len(path) == 0 {
		return strings.TrimSpace(text.StringValue(current))
	}

	mapped, ok := text.AsMap(current)
	if !ok {
		return ""
	}

	next, ok := mapped[path[0]]
	if !ok {
		return ""
	}
	return stringAtPath(next, path[1:])
}
