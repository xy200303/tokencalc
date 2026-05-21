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
	estimators map[Protocol]Estimator
}

type preparedPayloads struct {
	estimator provider.PreparedEstimator
	request   provider.RequestPayload
	response  provider.ResponsePayload
	reqReady  bool
	respReady bool
	reqErr    error
	respErr   error
}

var defaultService BatchService = New()

func New(options ...Option) BatchService {
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

	estimators := defaultEstimators(policy)
	if cfg.Registry != nil {
		for protocol, estimator := range cfg.Registry.estimatorEntries() {
			estimators[protocol] = estimator
		}
	}

	return &service{
		codec:      codec.NewTiktokenCodec(),
		estimators: estimators,
	}
}

func CountText(model string, text string) (int, string, error) {
	return defaultService.CountText(model, text)
}

func Estimate(req EstimateRequest) (EstimateResult, error) {
	return defaultService.Estimate(req)
}

func CountTexts(requests []CountTextRequest) []CountTextResult {
	return defaultService.CountTexts(requests)
}

func EstimateBatch(requests []EstimateRequest) []EstimateBatchResult {
	return defaultService.EstimateBatch(requests)
}

func (s *service) CountText(model string, text string) (int, string, error) {
	encoding, _ := resolveEncodingNote(model)
	count, err := s.codec.Count(encoding, text)
	if err != nil {
		return 0, encoding, err
	}
	return count, encoding, nil
}

func (s *service) CountTexts(requests []CountTextRequest) []CountTextResult {
	results := make([]CountTextResult, len(requests))
	for index, request := range requests {
		count, encoding, err := s.CountText(request.Model, request.Text)
		results[index] = CountTextResult{
			Count:    count,
			Encoding: encoding,
			Error:    err,
		}
	}
	return results
}

func (s *service) EstimateBatch(requests []EstimateRequest) []EstimateBatchResult {
	results := make([]EstimateBatchResult, len(requests))
	for index, request := range requests {
		result, err := s.Estimate(request)
		results[index] = EstimateBatchResult{
			Result: result,
			Error:  err,
		}
	}
	return results
}

func (s *service) Estimate(req EstimateRequest) (EstimateResult, error) {
	estimator, ok := s.estimators[req.Protocol]
	prepared := newPreparedPayloads(estimator)

	model, modelNote, err := resolveEstimateModelWithPrepared(req, prepared)
	if err != nil {
		return EstimateResult{}, err
	}

	encoding, encodingNote := resolveEncodingNote(model)
	callerReported := normalizeReportedUsage(req.ReportedUsage)

	result := EstimateResult{
		ResolvedModel: model,
		Encoding:      encoding,
		Supported:     true,
	}

	if callerReported.HasAny() && !usageNeedsMerge(callerReported) {
		result.Usage = callerReported
		result.Source = SourceReportedUsage
		result.Note = joinNotes(modelNote, encodingNote, "reported usage provided by caller", "reported usage used directly")
		return result, nil
	}

	if !ok {
		if callerReported.HasAny() {
			result.Usage = callerReported
			result.Source = SourceReportedUsage
			result.Note = joinNotes(
				modelNote,
				encodingNote,
				"reported usage provided by caller",
				"protocol unsupported, reported usage returned as-is",
			)
			return result, nil
		}
		result.Source = SourceUnsupported
		result.Supported = false
		result.Note = joinNotes(modelNote, encodingNote, fmt.Sprintf("unsupported protocol: %s", req.Protocol))
		return result, nil
	}

	extractedReported, err := extractReportedUsageWithPrepared(estimator, prepared, req.ResponseBody, req.IsStream)
	if err != nil {
		return EstimateResult{}, fmt.Errorf("extract reported usage: %w", err)
	}

	reported := callerReported
	if extractedReported.Usage.HasAny() {
		reported = MergeUsage(reported, toUsage(extractedReported.Usage))
	}

	if reported.HasAny() && !usageNeedsMerge(reported) {
		result.Usage = reported
		result.Source = SourceReportedUsage
		result.Note = joinNotes(
			modelNote,
			encodingNote,
			reportedUsageOriginNote(callerReported, extractedReported),
			"reported usage used directly",
		)
		return result, nil
	}

	promptResult, err := extractPromptWithPrepared(estimator, prepared, req.RequestBody)
	if err != nil {
		return EstimateResult{}, fmt.Errorf("extract prompt: %w", err)
	}

	completionResult, err := extractCompletionWithPrepared(estimator, prepared, req.ResponseBody, req.IsStream)
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
			modelNote,
			encodingNote,
			reportedUsageOriginNote(callerReported, extractedReported),
			promptResult.Note,
			completionResult.Note,
			"reported usage incomplete, merged with local estimate",
		)
	case reported.HasAny():
		result.Usage = reported
		result.Source = SourceReportedUsage
		result.Note = joinNotes(
			modelNote,
			encodingNote,
			reportedUsageOriginNote(callerReported, extractedReported),
			promptResult.Note,
			completionResult.Note,
			"local estimate unavailable, reported usage returned",
		)
	case localUsage.HasAny():
		result.Usage = localUsage
		result.Source = SourceLocalEstimate
		result.Note = joinNotes(modelNote, encodingNote, promptResult.Note, completionResult.Note, "local estimate used")
	default:
		result.Source = SourceUnsupported
		result.Supported = promptResult.Supported || completionResult.Supported
		if !result.Supported {
			result.Note = joinNotes(modelNote, encodingNote, promptResult.Note, completionResult.Note, "protocol payload unsupported")
		} else {
			result.Note = joinNotes(modelNote, encodingNote, promptResult.Note, completionResult.Note, "no text extracted from payload")
		}
	}

	return result, nil
}

