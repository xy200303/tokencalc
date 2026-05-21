package tokencalc

import "fmt"

type ErrUnsupportedProtocol struct {
	Protocol Protocol
}

func (e ErrUnsupportedProtocol) Error() string {
	return fmt.Sprintf("unsupported protocol: %s", e.Protocol)
}
