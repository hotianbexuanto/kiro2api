package stats

import "time"

// RequestRecord 单条请求记录
type RequestRecord struct {
	ID                   string    `json:"id"`
	Timestamp            time.Time `json:"timestamp"`
	Method               string    `json:"method"`
	Path                 string    `json:"path"`
	RequestType          string    `json:"request_type,omitempty"` // "anthropic" | "openai"
	Model                string    `json:"model"`
	Stream               bool      `json:"stream"`
	StatusCode           int       `json:"status_code"`
	Latency              int64     `json:"latency"`              // ms - 总耗时
	TTFB                 int64     `json:"ttfb,omitempty"`       // ms - 首字时间 (Time To First Byte)，仅流式请求
	InputTokens          int       `json:"input_tokens"`
	OutputTokens         int       `json:"output_tokens"`
	CacheReadInputTokens int       `json:"cache_read_input_tokens,omitempty"`
	CacheCreationTokens  int       `json:"cache_creation_input_tokens,omitempty"`
	CreditUsage          float64   `json:"credit_usage,omitempty"`          // 来自 meteringEvent
	ContextUsagePercent  float64   `json:"context_usage_percent,omitempty"` // 来自 contextUsageEvent
	TokenIndex           int       `json:"token_index"`
	ConversationId       string    `json:"conversation_id,omitempty"`
	Group                string    `json:"group"`
	Error                string    `json:"error,omitempty"`
}

// Stats 统计摘要
type Stats struct {
	TotalRequests   int64                `json:"total_requests"`
	SuccessRequests int64                `json:"success_requests"`
	FailedRequests  int64                `json:"failed_requests"`
	AvgLatency      float64              `json:"avg_latency"`
	ModelUsage      map[string]int64     `json:"model_usage"`
	PathUsage       map[string]int64     `json:"path_usage"`
	HourlyRequests  [24]int64            `json:"hourly_requests"`
	HourlyByModel   map[string][24]int64 `json:"hourly_by_model"`
	HourlyCredit    [24]float64          `json:"hourly_credit"`
	RecentRecords   []RequestRecord      `json:"recent_records"`
}
