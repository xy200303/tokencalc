package tokencalc

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/xy200303/tokencalc/internal/provider"
)

type StreamingCounter struct {
	mu        sync.Mutex
	service   *service
	options   []Option
	collector StreamCollector
	req       EstimateRequest
	prompt    EstimateResult
	usePrompt bool
	parser    streamEventParser
	acc       provider.StreamAccumulator
	fastPath  bool
	last      EstimateResult
	hasLast   bool
	dirty     bool
}

func NewStreamingCounter(req EstimateRequest, options ...Option) (*StreamingCounter, error) {
	counter := &StreamingCounter{
		service: newService(options...),
		options: append([]Option(nil), options...),
	}
	if err := counter.resetLocked(req); err != nil {
		return nil, err
	}
	return counter, nil
}

func (c *StreamingCounter) AddChunk(part []byte) (StreamEstimateUpdate, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.collector.AddChunk(part); err != nil {
		return StreamEstimateUpdate{}, err
	}
	c.dirty = true

	if c.fastPath {
		return c.processIncremental(part, false)
	}

	return c.recalculate(false)
}

func (c *StreamingCounter) FinalResult() (StreamEstimateUpdate, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.dirty && c.hasLast {
		return c.currentUpdate(), nil
	}

	if c.fastPath {
		return c.processIncremental(nil, true)
	}

	return c.recalculate(true)
}

func (c *StreamingCounter) FinalBody() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.collector.FinalBody()
}

func (c *StreamingCounter) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.resetLocked(c.req)
}

func (c *StreamingCounter) Reset(req EstimateRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.resetLocked(req)
}

func (c *StreamingCounter) ResetRequestBody(requestBody []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := c.req
	req.RequestBody = requestBody
	return c.resetLocked(req)
}

func (c *StreamingCounter) recalculate(strict bool) (StreamEstimateUpdate, error) {
	result, err := c.estimateCurrent()
	if err != nil {
		if !strict && isIncompleteStreamError(err) {
			return c.currentUpdate(), nil
		}
		return c.currentUpdate(), err
	}

	return c.finishUpdate(result), nil
}

func (c *StreamingCounter) finishUpdate(result EstimateResult) StreamEstimateUpdate {
	update := StreamEstimateUpdate{
		Result:  result,
		Updated: true,
	}

	if c.hasLast {
		if result == c.last {
			update.Updated = false
		} else {
			update.Delta = usageDiff(result.Usage, c.last.Usage)
		}
	} else {
		update.Delta = NormalizeUsage(result.Usage)
	}

	c.last = result
	c.hasLast = true
	c.dirty = false
	return update
}

func (c *StreamingCounter) currentUpdate() StreamEstimateUpdate {
	if !c.hasLast {
		return StreamEstimateUpdate{}
	}
	return StreamEstimateUpdate{Result: c.last}
}

func usageDiff(current Usage, previous Usage) Usage {
	return NormalizeUsage(Usage{
		PromptTokens:     current.PromptTokens - previous.PromptTokens,
		CompletionTokens: current.CompletionTokens - previous.CompletionTokens,
		TotalTokens:      current.TotalTokens - previous.TotalTokens,
	})
}

func isIncompleteStreamError(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "unexpected eof")
}

func collectorBody(collector StreamCollector) []byte {
	type bodyViewCollector interface {
		BodyView() []byte
	}

	if view, ok := collector.(bodyViewCollector); ok {
		return view.BodyView()
	}
	return collector.FinalBody()
}

func (c *StreamingCounter) resetLocked(req EstimateRequest) error {
	collector, err := NewStreamCollectorWithOptions(req.Protocol, c.options...)
	if err != nil {
		return err
	}

	req.ResponseBody = nil
	req.IsStream = true

	c.collector = collector
	c.req = req
	c.prompt = EstimateResult{}
	c.usePrompt = false
	c.parser = streamEventParser{}
	c.acc = nil
	c.fastPath = false
	c.last = EstimateResult{}
	c.hasLast = false
	c.dirty = false
	c.preparePromptBase()
	c.prepareFastPath()

	return nil
}

func (c *StreamingCounter) preparePromptBase() {
	if len(c.req.RequestBody) == 0 || c.req.ReportedUsage != nil {
		return
	}

	promptReq := c.req
	promptReq.ResponseBody = nil
	promptReq.IsStream = false

	promptResult, err := c.service.estimatePromptOnly(promptReq)
	if err != nil {
		return
	}
	if strings.TrimSpace(promptResult.ResolvedModel) == "" {
		return
	}

	c.prompt = promptResult
	c.usePrompt = true
}

func (c *StreamingCounter) prepareFastPath() {
	estimator, ok := c.service.estimators[c.req.Protocol]
	if !ok {
		return
	}

	incremental, ok := estimator.(provider.IncrementalStreamEstimator)
	if !ok {
		return
	}

	c.acc = incremental.NewStreamAccumulator()
	c.fastPath = c.acc != nil
}

func (c *StreamingCounter) estimateCurrent() (EstimateResult, error) {
	body := collectorBody(c.collector)
	if !c.usePrompt {
		req := c.req
		req.ResponseBody = body
		req.IsStream = true
		return c.service.Estimate(req)
	}

	responseReq := EstimateRequest{
		Protocol:     c.req.Protocol,
		RequestModel: c.prompt.ResolvedModel,
		IsStream:     true,
		ResponseBody: body,
	}
	if model := strings.TrimSpace(c.req.UpstreamModel); model != "" {
		responseReq.UpstreamModel = model
		responseReq.RequestModel = ""
	}

	responseResult, err := c.service.estimateResponseOnly(responseReq)
	if err != nil {
		return EstimateResult{}, err
	}

	return combinePromptAndResponse(c.prompt, responseResult), nil
}

