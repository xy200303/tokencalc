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

type CountTextRequest struct {
	Model string
	Text  string
}

type CountTextResult struct {
	Count    int
	Encoding string
	Error    error
}

type EstimateResult struct {
	Usage             Usage
	ResolvedModel     string
	Source            EstimateSource
	Encoding          string
	Supported         bool
	Note              string
	PromptTextLen     int
	CompletionTextLen int
}

type EstimateBatchResult struct {
	Result EstimateResult
	Error  error
}

type StreamEstimateUpdate struct {
	Result  EstimateResult
	Delta   Usage
	Updated bool
}

type Service interface {
	Estimate(req EstimateRequest) (EstimateResult, error)
	CountText(model string, text string) (count int, encoding string, err error)
}

type BatchService interface {
	Service
	EstimateBatch(requests []EstimateRequest) []EstimateBatchResult
	CountTexts(requests []CountTextRequest) []CountTextResult
}

type Config struct {
	PlaceholderPolicy PlaceholderPolicy
	Registry          *Registry
}

type Option func(*Config)

func WithPlaceholderPolicy(policy PlaceholderPolicy) Option {
	return func(cfg *Config) {
		cfg.PlaceholderPolicy = policy
	}
}

func WithRegistry(registry *Registry) Option {
	return func(cfg *Config) {
		if registry == nil {
			cfg.Registry = nil
			return
		}
		cfg.Registry = registry.Clone()
	}
}

func WithEstimator(protocol Protocol, estimator Estimator) Option {
	return func(cfg *Config) {
		if protocol == "" || estimator == nil {
			return
		}
		if cfg.Registry == nil {
			cfg.Registry = NewRegistry()
		}
		cfg.Registry.Register(protocol, estimator)
	}
}

func WithStreamCollectorFactory(protocol Protocol, factory StreamCollectorFactory) Option {
	return func(cfg *Config) {
		if protocol == "" || factory == nil {
			return
		}
		if cfg.Registry == nil {
			cfg.Registry = NewRegistry()
		}
		cfg.Registry.RegisterStreamCollector(protocol, factory)
	}
}
