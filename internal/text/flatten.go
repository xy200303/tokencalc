package text

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

type PlaceholderPolicy struct {
	ImageTokenCost int
	AudioTokenCost int
	FileTokenCost  int
}

type Builder struct {
	text        strings.Builder
	hasText     bool
	extraTokens int
	runeCount   int
	notes       []string
	seenNotes   map[string]struct{}
}

func NewBuilder() *Builder {
	return &Builder{
		seenNotes: make(map[string]struct{}),
	}
}

func (b *Builder) AddText(value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if b.hasText {
		b.text.WriteByte('\n')
		b.runeCount++
	}
	b.text.WriteString(value)
	b.runeCount += utf8.RuneCountInString(value)
	b.hasText = true
}

func (b *Builder) AddJSON(value any) {
	encoded, err := json.Marshal(value)
	if err != nil {
		b.AddText(fmt.Sprintf("%v", value))
		return
	}
	b.AddText(string(encoded))
}

func (b *Builder) AddPlaceholder(kind string, tokens int) {
	if tokens <= 0 {
		return
	}

	b.extraTokens += tokens

	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "image":
		b.AddNote("image parts counted by placeholder policy")
	case "audio":
		b.AddNote("audio parts counted by placeholder policy")
	case "file":
		b.AddNote("file parts counted by placeholder policy")
	}
}

func (b *Builder) AddNote(note string) {
	note = strings.TrimSpace(note)
	if note == "" {
		return
	}
	if _, ok := b.seenNotes[note]; ok {
		return
	}
	b.seenNotes[note] = struct{}{}
	b.notes = append(b.notes, note)
}

func (b *Builder) Result() (text string, extraTokens int, note string) {
	return b.text.String(), b.extraTokens, strings.Join(b.notes, "; ")
}

func (b *Builder) ResultText() string {
	return b.text.String()
}

func (b *Builder) RuneCount() int {
	return b.runeCount
}

func AsMap(value any) (map[string]any, bool) {
	mapped, ok := value.(map[string]any)
	return mapped, ok
}

func AsSlice(value any) ([]any, bool) {
	items, ok := value.([]any)
	return items, ok
}

func StringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case bool:
		return strconv.FormatBool(typed)
	default:
		return ""
	}
}

func FirstString(mapped map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := mapped[key]; ok {
			if text := StringValue(value); text != "" {
				return text
			}
		}
	}
	return ""
}
