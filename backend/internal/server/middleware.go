package server

import (
	"net/http"
	"strings"
	"time"

	"kiro2api/internal/auth"
	"kiro2api/internal/logger"
	"kiro2api/internal/server/handler"
	"kiro2api/internal/stats"
	"kiro2api/internal/utils"

	"github.com/gin-gonic/gin"
)

// PathBasedAuthMiddleware 创建基于路径的API密钥验证中间件
func PathBasedAuthMiddleware(keyMgr *auth.APIKeyManager, protectedPrefixes []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 检查是否需要认证
		if !requiresAuth(path, protectedPrefixes) {
			logger.Debug("跳过认证", logger.String("path", path))
			c.Next()
			return
		}

		keyConfig := validateAPIKey(c, keyMgr)
		if keyConfig == nil {
			c.Abort()
			return
		}

		// 存储 key 配置到 context
		c.Set("api_key_config", keyConfig)
		c.Next()
	}
}

// RequestIDMiddleware 为每个请求注入 request_id 并通过响应头返回
// - 优先使用客户端的 X-Request-ID
// - 若无则生成一个UUID（utils.GenerateUUID）
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = "req_" + utils.GenerateUUID()
		}
		c.Set("request_id", rid)
		c.Writer.Header().Set("X-Request-ID", rid)
		c.Next()
	}
}

// GetRequestID 从上下文读取 request_id（若不存在返回空串）
func GetRequestID(c *gin.Context) string {
	if v, ok := c.Get("request_id"); ok {
		if s, ok2 := v.(string); ok2 {
			return s
		}
	}
	return ""
}

// GetMessageID 从上下文读取 message_id（若不存在返回空串）
func GetMessageID(c *gin.Context) string {
	if v, ok := c.Get("message_id"); ok {
		if s, ok2 := v.(string); ok2 {
			return s
		}
	}
	return ""
}

// addReqFields 注入标准请求字段，统一上下游日志可追踪（DRY）
func addReqFields(c *gin.Context, fields ...logger.Field) []logger.Field {
	rid := GetRequestID(c)
	mid := GetMessageID(c)
	// 预留容量避免重复分配
	out := make([]logger.Field, 0, len(fields)+2)
	if rid != "" {
		out = append(out, logger.String("request_id", rid))
	}
	if mid != "" {
		out = append(out, logger.String("message_id", mid))
	}
	out = append(out, fields...)
	return out
}

// requiresAuth 检查指定路径是否需要认证
func requiresAuth(path string, protectedPrefixes []string) bool {
	for _, prefix := range protectedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	// 兼容分组端点：/:group/v1/...
	// - /v1/...            -> parts[0] == "v1"
	// - /group/v1/...      -> parts[1] == "v1"
	trimmed := strings.TrimPrefix(path, "/")
	if trimmed == "" {
		return false
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) >= 1 && parts[0] == "v1" {
		return true
	}
	if len(parts) >= 2 && parts[1] == "v1" {
		return true
	}
	return false
}

// extractAPIKey 提取API密钥的通用逻辑
func extractAPIKey(c *gin.Context) string {
	apiKey := c.GetHeader("Authorization")
	if apiKey == "" {
		apiKey = c.GetHeader("x-api-key")
	} else {
		apiKey = strings.TrimPrefix(apiKey, "Bearer ")
	}
	return apiKey
}

// validateAPIKey 验证API密钥并返回配置
func validateAPIKey(c *gin.Context, keyMgr *auth.APIKeyManager) *auth.APIKeyConfig {
	providedApiKey := extractAPIKey(c)

	if providedApiKey == "" {
		logger.Warn("请求缺少Authorization或x-api-key头")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "401"})
		return nil
	}

	// 如果没有配置任何 API key，拒绝所有请求
	if keyMgr.IsEmpty() {
		logger.Error("未配置任何API Key")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "401"})
		return nil
	}

	keyConfig := keyMgr.Get(providedApiKey)
	if keyConfig == nil {
		logger.Error("API Key验证失败")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "401"})
		return nil
	}

	return keyConfig
}

// CheckGroupPermission 检查当前请求是否有访问指定 group 的权限
func CheckGroupPermission(c *gin.Context, group string) bool {
	v, exists := c.Get("api_key_config")
	if !exists {
		return false
	}
	keyConfig, ok := v.(*auth.APIKeyConfig)
	if !ok {
		return false
	}
	return keyConfig.HasGroupPermission(group)
}

// GetAPIKeyConfig 从 context 获取 API key 配置
func GetAPIKeyConfig(c *gin.Context) *auth.APIKeyConfig {
	v, exists := c.Get("api_key_config")
	if !exists {
		return nil
	}
	keyConfig, ok := v.(*auth.APIKeyConfig)
	if !ok {
		return nil
	}
	return keyConfig
}

