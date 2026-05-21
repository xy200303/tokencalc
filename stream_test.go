package tokencalc

import "testing"

func TestNewStreamCollectorUnsupported(t *testing.T) {
	t.Parallel()

	if _, err := NewStreamCollector("unsupported"); err == nil {
		t.Fatal("expected error for unsupported protocol")
	}
}