func (c *StreamingCounter) processIncremental(part []byte, strict bool) (StreamEstimateUpdate, error) {
	events := c.parser.AddChunk(part, strict)
	if len(events) == 0 {
		if len(bytes.TrimSpace(c.parser.pending)) == 0 {
			c.dirty = false
		}
		return c.currentUpdate(), nil
	}

	for _, event := range events {
		payload, err := decodeAnyObject(event)
		if err != nil {
			if !strict && isIncompleteStreamError(err) {
				return c.currentUpdate(), nil
			}
			return c.currentUpdate(), err
		}
		c.acc.AddEvent(payload)
	}

	result, err := c.estimateCurrentIncremental()
	if err != nil {
		return c.currentUpdate(), err
	}

	return c.finishUpdate(result), nil
}

func (c *StreamingCounter) estimateCurrentIncremental() (EstimateResult, error) {
	model, modelNote := c.incrementalResolvedModel()
	encoding, encodingNote := resolveEncodingNote(model)
	callerReported := normalizeReportedUsage(c.req.ReportedUsage)
	extractedReported := c.acc.ReportedUsage()
	reported := callerReported
	if extractedReported.Usage.HasAny() {
		reported = MergeUsage(reported, toUsage(extractedReported.Usage))
	}

	completionResult := c.acc.Completion()
	result := EstimateResult{
		ResolvedModel: model,
		Encoding:      encoding,
		Supported:     true,
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
		return c.combineIncrementalPrompt(result), nil
	}

	localUsage := Usage{}
	if completionResult.Text != "" || completionResult.ExtraTokens > 0 {
		count, err := c.service.codec.Count(encoding, completionResult.Text)
		if err != nil {
			return EstimateResult{}, err
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
			completionResult.Note,
			"local estimate unavailable, reported usage returned",
		)
	case localUsage.HasAny():
		result.Usage = localUsage
		result.Source = SourceLocalEstimate
		result.Note = joinNotes(modelNote, encodingNote, completionResult.Note, "local estimate used")
	default:
		result.Source = SourceUnsupported
		result.Supported = completionResult.Supported
		if !result.Supported {
			result.Note = joinNotes(modelNote, encodingNote, completionResult.Note, "protocol payload unsupported")
		} else {
			result.Note = joinNotes(modelNote, encodingNote, completionResult.Note, "no text extracted from payload")
		}
	}

	return c.combineIncrementalPrompt(result), nil
}

func (c *StreamingCounter) combineIncrementalPrompt(response EstimateResult) EstimateResult {
	if !c.usePrompt {
		return response
	}
	return combinePromptAndResponse(c.prompt, response)
}

func (c *StreamingCounter) incrementalResolvedModel() (string, string) {
	if model := strings.TrimSpace(c.req.UpstreamModel); model != "" {
		return model, ""
	}
	if model := strings.TrimSpace(c.req.RequestModel); model != "" {
		return model, ""
	}
	if c.usePrompt && strings.TrimSpace(c.prompt.ResolvedModel) != "" {
		return c.prompt.ResolvedModel, ""
	}
	if c.acc != nil {
		if model := strings.TrimSpace(c.acc.Model()); model != "" {
			return model, "model extracted from response body"
		}
	}
	return "", ""
}

func combinePromptAndResponse(prompt EstimateResult, response EstimateResult) EstimateResult {
	result := response

	if strings.TrimSpace(result.ResolvedModel) == "" {
		result.ResolvedModel = prompt.ResolvedModel
	}
	if strings.TrimSpace(result.Encoding) == "" {
		result.Encoding = prompt.Encoding
	}

	result.PromptTextLen = prompt.PromptTextLen
	result.Supported = prompt.Supported || response.Supported
	result.Note = joinNotes(prompt.Note, response.Note)
	result.Usage = mergePromptUsage(prompt.Usage, response.Usage)

	if result.Source == SourceUnsupported && prompt.Usage.HasAny() {
		result.Source = SourceLocalEstimate
	}

	return result
}

func mergePromptUsage(prompt Usage, current Usage) Usage {
	current = NormalizeUsage(current)
	prompt = NormalizeUsage(prompt)

	if current.PromptTokens == 0 {
		current.PromptTokens = prompt.PromptTokens
	}

	if current.TotalTokens == 0 || current.TotalTokens < current.PromptTokens+current.CompletionTokens {
		current.TotalTokens = current.PromptTokens + current.CompletionTokens
	}

	return NormalizeUsage(current)
}

type streamEventParser struct {
	pending []byte
}

func (p *streamEventParser) AddChunk(part []byte, flush bool) [][]byte {
	buf := make([]byte, 0, len(p.pending)+len(part))
	buf = append(buf, p.pending...)
	buf = append(buf, part...)

	events := make([][]byte, 0)
	start := 0
	for index := 0; index < len(buf); index++ {
		if buf[index] != '\n' {
			continue
		}
		p.appendLineEvents(buf[start:index], &events)
		start = index + 1
	}

	if flush {
		p.appendLineEvents(buf[start:], &events)
		p.pending = nil
		return events
	}

	p.pending = append(p.pending[:0], buf[start:]...)
	return events
}

func (p *streamEventParser) appendLineEvents(line []byte, events *[][]byte) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return
	}

	if bytes.HasPrefix(line, []byte("event:")) {
		return
	}

	if bytes.HasPrefix(line, []byte("data:")) {
		payload := bytes.TrimSpace(line[len("data:"):])
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			return
		}
		*events = append(*events, payload)
		return
	}

	if line[0] == '{' || line[0] == '[' {
		*events = append(*events, line)
	}
}
