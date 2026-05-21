package stream

import (
	"bufio"
	"bytes"
	"strings"
	"sync"
)

type Collector interface {
	AddChunk(part []byte) error
	FinalBody() []byte
}

type bytesCollector struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func NewPassthroughCollector() Collector {
	return &bytesCollector{}
}

func (c *bytesCollector) AddChunk(part []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.buf.Write(part)
	return err
}

func (c *bytesCollector) FinalBody() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte(nil), c.buf.Bytes()...)
}

func ExtractEvents(body []byte) [][]byte {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil
	}

	if bytes.Contains(trimmed, []byte("data:")) || bytes.Contains(trimmed, []byte("event:")) {
		return extractSSEData(trimmed)
	}

	lines := extractJSONLines(trimmed)
	if len(lines) > 0 {
		return lines
	}

	return [][]byte{trimmed}
}

func extractSSEData(body []byte) [][]byte {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	events := make([][]byte, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		events = append(events, []byte(data))
	}
	return events
}

func extractJSONLines(body []byte) [][]byte {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	lines := make([][]byte, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "{") || strings.HasPrefix(line, "[") {
			lines = append(lines, []byte(line))
		}
	}
	return lines
}
