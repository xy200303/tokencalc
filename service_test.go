package tokencalc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEstimateOpenAIChat(t *testing.T) {
	t.Parallel()

	service := New()
	requestBody := readFixture(t, "testdata/openai_chat/request.json")
	responseBody := readFixture(t, "testdata/openai_chat/response.json")

	result, err := service.Estimate(EstimateRequest{
		Protocol:     ProtocolOpenAIChat,
		RequestBody:  requestBody,
		ResponseBody: responseBody,
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}

	wantPrompt, _, err := service.CountText("gpt-4o-mini", "You are helpful.\nCount to three.")
	if err != nil {
		t.Fatalf("CountText() error = %v", err)
	}
	wantCompletion, _, err := service.CountText("gpt-4o-mini", "One, two, three.")
	if err != nil {
		t.Fatalf("CountText() error = %v", err)
	}

	if result.Source != SourceLocalEstimate {
		t.Fatalf("Source = %s, want %s", result.Source, SourceLocalEstimate)
	}
	if result.ResolvedModel != "gpt-4o-mini" {
		t.Fatalf("ResolvedModel = %q, want %q", result.ResolvedModel, "gpt-4o-mini")
	}
	if !result.Supported {
		t.Fatal("Supported = false, want true")
	}
	if result.Encoding != "o200k_base" {
		t.Fatalf("Encoding = %q, want %q", result.Encoding, "o200k_base")
	}
	if result.Usage.PromptTokens != wantPrompt {
		t.Fatalf("PromptTokens = %d, want %d", result.Usage.PromptTokens, wantPrompt)
	}
	if result.Usage.CompletionTokens != wantCompletion {
		t.Fatalf("CompletionTokens = %d, want %d", result.Usage.CompletionTokens, wantCompletion)
	}
	if result.Usage.TotalTokens != wantPrompt+wantCompletion {
		t.Fatalf("TotalTokens = %d, want %d", result.Usage.TotalTokens, wantPrompt+wantCompletion)
	}
	if !strings.Contains(result.Note, "model extracted from request body") {
		t.Fatalf("Note = %q, want request model note", result.Note)
	}
}

func TestEstimateAutoDetectModelFromResponseBody(t *testing.T) {
	t.Parallel()

	service := New()
	result, err := service.Estimate(EstimateRequest{
		Protocol:     ProtocolOpenAIChat,
		RequestBody:  readFixture(t, "testdata/openai_chat/request_no_model.json"),
		ResponseBody: readFixture(t, "testdata/openai_chat/response_with_model.json"),
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}

	if result.ResolvedModel != "gpt-4o-mini" {
		t.Fatalf("ResolvedModel = %q, want %q", result.ResolvedModel, "gpt-4o-mini")
	}
	if result.Encoding != "o200k_base" {
		t.Fatalf("Encoding = %q, want %q", result.Encoding, "o200k_base")
	}
	if !strings.Contains(result.Note, "model extracted from response body") {
		t.Fatalf("Note = %q, want response model note", result.Note)
	}
}

func TestEstimateOpenAIChatReportedUsageFromResponseBody(t *testing.T) {
	t.Parallel()

	service := New()
	result, err := service.Estimate(EstimateRequest{
		Protocol:     ProtocolOpenAIChat,
		RequestModel: "gpt-4o-mini",
		RequestBody:  readFixture(t, "testdata/openai_chat/request.json"),
		ResponseBody: readFixture(t, "testdata/openai_chat/response_with_usage.json"),
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}

	want := Usage{PromptTokens: 111, CompletionTokens: 222, TotalTokens: 333}
	if result.Source != SourceReportedUsage {
		t.Fatalf("Source = %s, want %s", result.Source, SourceReportedUsage)
	}
	if result.Usage != want {
		t.Fatalf("Usage = %+v, want %+v", result.Usage, want)
	}
	if !strings.Contains(result.Note, "reported usage extracted from response body") {
		t.Fatalf("Note = %q, want response usage note", result.Note)
	}
}

func TestEstimateOpenAIChatStream(t *testing.T) {
	t.Parallel()

	service := New()
	collector, err := NewStreamCollector(ProtocolOpenAIChat)
	if err != nil {
		t.Fatalf("NewStreamCollector() error = %v", err)
	}

	for _, chunk := range strings.Split(string(readFixture(t, "testdata/openai_chat/stream.sse")), "\n\n") {
		if strings.TrimSpace(chunk) == "" {
			continue
		}
		if err := collector.AddChunk([]byte(chunk + "\n\n")); err != nil {
			t.Fatalf("AddChunk() error = %v", err)
		}
	}

	result, err := service.Estimate(EstimateRequest{
		Protocol:     ProtocolOpenAIChat,
		RequestModel: "gpt-4o-mini",
		RequestBody:  readFixture(t, "testdata/openai_chat/request.json"),
		ResponseBody: collector.FinalBody(),
		IsStream:     true,
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}

	wantCompletion, _, err := service.CountText("gpt-4o-mini", "One\ntwo\nthree")
	if err != nil {
		t.Fatalf("CountText() error = %v", err)
	}
	if result.Usage.CompletionTokens != wantCompletion {
		t.Fatalf("CompletionTokens = %d, want %d", result.Usage.CompletionTokens, wantCompletion)
	}
}

func TestEstimateOpenAIChatReportedUsageFromStream(t *testing.T) {
	t.Parallel()

	service := New()
	result, err := service.Estimate(EstimateRequest{
		Protocol:     ProtocolOpenAIChat,
		RequestModel: "gpt-4o-mini",
		RequestBody:  readFixture(t, "testdata/openai_chat/request.json"),
		ResponseBody: readFixture(t, "testdata/openai_chat/stream_with_usage.sse"),
		IsStream:     true,
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}

	want := Usage{PromptTokens: 90, CompletionTokens: 12, TotalTokens: 102}
	if result.Source != SourceReportedUsage {
		t.Fatalf("Source = %s, want %s", result.Source, SourceReportedUsage)
	}
	if result.Usage != want {
		t.Fatalf("Usage = %+v, want %+v", result.Usage, want)
	}
	if !strings.Contains(result.Note, "reported usage extracted from stream events") {
		t.Fatalf("Note = %q, want stream usage note", result.Note)
	}
}

func TestEstimateSingleOpenAIChatStreamChunk(t *testing.T) {
	t.Parallel()

	service := New()
	result, err := service.Estimate(EstimateRequest{
		Protocol:     ProtocolOpenAIChat,
		RequestModel: "gpt-4o-mini",
		ResponseBody: []byte("data: {\"choices\":[{\"delta\":{\"content\":\"One\"}}]}\n\n"),
		IsStream:     true,
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}

	wantCompletion, _, err := service.CountText("gpt-4o-mini", "One")
	if err != nil {
		t.Fatalf("CountText() error = %v", err)
	}

	if result.Source != SourceLocalEstimate {
		t.Fatalf("Source = %s, want %s", result.Source, SourceLocalEstimate)
	}
	if result.Usage.PromptTokens != 0 {
		t.Fatalf("PromptTokens = %d, want %d", result.Usage.PromptTokens, 0)
	}
	if result.Usage.CompletionTokens != wantCompletion {
		t.Fatalf("CompletionTokens = %d, want %d", result.Usage.CompletionTokens, wantCompletion)
	}
	if result.Usage.TotalTokens != wantCompletion {
		t.Fatalf("TotalTokens = %d, want %d", result.Usage.TotalTokens, wantCompletion)
	}
}

func TestEstimateOpenAIResponsesMerge(t *testing.T) {
	t.Parallel()

	service := New()
	result, err := service.Estimate(EstimateRequest{
		Protocol:      ProtocolOpenAIResponses,
		RequestModel:  "gpt-4.1-mini",
		RequestBody:   readFixture(t, "testdata/openai_responses/request.json"),
		ResponseBody:  readFixture(t, "testdata/openai_responses/response.json"),
		ReportedUsage: &Usage{PromptTokens: 50},
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}

	wantCompletion, _, err := service.CountText("gpt-4.1-mini", "Gravity pulls objects together.")
	if err != nil {
		t.Fatalf("CountText() error = %v", err)
	}

	if result.Source != SourceMerged {
		t.Fatalf("Source = %s, want %s", result.Source, SourceMerged)
	}
	if result.Usage.PromptTokens != 50 {
		t.Fatalf("PromptTokens = %d, want %d", result.Usage.PromptTokens, 50)
	}
	if result.Usage.CompletionTokens != wantCompletion {
		t.Fatalf("CompletionTokens = %d, want %d", result.Usage.CompletionTokens, wantCompletion)
	}
	if result.Usage.TotalTokens != 50+wantCompletion {
		t.Fatalf("TotalTokens = %d, want %d", result.Usage.TotalTokens, 50+wantCompletion)
	}
	if !strings.Contains(result.Note, "merged with local estimate") {
		t.Fatalf("Note = %q, want merge note", result.Note)
	}
}

func TestEstimateOpenAIResponsesReportedUsageFromResponseBody(t *testing.T) {
	t.Parallel()

	service := New()
	result, err := service.Estimate(EstimateRequest{
		Protocol:     ProtocolOpenAIResponses,
		RequestModel: "gpt-4.1-mini",
		RequestBody:  readFixture(t, "testdata/openai_responses/request.json"),
		ResponseBody: readFixture(t, "testdata/openai_responses/response_with_usage.json"),
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}

	want := Usage{PromptTokens: 15, CompletionTokens: 6, TotalTokens: 21}
	if result.Source != SourceReportedUsage {
		t.Fatalf("Source = %s, want %s", result.Source, SourceReportedUsage)
	}
	if result.Usage != want {
		t.Fatalf("Usage = %+v, want %+v", result.Usage, want)
	}
}

func TestEstimateAnthropicPlaceholder(t *testing.T) {
	t.Parallel()

	service := New()
	result, err := service.Estimate(EstimateRequest{
		Protocol:     ProtocolAnthropic,
		RequestModel: "claude-3-5-sonnet",
		RequestBody:  readFixture(t, "testdata/anthropic/request.json"),
		ResponseBody: readFixture(t, "testdata/anthropic/response.json"),
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}

	wantPromptText, _, err := service.CountText("claude-3-5-sonnet", "You are Claude.\nSay hi.")
	if err != nil {
		t.Fatalf("CountText() error = %v", err)
	}
	wantCompletion, _, err := service.CountText("claude-3-5-sonnet", "Hi there.")
	if err != nil {
		t.Fatalf("CountText() error = %v", err)
	}

	wantPrompt := wantPromptText + DefaultPlaceholderPolicy().ImageTokenCost
	if result.Usage.PromptTokens != wantPrompt {
		t.Fatalf("PromptTokens = %d, want %d", result.Usage.PromptTokens, wantPrompt)
	}
	if result.Usage.CompletionTokens != wantCompletion {
		t.Fatalf("CompletionTokens = %d, want %d", result.Usage.CompletionTokens, wantCompletion)
	}
	if !strings.Contains(result.Note, "image parts counted by placeholder policy") {
		t.Fatalf("Note = %q, want placeholder note", result.Note)
	}
}

func TestEstimateAnthropicReportedUsageFromResponseBody(t *testing.T) {
	t.Parallel()

	service := New()
	result, err := service.Estimate(EstimateRequest{
		Protocol:     ProtocolAnthropic,
		RequestModel: "claude-3-5-sonnet",
		RequestBody:  readFixture(t, "testdata/anthropic/request.json"),
		ResponseBody: readFixture(t, "testdata/anthropic/response_with_usage.json"),
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}

	want := Usage{PromptTokens: 9, CompletionTokens: 4, TotalTokens: 13}
	if result.Source != SourceReportedUsage {
		t.Fatalf("Source = %s, want %s", result.Source, SourceReportedUsage)
	}
	if result.Usage != want {
		t.Fatalf("Usage = %+v, want %+v", result.Usage, want)
	}
}

func TestEstimateGemini(t *testing.T) {
	t.Parallel()

	service := New()
	result, err := service.Estimate(EstimateRequest{
		Protocol:     ProtocolGemini,
		RequestModel: "gemini-2.0-flash",
		RequestBody:  readFixture(t, "testdata/gemini/request.json"),
		ResponseBody: readFixture(t, "testdata/gemini/response.json"),
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}

	wantPrompt, _, err := service.CountText("gemini-2.0-flash", "Keep it short.\nSummarize stars.")
	if err != nil {
		t.Fatalf("CountText() error = %v", err)
	}
	wantCompletion, _, err := service.CountText("gemini-2.0-flash", "Stars are hot balls of gas.")
	if err != nil {
		t.Fatalf("CountText() error = %v", err)
	}

	if result.Usage.PromptTokens != wantPrompt {
		t.Fatalf("PromptTokens = %d, want %d", result.Usage.PromptTokens, wantPrompt)
	}
	if result.Usage.CompletionTokens != wantCompletion {
		t.Fatalf("CompletionTokens = %d, want %d", result.Usage.CompletionTokens, wantCompletion)
	}
}

func TestEstimateGeminiReportedUsageFromResponseBody(t *testing.T) {
	t.Parallel()

	service := New()
	result, err := service.Estimate(EstimateRequest{
		Protocol:     ProtocolGemini,
		RequestModel: "gemini-2.0-flash",
		RequestBody:  readFixture(t, "testdata/gemini/request.json"),
		ResponseBody: readFixture(t, "testdata/gemini/response_with_usage.json"),
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}

	want := Usage{PromptTokens: 12, CompletionTokens: 5, TotalTokens: 17}
	if result.Source != SourceReportedUsage {
		t.Fatalf("Source = %s, want %s", result.Source, SourceReportedUsage)
	}
	if result.Usage != want {
		t.Fatalf("Usage = %+v, want %+v", result.Usage, want)
	}
}

func TestEstimateAutoDetectGeminiModelFromResponseBody(t *testing.T) {
	t.Parallel()

	service := New()
	result, err := service.Estimate(EstimateRequest{
		Protocol:     ProtocolGemini,
		RequestBody:  readFixture(t, "testdata/gemini/request.json"),
		ResponseBody: readFixture(t, "testdata/gemini/response_with_model.json"),
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}

	if result.ResolvedModel != "gemini-2.0-flash" {
		t.Fatalf("ResolvedModel = %q, want %q", result.ResolvedModel, "gemini-2.0-flash")
	}
	if !strings.Contains(result.Note, "model extracted from response body") {
		t.Fatalf("Note = %q, want response model note", result.Note)
	}
}

func TestEstimateReportedUsageMergedFromCallerAndResponse(t *testing.T) {
	t.Parallel()

	service := New()
	result, err := service.Estimate(EstimateRequest{
		Protocol:      ProtocolOpenAIResponses,
		RequestModel:  "gpt-4.1-mini",
		RequestBody:   readFixture(t, "testdata/openai_responses/request.json"),
		ResponseBody:  readFixture(t, "testdata/openai_responses/response_with_usage.json"),
		ReportedUsage: &Usage{PromptTokens: 50},
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}

	want := Usage{PromptTokens: 50, CompletionTokens: 6, TotalTokens: 56}
	if result.Source != SourceReportedUsage {
		t.Fatalf("Source = %s, want %s", result.Source, SourceReportedUsage)
	}
	if result.Usage != want {
		t.Fatalf("Usage = %+v, want %+v", result.Usage, want)
	}
	if !strings.Contains(result.Note, "merged from caller and response body") {
		t.Fatalf("Note = %q, want merged origin note", result.Note)
	}
}

func TestEstimateUpstreamModelOverridesPayloadModel(t *testing.T) {
	t.Parallel()

	service := New()
	result, err := service.Estimate(EstimateRequest{
		Protocol:      ProtocolOpenAIChat,
		UpstreamModel: "claude-3-5-sonnet",
		RequestBody:   readFixture(t, "testdata/openai_chat/request.json"),
		ResponseBody:  readFixture(t, "testdata/openai_chat/response.json"),
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}

	if result.ResolvedModel != "claude-3-5-sonnet" {
		t.Fatalf("ResolvedModel = %q, want %q", result.ResolvedModel, "claude-3-5-sonnet")
	}
	if result.Encoding != "cl100k_base" {
		t.Fatalf("Encoding = %q, want %q", result.Encoding, "cl100k_base")
	}
	if strings.Contains(result.Note, "model extracted from request body") {
		t.Fatalf("Note = %q, did not expect extracted model note", result.Note)
	}
}

func TestEstimateInvalidJSON(t *testing.T) {
	t.Parallel()

	service := New()
	_, err := service.Estimate(EstimateRequest{
		Protocol:     ProtocolOpenAIChat,
		RequestModel: "gpt-4o-mini",
		RequestBody:  []byte("{"),
	})
	if err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func readFixture(t *testing.T, relative string) []byte {
	t.Helper()

	fullPath := filepath.Join(relative)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", relative, err)
	}
	return data
}
