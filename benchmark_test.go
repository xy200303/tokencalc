package tokencalc

import (
	"os"
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

func mustReadBenchmarkFile(b *testing.B, path string) []byte {
	b.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		b.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return data
}
