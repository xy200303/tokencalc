package tokencalc

import "testing"

func TestCountTexts(t *testing.T) {
	t.Parallel()

	service := New()
	results := service.CountTexts([]CountTextRequest{
		{Model: "gpt-4o-mini", Text: "hello"},
		{Model: "claude-3-5-sonnet", Text: "world"},
	})

	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want %d", len(results), 2)
	}
	if results[0].Error != nil {
		t.Fatalf("results[0].Error = %v, want nil", results[0].Error)
	}
	if results[1].Error != nil {
		t.Fatalf("results[1].Error = %v, want nil", results[1].Error)
	}
	if results[0].Encoding != "o200k_base" {
		t.Fatalf("results[0].Encoding = %q, want %q", results[0].Encoding, "o200k_base")
	}
	if results[1].Encoding != "cl100k_base" {
		t.Fatalf("results[1].Encoding = %q, want %q", results[1].Encoding, "cl100k_base")
	}

	count, encoding, err := service.CountText("gpt-4o-mini", "hello")
	if err != nil {
		t.Fatalf("CountText() error = %v", err)
	}
	if results[0].Count != count || results[0].Encoding != encoding {
		t.Fatalf("results[0] = %+v, want count=%d encoding=%q", results[0], count, encoding)
	}
}

func TestEstimateBatch(t *testing.T) {
	t.Parallel()

	service := New()
	results := service.EstimateBatch([]EstimateRequest{
		{
			Protocol:     ProtocolOpenAIChat,
			RequestBody:  readFixture(t, "testdata/openai_chat/request.json"),
			ResponseBody: readFixture(t, "testdata/openai_chat/response.json"),
		},
		{
			Protocol:     ProtocolOpenAIChat,
			RequestModel: "gpt-4o-mini",
			RequestBody:  []byte("{"),
		},
	})

	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want %d", len(results), 2)
	}
	if results[0].Error != nil {
		t.Fatalf("results[0].Error = %v, want nil", results[0].Error)
	}
	if results[0].Result.Source != SourceLocalEstimate {
		t.Fatalf("results[0].Result.Source = %s, want %s", results[0].Result.Source, SourceLocalEstimate)
	}
	if results[1].Error == nil {
		t.Fatal("results[1].Error = nil, want non-nil")
	}
}

func TestTopLevelBatchHelpers(t *testing.T) {
	t.Parallel()

	countResults := CountTexts([]CountTextRequest{
		{Model: "gpt-4o-mini", Text: "hello"},
	})
	if len(countResults) != 1 {
		t.Fatalf("len(countResults) = %d, want %d", len(countResults), 1)
	}
	if countResults[0].Error != nil {
		t.Fatalf("countResults[0].Error = %v, want nil", countResults[0].Error)
	}

	estimateResults := EstimateBatch([]EstimateRequest{
		{
			Protocol:     ProtocolOpenAIChat,
			RequestBody:  readFixture(t, "testdata/openai_chat/request.json"),
			ResponseBody: readFixture(t, "testdata/openai_chat/response.json"),
		},
	})
	if len(estimateResults) != 1 {
		t.Fatalf("len(estimateResults) = %d, want %d", len(estimateResults), 1)
	}
	if estimateResults[0].Error != nil {
		t.Fatalf("estimateResults[0].Error = %v, want nil", estimateResults[0].Error)
	}
}
