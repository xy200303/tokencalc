package tokencalc

import internalstream "github.com/xy200303/tokencalc/internal/stream"

var builtInStreamCollectorFactories = map[Protocol]StreamCollectorFactory{
	ProtocolOpenAIChat:      func() StreamCollector { return internalstream.NewOpenAICollector() },
	ProtocolOpenAIResponses: func() StreamCollector { return internalstream.NewOpenAICollector() },
	ProtocolAnthropic:       func() StreamCollector { return internalstream.NewAnthropicCollector() },
	ProtocolGemini:          func() StreamCollector { return internalstream.NewGeminiCollector() },
}

type StreamCollector interface {
	AddChunk(part []byte) error
	FinalBody() []byte
}

func NewStreamCollector(protocol Protocol) (StreamCollector, error) {
	return NewStreamCollectorWithOptions(protocol)
}

func NewStreamCollectorWithOptions(protocol Protocol, options ...Option) (StreamCollector, error) {
	cfg := Config{}
	for _, option := range options {
		option(&cfg)
	}

	if cfg.Registry != nil {
		if collector, err := cfg.Registry.NewStreamCollector(protocol); err == nil {
			return collector, nil
		}
	}

	factory, ok := defaultStreamCollectorFactories()[protocol]
	if !ok {
		return nil, ErrUnsupportedProtocol{Protocol: protocol}
	}
	return factory(), nil
}

func defaultStreamCollectorFactories() map[Protocol]StreamCollectorFactory {
	return builtInStreamCollectorFactories
}

func newBuiltInStreamCollector(protocol Protocol) (StreamCollector, error) {
	factory, ok := defaultStreamCollectorFactories()[protocol]
	if !ok {
		return nil, ErrUnsupportedProtocol{Protocol: protocol}
	}
	return factory(), nil
}

func (r *Registry) newStreamCollectorOrDefault(protocol Protocol) (StreamCollector, error) {
	if r != nil {
		if collector, err := r.NewStreamCollector(protocol); err == nil {
			return collector, nil
		}
	}
	return newBuiltInStreamCollector(protocol)
}

func NewStreamCollectorFromRegistry(protocol Protocol, registry *Registry) (StreamCollector, error) {
	if registry == nil {
		return newBuiltInStreamCollector(protocol)
	}
	return registry.newStreamCollectorOrDefault(protocol)
}
