package tokencalc

import (
	"errors"
	"testing"
)

type fixedCollector struct {
	body []byte
}

func (c *fixedCollector) AddChunk(part []byte) error {
	c.body = append(c.body, part...)
	return nil
}

func (c *fixedCollector) FinalBody() []byte {
	return append([]byte(nil), c.body...)
}

func TestNewStreamCollectorUnsupported(t *testing.T) {
	t.Parallel()

	_, err := NewStreamCollector("unsupported")
	if err == nil {
		t.Fatal("expected error for unsupported protocol")
	}

	var unsupported ErrUnsupportedProtocol
	if !errors.As(err, &unsupported) {
		t.Fatalf("error = %T, want ErrUnsupportedProtocol", err)
	}
}

func TestNewStreamCollectorWithOptions(t *testing.T) {
	t.Parallel()

	collector, err := NewStreamCollectorWithOptions("custom_stream",
		WithStreamCollectorFactory("custom_stream", func() StreamCollector {
			return &fixedCollector{}
		}),
	)
	if err != nil {
		t.Fatalf("NewStreamCollectorWithOptions() error = %v", err)
	}

	if err := collector.AddChunk([]byte("hello")); err != nil {
		t.Fatalf("AddChunk() error = %v", err)
	}
	if got := string(collector.FinalBody()); got != "hello" {
		t.Fatalf("FinalBody() = %q, want %q", got, "hello")
	}
}

func TestNewStreamCollectorFromRegistryFallsBackToBuiltIns(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	collector, err := NewStreamCollectorFromRegistry(ProtocolOpenAIChat, registry)
	if err != nil {
		t.Fatalf("NewStreamCollectorFromRegistry() error = %v", err)
	}

	if err := collector.AddChunk([]byte("data: {\"x\":1}\n\n")); err != nil {
		t.Fatalf("AddChunk() error = %v", err)
	}
	if got := string(collector.FinalBody()); got == "" {
		t.Fatal("FinalBody() returned empty body")
	}
}

func TestRegistryClonePreservesStreamCollectorFactory(t *testing.T) {
	t.Parallel()

	registry := NewRegistry().RegisterStreamCollector("custom_stream", func() StreamCollector {
		return &fixedCollector{body: []byte("one")}
	})
	cloned := registry.Clone()

	registry.RegisterStreamCollector("custom_stream", func() StreamCollector {
		return &fixedCollector{body: []byte("two")}
	})

	collector, err := NewStreamCollectorFromRegistry("custom_stream", cloned)
	if err != nil {
		t.Fatalf("NewStreamCollectorFromRegistry() error = %v", err)
	}
	if got := string(collector.FinalBody()); got != "one" {
		t.Fatalf("FinalBody() = %q, want %q", got, "one")
	}
}
