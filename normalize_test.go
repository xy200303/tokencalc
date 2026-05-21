package tokencalc

import "testing"

func TestNormalizeUsage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input Usage
		want  Usage
	}{
		{
			name:  "fills total from parts",
			input: Usage{PromptTokens: 10, CompletionTokens: 5},
			want:  Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		},
		{
			name:  "repairs invalid total",
			input: Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 3},
			want:  Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		},
		{
			name:  "drops negatives",
			input: Usage{PromptTokens: -1, CompletionTokens: 4, TotalTokens: -9},
			want:  Usage{PromptTokens: 0, CompletionTokens: 4, TotalTokens: 4},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := NormalizeUsage(tt.input); got != tt.want {
				t.Fatalf("NormalizeUsage(%+v) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMergeUsage(t *testing.T) {
	t.Parallel()

	reported := Usage{PromptTokens: 50}
	estimated := Usage{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10}

	got := MergeUsage(reported, estimated)
	want := Usage{PromptTokens: 50, CompletionTokens: 3, TotalTokens: 53}
	if got != want {
		t.Fatalf("MergeUsage() = %+v, want %+v", got, want)
	}
}
