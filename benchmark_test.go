package tokencalc

import (
	"os"
	"strings"
	"testing"
)

func BenchmarkCountText(b *testing.B) {
	service := New()
	text := string(mustReadBenchmarkFile(b, "testdata/openai_chat/request.json"))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := service.CountText("gpt-4o-mini", text); err != nil {
			b.Fatalf("CountText() error = %v", err)
		}
	}
}

func BenchmarkCountTextsBatch8(b *testing.B) {
	service := New()
	text := string(mustReadBenchmarkFile(b, "testdata/openai_chat/request.json"))
	requests := make([]CountTextRequest, 8)
	for i := range requests {
		requests[i] = CountTextRequest{
			Model: "gpt-4o-mini",
			Text:  text,
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := service.CountTexts(requests)
		if len(results) != len(requests) {
			b.Fatalf("len(results) = %d, want %d", len(results), len(requests))
		}
		for _, result := range results {
			if result.Error != nil {
				b.Fatalf("CountTexts() result error = %v", result.Error)
			}
		}
	}
}

func BenchmarkEstimateOpenAIChatLocal(b *testing.B) {
	service := New()
	request := EstimateRequest{
		Protocol:     ProtocolOpenAIChat,
		RequestBody:  mustReadBenchmarkFile(b, "testdata/openai_chat/request.json"),
		ResponseBody: mustReadBenchmarkFile(b, "testdata/openai_chat/response.json"),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.Estimate(request); err != nil {
			b.Fatalf("Estimate() error = %v", err)
		}
	}
}

func BenchmarkEstimateOpenAIChatPromptOnly(b *testing.B) {
	service := New()
	request := EstimateRequest{
		Protocol:    ProtocolOpenAIChat,
		RequestBody: mustReadBenchmarkFile(b, "testdata/openai_chat/request.json"),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.Estimate(request); err != nil {
			b.Fatalf("Estimate() error = %v", err)
		}
	}
}

func BenchmarkEstimateOpenAIChatReportedUsage(b *testing.B) {
	service := New()
	request := EstimateRequest{
		Protocol:     ProtocolOpenAIChat,
		RequestBody:  mustReadBenchmarkFile(b, "testdata/openai_chat/request.json"),
		ResponseBody: mustReadBenchmarkFile(b, "testdata/openai_chat/response_with_usage.json"),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.Estimate(request); err != nil {
			b.Fatalf("Estimate() error = %v", err)
		}
	}
}

func BenchmarkEstimateOpenAIChatStream(b *testing.B) {
	service := New()
	request := EstimateRequest{
		Protocol:     ProtocolOpenAIChat,
		RequestBody:  mustReadBenchmarkFile(b, "testdata/openai_chat/request.json"),
		ResponseBody: mustReadBenchmarkFile(b, "testdata/openai_chat/stream.sse"),
		IsStream:     true,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.Estimate(request); err != nil {
			b.Fatalf("Estimate() error = %v", err)
		}
	}
}

func BenchmarkEstimateOpenAIChatSingleStreamChunk(b *testing.B) {
	service := New()
	request := EstimateRequest{
		Protocol:     ProtocolOpenAIChat,
		RequestModel: "gpt-4o-mini",
		ResponseBody: []byte("data: {\"choices\":[{\"delta\":{\"content\":\"One\"}}]}\n\n"),
		IsStream:     true,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.Estimate(request); err != nil {
			b.Fatalf("Estimate() error = %v", err)
		}
	}
}

func BenchmarkEstimateBatchOpenAIChat8(b *testing.B) {
	service := New()
	requestBody := mustReadBenchmarkFile(b, "testdata/openai_chat/request.json")
	responseBody := mustReadBenchmarkFile(b, "testdata/openai_chat/response.json")
	requests := make([]EstimateRequest, 8)
	for i := range requests {
		requests[i] = EstimateRequest{
			Protocol:     ProtocolOpenAIChat,
			RequestBody:  requestBody,
			ResponseBody: responseBody,
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := service.EstimateBatch(requests)
		if len(results) != len(requests) {
			b.Fatalf("len(results) = %d, want %d", len(results), len(requests))
		}
		for _, result := range results {
			if result.Error != nil {
				b.Fatalf("EstimateBatch() result error = %v", result.Error)
			}
		}
	}
}

func BenchmarkEstimateOpenAIResponsesLocal(b *testing.B) {
	service := New()
	request := EstimateRequest{
		Protocol:     ProtocolOpenAIResponses,
		RequestBody:  mustReadBenchmarkFile(b, "testdata/openai_responses/request.json"),
		ResponseBody: mustReadBenchmarkFile(b, "testdata/openai_responses/response.json"),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.Estimate(request); err != nil {
			b.Fatalf("Estimate() error = %v", err)
		}
	}
}

func BenchmarkEstimateOpenAIResponsesReportedUsage(b *testing.B) {
	service := New()
	request := EstimateRequest{
		Protocol:     ProtocolOpenAIResponses,
		RequestBody:  mustReadBenchmarkFile(b, "testdata/openai_responses/request.json"),
		ResponseBody: mustReadBenchmarkFile(b, "testdata/openai_responses/response_with_usage.json"),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.Estimate(request); err != nil {
			b.Fatalf("Estimate() error = %v", err)
		}
	}
}

func BenchmarkEstimateAnthropicLocal(b *testing.B) {
	service := New()
	request := EstimateRequest{
		Protocol:     ProtocolAnthropic,
		RequestBody:  mustReadBenchmarkFile(b, "testdata/anthropic/request.json"),
		ResponseBody: mustReadBenchmarkFile(b, "testdata/anthropic/response.json"),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.Estimate(request); err != nil {
			b.Fatalf("Estimate() error = %v", err)
		}
	}
}

func BenchmarkEstimateAnthropicReportedUsage(b *testing.B) {
	service := New()
	request := EstimateRequest{
		Protocol:     ProtocolAnthropic,
		RequestBody:  mustReadBenchmarkFile(b, "testdata/anthropic/request.json"),
		ResponseBody: mustReadBenchmarkFile(b, "testdata/anthropic/response_with_usage.json"),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.Estimate(request); err != nil {
			b.Fatalf("Estimate() error = %v", err)
		}
	}
}

func BenchmarkEstimateGeminiLocal(b *testing.B) {
	service := New()
	request := EstimateRequest{
		Protocol:     ProtocolGemini,
		RequestBody:  mustReadBenchmarkFile(b, "testdata/gemini/request.json"),
		ResponseBody: mustReadBenchmarkFile(b, "testdata/gemini/response.json"),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.Estimate(request); err != nil {
			b.Fatalf("Estimate() error = %v", err)
		}
	}
}

func BenchmarkEstimateGeminiReportedUsage(b *testing.B) {
	service := New()
	request := EstimateRequest{
		Protocol:     ProtocolGemini,
		RequestBody:  mustReadBenchmarkFile(b, "testdata/gemini/request.json"),
		ResponseBody: mustReadBenchmarkFile(b, "testdata/gemini/response_with_usage.json"),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.Estimate(request); err != nil {
			b.Fatalf("Estimate() error = %v", err)
		}
	}
}

func BenchmarkStreamingCounterOpenAIChatLocal(b *testing.B) {
	request := EstimateRequest{
		Protocol:     ProtocolOpenAIChat,
		RequestModel: "gpt-4o-mini",
		RequestBody:  mustReadBenchmarkFile(b, "testdata/openai_chat/request.json"),
	}
	chunks := mustReadBenchmarkStreamChunks(b, "testdata/openai_chat/stream.sse")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter, err := NewStreamingCounter(request)
		if err != nil {
			b.Fatalf("NewStreamingCounter() error = %v", err)
		}
		for _, chunk := range chunks {
			if _, err := counter.AddChunk(chunk); err != nil {
				b.Fatalf("AddChunk() error = %v", err)
			}
		}
		if _, err := counter.FinalResult(); err != nil {
			b.Fatalf("FinalResult() error = %v", err)
		}
	}
}

func BenchmarkStreamingCounterOpenAIChatUsageSync(b *testing.B) {
	request := EstimateRequest{
		Protocol:     ProtocolOpenAIChat,
		RequestModel: "gpt-4o-mini",
		RequestBody:  mustReadBenchmarkFile(b, "testdata/openai_chat/request.json"),
	}
	chunks := mustReadBenchmarkStreamChunks(b, "testdata/openai_chat/stream_with_usage.sse")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter, err := NewStreamingCounter(request)
		if err != nil {
			b.Fatalf("NewStreamingCounter() error = %v", err)
		}
		for _, chunk := range chunks {
			if _, err := counter.AddChunk(chunk); err != nil {
				b.Fatalf("AddChunk() error = %v", err)
			}
		}
		if _, err := counter.FinalResult(); err != nil {
			b.Fatalf("FinalResult() error = %v", err)
		}
	}
}

func mustReadBenchmarkFile(b *testing.B, path string) []byte {
	b.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		b.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return data
}

func mustReadBenchmarkStreamChunks(b *testing.B, path string) [][]byte {
	b.Helper()

	raw := string(mustReadBenchmarkFile(b, path))
	parts := strings.Split(raw, "\n\n")
	chunks := make([][]byte, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		chunks = append(chunks, []byte(part+"\n\n"))
	}
	return chunks
}
