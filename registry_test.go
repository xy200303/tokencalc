package tokencalc

import "testing"

type staticEstimator struct {
	promptText     string
	completionText string
	reportedUsage  ReportedUsageResult
	promptNote     string
	completionNote string
}

func (s staticEstimator) ExtractPrompt(body []byte) (ExtractResult, error) {
	return ExtractResult{
		Text:      s.promptText,
		Supported: true,
		Note:      s.promptNote,
	}, nil
}

func (s staticEstimator) ExtractCompletion(body []byte, isStream bool) (ExtractResult, error) {
	return ExtractResult{
		Text:      s.completionText,
		Supported: true,
		Note:      s.completionNote,
	}, nil
}

func (s staticEstimator) ExtractReportedUsage(body []byte, isStream bool) (ReportedUsageResult, error) {
	return s.reportedUsage, nil
}

func TestWithEstimatorRegistersCustomProtocol(t *testing.T) {
	t.Parallel()

	service := New(WithEstimator("custom_echo", staticEstimator{
		promptText:     "ping",
		completionText: "pong",
	}))

	result, err := service.Estimate(EstimateRequest{
		Protocol:      "custom_echo",
		UpstreamModel: "gpt-4o-mini",
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}

	wantPrompt, _, err := service.CountText("gpt-4o-mini", "ping")
	if err != nil {
		t.Fatalf("CountText() error = %v", err)
	}
	wantCompletion, _, err := service.CountText("gpt-4o-mini", "pong")
	if err != nil {
		t.Fatalf("CountText() error = %v", err)
	}

	if result.Source != SourceLocalEstimate {
		t.Fatalf("Source = %s, want %s", result.Source, SourceLocalEstimate)
	}
	if result.Usage.PromptTokens != wantPrompt {
		t.Fatalf("PromptTokens = %d, want %d", result.Usage.PromptTokens, wantPrompt)
	}
	if result.Usage.CompletionTokens != wantCompletion {
		t.Fatalf("CompletionTokens = %d, want %d", result.Usage.CompletionTokens, wantCompletion)
	}
}

func TestWithRegistryOverridesBuiltInEstimator(t *testing.T) {
	t.Parallel()

	registry := NewRegistry().Register(ProtocolOpenAIChat, staticEstimator{
		reportedUsage: ReportedUsageResult{
			Usage: ReportedUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
			Note: "custom registry estimator",
		},
	})

	service := New(WithRegistry(registry))
	result, err := service.Estimate(EstimateRequest{
		Protocol:      ProtocolOpenAIChat,
		UpstreamModel: "gpt-4o-mini",
		RequestBody:   []byte("{"),
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}

	want := Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}
	if result.Source != SourceReportedUsage {
		t.Fatalf("Source = %s, want %s", result.Source, SourceReportedUsage)
	}
	if result.Usage != want {
		t.Fatalf("Usage = %+v, want %+v", result.Usage, want)
	}
}

func TestWithRegistryClonesEntries(t *testing.T) {
	t.Parallel()

	registry := NewRegistry().Register(ProtocolOpenAIChat, staticEstimator{
		reportedUsage: ReportedUsageResult{
			Usage: ReportedUsage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
		},
	})

	service := New(WithRegistry(registry))
	registry.Register(ProtocolOpenAIChat, staticEstimator{
		reportedUsage: ReportedUsageResult{
			Usage: ReportedUsage{PromptTokens: 7, CompletionTokens: 8, TotalTokens: 15},
		},
	})

	result, err := service.Estimate(EstimateRequest{
		Protocol: ProtocolOpenAIChat,
	})
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}

	want := Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3}
	if result.Usage != want {
		t.Fatalf("Usage = %+v, want %+v", result.Usage, want)
	}
}