// StatsMiddleware 统计中间件 - 记录请求信息
func StatsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		// 只统计 API 请求
		path := c.Request.URL.Path
		trimmed := strings.TrimPrefix(path, "/")
		if trimmed == "" {
			return
		}
		parts := strings.Split(trimmed, "/")

		var isV1 bool
		var normalizedPath string
		if len(parts) >= 1 && parts[0] == "v1" {
			isV1 = true
			normalizedPath = path
		} else if len(parts) >= 2 && parts[1] == "v1" {
			isV1 = true
			// /{group}/v1/xxx -> /v1/xxx
			rest := ""
			if len(parts) > 2 {
				rest = "/" + strings.Join(parts[2:], "/")
			}
			normalizedPath = "/v1" + rest
		}
		if !isV1 {
			return
		}

		latency := time.Since(start).Milliseconds()
		record := stats.RequestRecord{
			ID:         GetRequestID(c),
			Timestamp:  start,
			Method:     c.Request.Method,
			Path:       normalizedPath,
			StatusCode: c.Writer.Status(),
			Latency:    latency,
		}

		// 从上下文读取可选字段
		if v, ok := c.Get("stats_model"); ok {
			record.Model = v.(string)
		}
		if v, ok := c.Get("stats_request_type"); ok {
			record.RequestType = v.(string)
		}
		if v, ok := c.Get("stats_stream"); ok {
			record.Stream = v.(bool)
		}
		if v, ok := c.Get("stats_input_tokens"); ok {
			record.InputTokens = v.(int)
		}
		if v, ok := c.Get("stats_output_tokens"); ok {
			record.OutputTokens = v.(int)
		}
		if v, ok := c.Get("stats_token_index"); ok {
			record.TokenIndex = v.(int)
		}
		if v, ok := c.Get("stats_conversation_id"); ok {
			record.ConversationId = v.(string)
		}
		if v, ok := c.Get("stats_group"); ok {
			record.Group = v.(string)
		} else if len(parts) >= 2 && parts[1] == "v1" {
			// 为未显式设置 group 的请求兜底（例如 /:group/v1/models）
			record.Group = parts[0]
		}
		if record.Group == "" {
			record.Group = auth.GetDefaultGroup()
		}
		if v, ok := c.Get("stats_error"); ok {
			record.Error = v.(string)
		}
		if v, ok := c.Get("stats_cache_read"); ok {
			record.CacheReadInputTokens = v.(int)
		}
		if v, ok := c.Get("stats_cache_creation"); ok {
			record.CacheCreationTokens = v.(int)
		}
		if v, ok := c.Get("stats_credit_usage"); ok {
			record.CreditUsage = v.(float64)
		}
		if v, ok := c.Get("stats_context_usage"); ok {
			record.ContextUsagePercent = v.(float64)
		}
		if v, ok := c.Get("stats_ttfb"); ok {
			record.TTFB = v.(int64)
		}

		if collector := handler.GetStatsCollector(); collector != nil {
			collector.Record(record)
		}

		// 输出 credit-based token 计算日志
		if record.CreditUsage > 0 || record.ContextUsagePercent > 0 {
			// 根据 contextUsagePercent 计算实际 input tokens
			// 注意: contextUsagePercent 是百分比形式 (如 22.5 表示 22.5%)
			// 公式: contextUsagePercent / 100 × 200000 = actual input tokens
			actualInputTokens := int(record.ContextUsagePercent / 100 * 200000)

			// 根据 credit 反推 output tokens
			// 公式: credit = (input × 3 + output × 15) / 1,000,000
			// 所以: output = (credit × 1,000,000 - input × 3) / 15
			var calculatedOutputTokens int
			if record.CreditUsage > 0 && actualInputTokens > 0 {
				calculatedOutputTokens = int((record.CreditUsage*1000000 - float64(actualInputTokens)*3) / 15)
				if calculatedOutputTokens < 0 {
					calculatedOutputTokens = 0 // 可能是缓存命中，input 成本降低
				}
			}

			// 检测缓存命中：基于 Anthropic Prompt Caching 计价规则
			// Cache read: 0.3 / MTok (10% of regular)
			// Regular input: 3 / MTok
			// Output: 15 / MTok
			var cacheHit bool
			if record.CreditUsage > 0 && actualInputTokens > 0 && calculatedOutputTokens >= 0 {
				// 计算期望 credit（无缓存）
				expectedCredit := float64(actualInputTokens)*3/1000000 + float64(calculatedOutputTokens)*15/1000000

				// 如果实际 credit 显著低于期望值，说明有缓存命中
				// 缓存价格是 0.1x，所以阈值设为 0.6（考虑部分缓存的情况）
				if record.CreditUsage < expectedCredit*0.6 {
					cacheHit = true
				}
			}

			logger.Info("credit计量",
				logger.String("request_id", record.ID),
				logger.String("model", record.Model),
				logger.Float64("credit_usage", record.CreditUsage),
				logger.Float64("context_usage_percent", record.ContextUsagePercent),
				logger.Int("actual_input_tokens", actualInputTokens),
				logger.Int("calculated_output_tokens", calculatedOutputTokens),
				logger.Int("estimated_input_tokens", record.InputTokens),
				logger.Int("estimated_output_tokens", record.OutputTokens),
				logger.Bool("cache_hit", cacheHit),
				logger.Int64("latency_ms", record.Latency),
				logger.Int64("ttfb_ms", record.TTFB))
		}
	}
}
