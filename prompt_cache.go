package tokencalc

import (
	"bytes"
	"container/list"
	"hash/maphash"
	"sync"
)

type promptCacheKey struct {
	protocol      Protocol
	requestModel  string
	upstreamModel string
	bodyHash      uint64
	bodyLen       int
}

type promptCacheEntry struct {
	key    promptCacheKey
	body   []byte
	result EstimateResult
}

type promptResultCache struct {
	mu      sync.Mutex
	seed    maphash.Seed
	maxSize int
	ll      *list.List
	items   map[promptCacheKey]*list.Element
}

func newPromptResultCache(maxSize int) *promptResultCache {
	return &promptResultCache{
		seed:    maphash.MakeSeed(),
		maxSize: maxSize,
		ll:      list.New(),
		items:   make(map[promptCacheKey]*list.Element, maxSize),
	}
}

func (c *promptResultCache) get(req EstimateRequest) (EstimateResult, bool) {
	if c == nil || len(req.RequestBody) == 0 {
		return EstimateResult{}, false
	}

	key := c.makeKey(req)

	c.mu.Lock()
	defer c.mu.Unlock()

	element, ok := c.items[key]
	if !ok {
		return EstimateResult{}, false
	}

	entry := element.Value.(*promptCacheEntry)
	if !bytes.Equal(entry.body, req.RequestBody) {
		return EstimateResult{}, false
	}

	c.ll.MoveToFront(element)
	return entry.result, true
}

func (c *promptResultCache) put(req EstimateRequest, result EstimateResult) {
	if c == nil || len(req.RequestBody) == 0 || !result.Usage.HasAny() {
		return
	}

	key := c.makeKey(req)

	c.mu.Lock()
	defer c.mu.Unlock()

	if element, ok := c.items[key]; ok {
		entry := element.Value.(*promptCacheEntry)
		entry.body = append(entry.body[:0], req.RequestBody...)
		entry.result = result
		c.ll.MoveToFront(element)
		return
	}

	entry := &promptCacheEntry{
		key:    key,
		body:   append([]byte(nil), req.RequestBody...),
		result: result,
	}
	element := c.ll.PushFront(entry)
	c.items[key] = element

	if c.ll.Len() <= c.maxSize {
		return
	}

	oldest := c.ll.Back()
	if oldest == nil {
		return
	}
	c.ll.Remove(oldest)
	oldEntry := oldest.Value.(*promptCacheEntry)
	delete(c.items, oldEntry.key)
}

func (c *promptResultCache) makeKey(req EstimateRequest) promptCacheKey {
	var hash maphash.Hash
	hash.SetSeed(c.seed)
	_, _ = hash.WriteString(string(req.Protocol))
	_, _ = hash.WriteString("\x00")
	_, _ = hash.WriteString(req.RequestModel)
	_, _ = hash.WriteString("\x00")
	_, _ = hash.WriteString(req.UpstreamModel)
	_, _ = hash.Write(req.RequestBody)

	return promptCacheKey{
		protocol:      req.Protocol,
		requestModel:  req.RequestModel,
		upstreamModel: req.UpstreamModel,
		bodyHash:      hash.Sum64(),
		bodyLen:       len(req.RequestBody),
	}
}

func (s *service) estimatePromptOnlyCached(req EstimateRequest) (EstimateResult, error) {
	if result, ok := s.prompt.get(req); ok {
		return result, nil
	}

	result, err := s.estimatePromptOnly(req)
	if err != nil {
		return EstimateResult{}, err
	}
	s.prompt.put(req, result)
	return result, nil
}
