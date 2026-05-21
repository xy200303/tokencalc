package tokencalc

import (
	"errors"
	"io"
	"strings"
	"sync"
)

type StreamingCounter struct {
	mu        sync.Mutex
	service   BatchService
	collector StreamCollector
	req       EstimateRequest
	last      EstimateResult
	hasLast   bool
}

func NewStreamingCounter(req EstimateRequest, options ...Option) (*StreamingCounter, error) {
	collector, err := NewStreamCollectorWithOptions(req.Protocol, options...)
	if err != nil {
		return nil, err
	}

	req.IsStream = true

	return &StreamingCounter{
		service:   New(options...),
		collector: collector,
		req:       req,
	}, nil
}

func (c *StreamingCounter) AddChunk(part []byte) (StreamEstimateUpdate, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.collector.AddChunk(part); err != nil {
		return StreamEstimateUpdate{}, err
	}

	return c.recalculate(false)
}

func (c *StreamingCounter) FinalResult() (StreamEstimateUpdate, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.recalculate(true)
}

func (c *StreamingCounter) FinalBody() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.collector.FinalBody()
}

func (c *StreamingCounter) recalculate(strict bool) (StreamEstimateUpdate, error) {
	req := c.req
	req.ResponseBody = c.collector.FinalBody()
	req.IsStream = true

	result, err := c.service.Estimate(req)
	if err != nil {
		if !strict && isIncompleteStreamError(err) {
			return c.currentUpdate(), nil
		}
		return c.currentUpdate(), err
	}

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
	return update, nil
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
