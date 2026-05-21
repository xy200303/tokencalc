package codec

type Counter interface {
	Count(encoding string, text string) (int, error)
}
