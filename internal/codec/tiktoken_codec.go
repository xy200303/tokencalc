package codec

import (
	"fmt"
	"strings"
	"sync"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

type TiktokenCodec struct {
	mu       sync.RWMutex
	encoders map[string]*tiktoken.Tiktoken
}

func NewTiktokenCodec() *TiktokenCodec {
	return &TiktokenCodec{
		encoders: make(map[string]*tiktoken.Tiktoken),
	}
}

func (c *TiktokenCodec) Count(encoding string, text string) (int, error) {
	encoding = strings.TrimSpace(encoding)
	if encoding == "" {
		return 0, fmt.Errorf("encoding is required")
	}

	tk, err := c.get(encoding)
	if err != nil {
		return 0, err
	}

	return len(tk.EncodeOrdinary(text)), nil
}

func (c *TiktokenCodec) get(encoding string) (*tiktoken.Tiktoken, error) {
	c.mu.RLock()
	tk, ok := c.encoders[encoding]
	c.mu.RUnlock()
	if ok {
		return tk, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if tk, ok := c.encoders[encoding]; ok {
		return tk, nil
	}

	loaded, err := tiktoken.GetEncoding(encoding)
	if err != nil {
		return nil, fmt.Errorf("load encoding %q: %w", encoding, err)
	}
	c.encoders[encoding] = loaded
	return loaded, nil
}