func toUsage(usage provider.ReportedUsage) Usage {
	return Usage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

func defaultEstimators(policy provider.PlaceholderPolicy) map[Protocol]Estimator {
	return map[Protocol]Estimator{
		ProtocolOpenAIChat:      provider.NewOpenAIChat(policy),
		ProtocolOpenAIResponses: provider.NewOpenAIResponses(policy),
		ProtocolAnthropic:       provider.NewAnthropic(policy),
		ProtocolGemini:          provider.NewGemini(policy),
	}
}

func reportedUsageOriginNote(caller Usage, extracted provider.ReportedUsageResult) string {
	switch {
	case caller.HasAny() && extracted.Usage.HasAny():
		return joinNotes("reported usage merged from caller and response body", extracted.Note)
	case caller.HasAny():
		return "reported usage provided by caller"
	case extracted.Usage.HasAny():
		return extracted.Note
	default:
		return ""
	}
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

func newPreparedPayloads(estimator Estimator) *preparedPayloads {
	preparedEstimator, ok := estimator.(provider.PreparedEstimator)
	if !ok {
		return nil
	}
	return &preparedPayloads{estimator: preparedEstimator}
}

func (p *preparedPayloads) Request(body []byte) (provider.RequestPayload, error) {
	if p == nil {
		return provider.RequestPayload{}, nil
	}
	if p.reqReady {
		return p.request, p.reqErr
	}
	p.request, p.reqErr = p.estimator.PrepareRequest(body)
	p.reqReady = true
	return p.request, p.reqErr
}

func (p *preparedPayloads) Response(body []byte, isStream bool) (provider.ResponsePayload, error) {
	if p == nil {
		return provider.ResponsePayload{}, nil
	}
	if p.respReady {
		return p.response, p.respErr
	}
	p.response, p.respErr = p.estimator.PrepareResponse(body, isStream)
	p.respReady = true
	return p.response, p.respErr
}

func resolveEstimateModelWithPrepared(req EstimateRequest, prepared *preparedPayloads) (string, string, error) {
	if model := strings.TrimSpace(req.UpstreamModel); model != "" {
		return model, "", nil
	}
	if model := strings.TrimSpace(req.RequestModel); model != "" {
		return model, "", nil
	}

	if prepared != nil {
		requestPayload, err := prepared.Request(req.RequestBody)
		if err != nil {
			return "", "", fmt.Errorf("extract model from request body: %w", err)
		}
		if model := strings.TrimSpace(prepared.estimator.ExtractRequestModelPrepared(requestPayload)); model != "" {
			return model, "model extracted from request body", nil
		}

		responsePayload, err := prepared.Response(req.ResponseBody, req.IsStream)
		if err != nil {
			return "", "", fmt.Errorf("extract model from response body: %w", err)
		}
		if model := strings.TrimSpace(prepared.estimator.ExtractResponseModelPrepared(responsePayload, req.IsStream)); model != "" {
			return model, "model extracted from response body", nil
		}
	}

	return resolveEstimateModel(req)
}

func extractPromptWithPrepared(estimator Estimator, prepared *preparedPayloads, body []byte) (ExtractResult, error) {
	if prepared == nil {
		return estimator.ExtractPrompt(body)
	}
	requestPayload, err := prepared.Request(body)
	if err != nil {
		return ExtractResult{}, err
	}
	return prepared.estimator.ExtractPromptPrepared(requestPayload)
}

func extractCompletionWithPrepared(estimator Estimator, prepared *preparedPayloads, body []byte, isStream bool) (ExtractResult, error) {
	if prepared == nil {
		return estimator.ExtractCompletion(body, isStream)
	}
	responsePayload, err := prepared.Response(body, isStream)
	if err != nil {
		return ExtractResult{}, err
	}
	return prepared.estimator.ExtractCompletionPrepared(responsePayload, isStream)
}

func extractReportedUsageWithPrepared(estimator Estimator, prepared *preparedPayloads, body []byte, isStream bool) (ReportedUsageResult, error) {
	if prepared == nil {
		return estimator.ExtractReportedUsage(body, isStream)
	}
	responsePayload, err := prepared.Response(body, isStream)
	if err != nil {
		return ReportedUsageResult{}, err
	}
	return prepared.estimator.ExtractReportedUsagePrepared(responsePayload, isStream)
}
