package tokencalc

import (
	"fmt"

	internalstream "github.com/xy200303/tokencalc/internal/stream"
)

type StreamCollector interface {
	AddChunk(part []byte) error
	FinalBody() []byte
}

func NewStreamCollector(protocol Protocol) (StreamCollector, error) {
	switch protocol {
	case ProtocolOpenAIChat, ProtocolOpenAIResponses:
		return internalstream.NewOpenAICollector(), nil
	case ProtocolAnthropic:
		return internalstream.NewAnthropicCollector(), nil
	case ProtocolGemini:
		return internalstream.NewGeminiCollector(), nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}
}
