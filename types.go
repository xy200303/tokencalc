package tokencalc

type Protocol string

const (
	ProtocolOpenAIChat      Protocol = "openai_chat"
	ProtocolOpenAIResponses Protocol = "openai_responses"
	ProtocolAnthropic       Protocol = "anthropic_messages"
	ProtocolGemini          Protocol = "gemini_contents"
)

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (u Usage) HasAny() bool {
	return u.PromptTokens > 0 || u.CompletionTokens > 0 || u.TotalTokens > 0
}

type EstimateSource string

const (
	SourceReportedUsage EstimateSource = "reported_usage"
	SourceLocalEstimate EstimateSource = "local_estimate"
	SourceMerged        EstimateSource = "merged"
	SourceUnsupported   EstimateSource = "unsupported"
)

type EstimateRequest struct {
	Protocol      Protocol
	RequestModel  string
	UpstreamModel string
	IsStream      bool
	RequestBody   []byte
	ResponseBody  []byte
	ReportedUsage *Usage
}

type EstimateResult struct {
	Usage             Usage
	Source            EstimateSource
	Encoding          string
	Supported         bool
	Note              string
	PromptTextLen     int
	CompletionTextLen int
}

type Service interface {
	Estimate(req EstimateRequest) (EstimateResult, error)
	CountText(model string, text string) (count int, encoding string, err error)
}

type Config struct {
	PlaceholderPolicy PlaceholderPolicy
}

type Option func(*Config)

func WithPlaceholderPolicy(policy PlaceholderPolicy) Option {
	return func(cfg *Config) {
		cfg.PlaceholderPolicy = policy
	}
}
