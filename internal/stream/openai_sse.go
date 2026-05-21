package stream

func NewOpenAICollector() Collector {
	return NewPassthroughCollector()
}
