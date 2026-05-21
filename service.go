package tokencalc

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/xy200303/tokencalc/internal/codec"
	"github.com/xy200303/tokencalc/internal/provider"
)

type service struct {
	codec      codec.Counter
	estimators map[Protocol]provider.Estimator
}

var defaultService Service = New()

func New(options ...Option) Service {
	cfg := Config{
		PlaceholderPolicy: DefaultPlaceholderPolicy(),
	}
	for _, option := range options {
		option(&cfg)
	}

	policy := provider.PlaceholderPolicy{
		ImageTokenCost: cfg.PlaceholderPolicy.ImageTokenCost,
		AudioTokenCost: cfg.PlaceholderPolicy.AudioTokenCost,
		FileTokenCost:  cfg.PlaceholderPolicy.FileTokenCost,
	}

	return &service{
		codec: codec.NewTiktokenCodec(),
		estimators: map[Protocol]provider.Estimator{
			ProtocolOpenAIChat:      provider.NewOpenAIChat(policy),
			ProtocolOpenAIResponses: provider.NewOpenAIResponses(policy),
			ProtocolAnthropic:       provider.NewAnthropic(policy),
			ProtocolGemini:          provider.NewGemini(policy),
		},
	}
}

func CountText(model string, text string) (int, string, error) {
	return defaultService.CountText(model, text)
}

func Estimate(req EstimateRequest) (EstimateResult, error) {
	return defaultService.Estimate(req)
}

func (s *service) CountText(model string, text string) (int, string, error) {
	encoding, _ := resolveEncodingNote(model)
	count, err := s.codec.Count(encoding, text)
	if err != nil {
		return 0, encoding, err
	}
	return count, encoding, nil
}

func (s *service) Estimate(req EstimateRequest) (EstimateResult, error) {
	model := req.UpstreamModel
	if model == "" {
		model = req.RequestModel
	}

	encoding, encodingNote := resolveEncodingNote(model)
	reported := normalizeReportedUsage(req.ReportedUsage)

	result := EstimateResult{
		Encoding:  encoding,
		Supported: true,
	}

	if reported.HasAny() && !usageNeedsMerge(reported) {
		result.Usage = reported
		result.Source = SourceReportedUsage
		result.Note = joinNotes(encodingNote, "reported usage used directly")
		return result, nil
	}

	estimator, ok := s.estimators[req.Protocol]
	if !ok {
		if reported.HasAny() {
			result.Usage = reported
			result.Source = SourceReportedUsage
			result.Note = joinNotes(encodingNote, "protocol unsupported, reported usage returned as-is")
			return result, nil
		}
		result.Source = SourceUnsupported
		result.Supported = false
		result.Note = joinNotes(encodingNote, fmt.Sprintf("unsupported protocol: %s", req.Protocol))
		return result, nil
	}

	promptResult, err := estimator.ExtractPrompt(req.RequestBody)
	if err != nil {
		return EstimateResult{}, fmt.Errorf("extract prompt: %w", err)
	}

	completionResult, err := estimator.ExtractCompletion(req.ResponseBody, req.IsStream)
	if err != nil {
		return EstimateResult{}, fmt.Errorf("extract completion: %w", err)
	}

	localUsage := Usage{}

	if promptResult.Text != "" || promptResult.ExtraTokens > 0 {
		count, err := s.codec.Count(encoding, promptResult.Text)
		if err != nil {
			return EstimateResult{}, fmt.Errorf("count prompt tokens: %w", err)
		}
		localUsage.PromptTokens = count + promptResult.ExtraTokens
		result.PromptTextLen = utf8.RuneCountInString(promptResult.Text)
	}

	if completionResult.Text != "" || completionResult.ExtraTokens > 0 {
		count, err := s.codec.Count(encoding, completionResult.Text)
		if err != nil {
			return EstimateResult{}, fmt.Errorf("count completion tokens: %w", err)
		}
		localUsage.CompletionTokens = count + completionResult.ExtraTokens
		result.CompletionTextLen = utf8.RuneCountInString(completionResult.Text)
	}

	localUsage = NormalizeUsage(localUsage)

	switch {
	case reported.HasAny() && localUsage.HasAny():
		result.Usage = MergeUsage(reported, localUsage)
		result.Source = SourceMerged
		result.Note = joinNotes(
			encodingNote,
			promptResult.Note,
			completionResult.Note,
			"reported usage incomplete, merged with local estimate",
		)
	case reported.HasAny():
		result.Usage = reported
		result.Source = SourceReportedUsage
		result.Note = joinNotes(
			encodingNote,
			promptResult.Note,
			completionResult.Note,
			"local estimate unavailable, reported usage returned",
		)
	case localUsage.HasAny():
		result.Usage = localUsage
		result.Source = SourceLocalEstimate
		result.Note = joinNotes(encodingNote, promptResult.Note, completionResult.Note, "local estimate used")
	default:
		result.Source = SourceUnsupported
		result.Supported = promptResult.Supported || completionResult.Supported
		if !result.Supported {
			result.Note = joinNotes(encodingNote, promptResult.Note, completionResult.Note, "protocol payload unsupported")
		} else {
			result.Note = joinNotes(encodingNote, promptResult.Note, completionResult.Note, "no text extracted from payload")
		}
	}

	return result, nil
}

func joinNotes(notes ...string) string {
	filtered := make([]string, 0, len(notes))
	seen := map[string]struct{}{}
	for _, note := range notes {
		note = strings.TrimSpace(note)
		if note == "" {
			continue
		}
		if _, ok := seen[note]; ok {
			continue
		}
		seen[note] = struct{}{}
		filtered = append(filtered, note)
	}
	return strings.Join(filtered, "; ")
}
