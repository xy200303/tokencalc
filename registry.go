package tokencalc

import (
	"sync"

	"github.com/xy200303/tokencalc/internal/provider"
)

type ExtractResult = provider.ExtractResult
type ReportedUsage = provider.ReportedUsage
type ReportedUsageResult = provider.ReportedUsageResult

type Estimator interface {
	ExtractPrompt(body []byte) (ExtractResult, error)
	ExtractCompletion(body []byte, isStream bool) (ExtractResult, error)
	ExtractReportedUsage(body []byte, isStream bool) (ReportedUsageResult, error)
}

type StreamCollectorFactory func() StreamCollector

type Registry struct {
	mu               sync.RWMutex
	estimators       map[Protocol]Estimator
	streamCollectors map[Protocol]StreamCollectorFactory
}

func NewRegistry() *Registry {
	return &Registry{
		estimators:       make(map[Protocol]Estimator),
		streamCollectors: make(map[Protocol]StreamCollectorFactory),
	}
}

func (r *Registry) Register(protocol Protocol, estimator Estimator) *Registry {
	if r == nil || protocol == "" || estimator == nil {
		return r
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.estimators == nil {
		r.estimators = make(map[Protocol]Estimator)
	}
	r.estimators[protocol] = estimator
	return r
}

func (r *Registry) Estimator(protocol Protocol) (Estimator, bool) {
	if r == nil {
		return nil, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	estimator, ok := r.estimators[protocol]
	return estimator, ok
}

func (r *Registry) RegisterStreamCollector(protocol Protocol, factory StreamCollectorFactory) *Registry {
	if r == nil || protocol == "" || factory == nil {
		return r
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.streamCollectors == nil {
		r.streamCollectors = make(map[Protocol]StreamCollectorFactory)
	}
	r.streamCollectors[protocol] = factory
	return r
}

func (r *Registry) StreamCollectorFactory(protocol Protocol) (StreamCollectorFactory, bool) {
	if r == nil {
		return nil, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, ok := r.streamCollectors[protocol]
	return factory, ok
}

func (r *Registry) NewStreamCollector(protocol Protocol) (StreamCollector, error) {
	factory, ok := r.StreamCollectorFactory(protocol)
	if !ok {
		return nil, ErrUnsupportedProtocol{Protocol: protocol}
	}
	return factory(), nil
}

func (r *Registry) Clone() *Registry {
	cloned := NewRegistry()
	if r == nil {
		return cloned
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for protocol, estimator := range r.estimators {
		cloned.estimators[protocol] = estimator
	}
	for protocol, factory := range r.streamCollectors {
		cloned.streamCollectors[protocol] = factory
	}
	return cloned
}

func (r *Registry) estimatorEntries() map[Protocol]Estimator {
	if r == nil {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	cloned := make(map[Protocol]Estimator, len(r.estimators))
	for protocol, estimator := range r.estimators {
		cloned[protocol] = estimator
	}
	return cloned
}

func (r *Registry) streamCollectorEntries() map[Protocol]StreamCollectorFactory {
	if r == nil {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	cloned := make(map[Protocol]StreamCollectorFactory, len(r.streamCollectors))
	for protocol, factory := range r.streamCollectors {
		cloned[protocol] = factory
	}
	return cloned
}
