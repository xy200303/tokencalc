package tokencalc

import "testing"

func TestResolveEncoding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		model string
		want  string
	}{
		{model: "gpt-4o-mini", want: "o200k_base"},
		{model: "gpt-5", want: "o200k_base"},
		{model: "claude-3-5-sonnet", want: "cl100k_base"},
		{model: "gemini-2.0-flash", want: "cl100k_base"},
		{model: "unknown-model", want: "cl100k_base"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.model, func(t *testing.T) {
			t.Parallel()
			if got := ResolveEncoding(tt.model); got != tt.want {
				t.Fatalf("ResolveEncoding(%q) = %q, want %q", tt.model, got, tt.want)
			}
		})
	}
}
