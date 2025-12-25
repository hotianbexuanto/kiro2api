package config

import (
	"os"
	"strconv"
	"time"
)

// ModelMap 模型映射表
var ModelMap = map[string]string{
	// Sonnet 4.5 系列
	"claude-sonnet-4-5":          "CLAUDE_SONNET_4_5_20250929_V1_0",
	"claude-sonnet-4-5-20250929": "CLAUDE_SONNET_4_5_20250929_V1_0",
	// Sonnet 4 系列
	"claude-sonnet-4-0":        "CLAUDE_SONNET_4_20250514_V1_0",
	"claude-sonnet-4-20250514": "CLAUDE_SONNET_4_20250514_V1_0",
	// Sonnet 3.7 系列
	"claude-3-7-sonnet-latest":   "CLAUDE_3_7_SONNET_20250219_V1_0",
	"claude-3-7-sonnet-20250219": "CLAUDE_3_7_SONNET_20250219_V1_0",
	// Haiku 4.5 系列
	"claude-haiku-4-5":          "auto",
	"claude-haiku-4-5-20251001": "auto",
	// Haiku 3.5 系列
	"claude-3-5-haiku-20241022": "auto",
	// Opus 4.5 系列
	"claude-opus-4-5":          "claude-opus-4.5",
	"claude-opus-4-5-20251101": "claude-opus-4.5",
	// Opus 4.1 系列
	"claude-opus-4-1":          "claude-opus-4.5",
	"claude-opus-4-1-20250805": "claude-opus-4.5",
	// Opus 4 系列
	"claude-opus-4-0":        "claude-opus-4.5",
	"claude-opus-4-20250514": "claude-opus-4.5",
}

// RefreshTokenURL 刷新token的URL (social方式)
const RefreshTokenURL = "https://prod.us-east-1.auth.desktop.kiro.dev/refreshToken"

// IdcRefreshTokenURL IdC认证方式的刷新token URL
const IdcRefreshTokenURL = "https://oidc.us-east-1.amazonaws.com/token"

// CodeWhispererURL CodeWhisperer API的URL
const CodeWhispererURL = "https://codewhisperer.us-east-1.amazonaws.com/generateAssistantResponse"

// Kiro 伪装常量
const (
	KiroSDKVersion = "1.0.27"
	KiroIDEVersion = "0.8.0"
	// 统一操作系统伪装
	KiroOS          = "win32#10.0.19044"
	KiroNodeVersion = "22.17.0"
)

// KiroFingerprint 默认指纹（兼容旧代码）
// 推荐使用 GenerateFingerprint() 动态生成
const KiroFingerprint = "66c23a8c5d15afabec89ef9954ef52a119f10d369df04d548fc6c1eac694b0d1"

// UnsupportedTools CodeWhisperer 不支持的工具名称
// 包括 Anthropic Computer Use 工具和其他特有工具
var UnsupportedTools = map[string]bool{
	// 网络搜索
	"web_search": true,
	"websearch":  true,
	// Anthropic Computer Use
	"computer":             true,
	"computer_20241022":    true,
	"computer_20250124":    true,
	"bash_20241022":        true,
	"bash_20250124":        true,
	"text_editor_20241022": true,
	"text_editor_20250124": true,
	"textEditor_20250429":  true,
	"str_replace_editor":   true,
	// Anthropic Code Execution
	"code_execution":          true,
	"code_execution_20250825": true,
}

// IsUnsupportedTool 检查工具是否不被支持
func IsUnsupportedTool(name string) bool {
	return UnsupportedTools[name]
}

// 重试和冷却配置
const (
	// MaxRetries 最大重试次数
	MaxRetries = 2
	// TokenCooldownDuration Token 冷却时间（限流/失败后）
	TokenCooldownDuration = 30 * time.Second
	// TokenCacheTTL Token 缓存有效期
	TokenCacheTTL = 5 * time.Minute
	// RetryableStatusCodes 可重试的 HTTP 状态码
)

// HTTP 客户端配置
const (
	// HTTPClientKeepAlive HTTP 连接保活时间
	HTTPClientKeepAlive = 30 * time.Second
	// HTTPClientTLSHandshakeTimeout TLS 握手超时时间
	HTTPClientTLSHandshakeTimeout = 10 * time.Second
)

// RetryableStatusCodes 可重试的状态码集合
var RetryableStatusCodes = map[int]bool{
	429: true, // Too Many Requests
	500: true, // Internal Server Error
	502: true, // Bad Gateway
	503: true, // Service Unavailable
	504: true, // Gateway Timeout
}

// IsRetryableStatus 检查状态码是否可重试
func IsRetryableStatus(code int) bool {
	return RetryableStatusCodes[code]
}

// MaxToolDescriptionLength 工具描述的最大长度（字符数）
// 可通过环境变量 MAX_TOOL_DESCRIPTION_LENGTH 配置，默认 10000
var MaxToolDescriptionLength = getEnvIntWithDefault("MAX_TOOL_DESCRIPTION_LENGTH", 10000)

// getEnvIntWithDefault 获取整数类型环境变量（带默认值）
func getEnvIntWithDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
