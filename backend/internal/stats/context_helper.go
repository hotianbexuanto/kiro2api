package stats

import (
	"kiro2api/internal/auth"

	"github.com/gin-gonic/gin"
)

// SetStatsModel 设置统计模型
func SetModel(c *gin.Context, model string) {
	c.Set("stats_model", model)
}

func SetRequestType(c *gin.Context, requestType string) {
	c.Set("stats_request_type", requestType)
}

func SetStream(c *gin.Context, stream bool) {
	c.Set("stats_stream", stream)
}

// SetStatsTokens 设置统计 token 数
func SetTokens(c *gin.Context, input, output int) {
	c.Set("stats_input_tokens", input)
	c.Set("stats_output_tokens", output)
}

// SetStatsTokenIndex 设置使用的 token 索引
func SetTokenIndex(c *gin.Context, index int) {
	c.Set("stats_token_index", index)
}

// SetConversationId 设置会话ID
func SetConversationId(c *gin.Context, convId string) {
	c.Set("stats_conversation_id", convId)
}

// SetStatsGroup 设置 token 分组
func SetGroup(c *gin.Context, group string) {
	// 空字符串使用默认分组
	if group == "" {
		group = auth.GetDefaultGroup()
	}
	c.Set("stats_group", group)
}

// SetStatsError 设置错误信息
func SetError(c *gin.Context, err string) {
	c.Set("stats_error", err)
}

// SetStatsCacheTokens 设置缓存 token 数
func SetCacheTokens(c *gin.Context, cacheRead, cacheCreation int) {
	c.Set("stats_cache_read", cacheRead)
	c.Set("stats_cache_creation", cacheCreation)
}

// SetStatsCreditUsage 设置 credit 使用量（来自 meteringEvent）
func SetCreditUsage(c *gin.Context, usage float64) {
	c.Set("stats_credit_usage", usage)
}

// SetStatsContextUsage 设置上下文使用百分比（来自 contextUsageEvent）
func SetContextUsage(c *gin.Context, percent float64) {
	c.Set("stats_context_usage", percent)
}

// SetTTFB 设置首字时间 (Time To First Byte)
func SetTTFB(c *gin.Context, ttfb int64) {
	c.Set("stats_ttfb", ttfb)
}
