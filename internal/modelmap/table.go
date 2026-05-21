package modelmap

import "strings"

type MatchSource string

const (
	MatchExact   MatchSource = "exact"
	MatchPrefix  MatchSource = "prefix"
	MatchDefault MatchSource = "default"
)

const DefaultEncoding = "cl100k_base"

var exactModels = map[string]string{
	"gpt-4o":            "o200k_base",
	"gpt-4o-mini":       "o200k_base",
	"gpt-4.1":           "o200k_base",
	"gpt-4.1-mini":      "o200k_base",
	"gpt-4.1-nano":      "o200k_base",
	"gpt-5":             "o200k_base",
	"gpt-5-mini":        "o200k_base",
	"gpt-5-nano":        "o200k_base",
	"o1":                "o200k_base",
	"o1-mini":           "o200k_base",
	"o3":                "o200k_base",
	"o3-mini":           "o200k_base",
	"o4-mini":           "o200k_base",
	"chatgpt-4o-latest": "o200k_base",
}

var prefixModels = []struct {
	Prefix   string
	Encoding string
}{
	{Prefix: "gpt-5", Encoding: "o200k_base"},
	{Prefix: "gpt-4o", Encoding: "o200k_base"},
	{Prefix: "gpt-4.1", Encoding: "o200k_base"},
	{Prefix: "o1", Encoding: "o200k_base"},
	{Prefix: "o3", Encoding: "o200k_base"},
	{Prefix: "o4", Encoding: "o200k_base"},
	{Prefix: "qwen", Encoding: "o200k_base"},
	{Prefix: "gpt-4", Encoding: "cl100k_base"},
	{Prefix: "gpt-3.5", Encoding: "cl100k_base"},
	{Prefix: "text-embedding-3", Encoding: "cl100k_base"},
	{Prefix: "claude", Encoding: "cl100k_base"},
	{Prefix: "gemini", Encoding: "cl100k_base"},
}

func Resolve(model string) (string, MatchSource) {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return DefaultEncoding, MatchDefault
	}

	if encoding, ok := exactModels[model]; ok {
		return encoding, MatchExact
	}

	for _, item := range prefixModels {
		if strings.HasPrefix(model, item.Prefix) {
			return item.Encoding, MatchPrefix
		}
	}

	return DefaultEncoding, MatchDefault
}
