package tokencalc

import (
	"github.com/xy200303/tokencalc/internal/modelmap"
)

func ResolveEncoding(model string) string {
	encoding, _ := modelmap.Resolve(model)
	return encoding
}

func resolveEncodingNote(model string) (string, string) {
	encoding, source := modelmap.Resolve(model)

	switch source {
	case modelmap.MatchExact:
		return encoding, ""
	case modelmap.MatchPrefix:
		return encoding, "model prefix matched encoding"
	default:
		if model == "" {
			return encoding, "model missing, fallback encoding applied"
		}
		return encoding, "unknown model, fallback encoding applied"
	}
}
