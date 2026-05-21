package tokencalc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	internalstream "github.com/xy200303/tokencalc/internal/stream"
	"github.com/xy200303/tokencalc/internal/text"
)

func resolveEstimateModel(req EstimateRequest) (string, string, error) {
	if model := strings.TrimSpace(req.UpstreamModel); model != "" {
		return model, "", nil
	}
	if model := strings.TrimSpace(req.RequestModel); model != "" {
		return model, "", nil
	}

	model, err := extractModelFromBody(req.Protocol, req.RequestBody, false)
	if err != nil {
		return "", "", fmt.Errorf("extract model from request body: %w", err)
	}
	if model != "" {
		return model, "model extracted from request body", nil
	}

	model, err = extractModelFromBody(req.Protocol, req.ResponseBody, req.IsStream)
	if err != nil {
		return "", "", fmt.Errorf("extract model from response body: %w", err)
	}
	if model != "" {
		return model, "model extracted from response body", nil
	}

	return "", "", nil
}

func extractModelFromBody(protocol Protocol, body []byte, isStream bool) (string, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return "", nil
	}

	paths := modelPathsForProtocol(protocol)
	if len(paths) == 0 {
		paths = defaultModelPaths()
	}

	if !isStream {
		payload, err := decodeAnyObject(body)
		if err != nil {
			return "", err
		}
		return findModelByPaths(payload, paths), nil
	}

	for _, event := range internalstream.ExtractEvents(body) {
		event = bytes.TrimSpace(event)
		if len(event) == 0 || bytes.Equal(event, []byte("[DONE]")) {
			continue
		}

		payload, err := decodeAnyObject(event)
		if err != nil {
			return "", err
		}
		if model := findModelByPaths(payload, paths); model != "" {
			return model, nil
		}
	}

	return "", nil
}

func decodeAnyObject(body []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()

	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func modelPathsForProtocol(protocol Protocol) [][]string {
	switch protocol {
	case ProtocolOpenAIChat, ProtocolOpenAIResponses:
		return [][]string{
			{"model"},
			{"response", "model"},
		}
	case ProtocolAnthropic:
		return [][]string{
			{"model"},
			{"message", "model"},
		}
	case ProtocolGemini:
		return [][]string{
			{"model"},
			{"modelVersion"},
			{"model_version"},
			{"response", "model"},
			{"response", "modelVersion"},
		}
	default:
		return defaultModelPaths()
	}
}

func defaultModelPaths() [][]string {
	return [][]string{
		{"model"},
		{"modelVersion"},
		{"model_version"},
	}
}

func findModelByPaths(payload map[string]any, paths [][]string) string {
	for _, path := range paths {
		if model := findStringAtPath(payload, path); model != "" {
			return model
		}
	}
	return ""
}

func findStringAtPath(current any, path []string) string {
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
	return findStringAtPath(next, path[1:])
}
