package handler

import (
	"net/http"

	"kiro2api/internal/auth"
	"kiro2api/internal/config"
	"kiro2api/internal/service"

	"github.com/gin-gonic/gin"
)

// GetSettingsWithAuth GET /api/settings - 带 authService
func GetSettingsWithAuth(c *gin.Context, authService *auth.AuthService) {
	settings := GetSettingsManager().Get()

	// 限流器实时状态
	var rateLimiterStats map[string]interface{}
	if rl := GetRateLimiter(); rl != nil {
		rateLimiterStats = rl.Stats()
	}

	// 获取可用 token 数和 in-flight 统计
	var activeTokens int
	var globalInFlight int64
	var tokensWithInFlight int
	if authService != nil {
		repo := authService.GetRepository()
		activeTokens, _ = repo.CountActive()
		globalInFlight, tokensWithInFlight = authService.GetGlobalInFlightStats()
	}

	c.JSON(http.StatusOK, gin.H{
		"settings":              settings,
		"rate_limiter":          rateLimiterStats,
		"active_tokens":         activeTokens,
		"global_in_flight":      globalInFlight,
		"tokens_with_in_flight": tokensWithInFlight,
	})
}

// GetSettings GET /api/settings (兼容旧接口)
func GetSettings(c *gin.Context) {
	settings := GetSettingsManager().Get()

	var rateLimiterStats map[string]interface{}
	if rl := GetRateLimiter(); rl != nil {
		rateLimiterStats = rl.Stats()
	}

	c.JSON(http.StatusOK, gin.H{
		"settings":     settings,
		"rate_limiter": rateLimiterStats,
	})
}

// UpdateSettings POST /api/settings
func UpdateSettings(c *gin.Context) {
	var req config.Settings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求格式"})
		return
	}

	// 验证参数
	if req.RateLimitQPS <= 0 {
		req.RateLimitQPS = service.DefaultRateLimitQPS
	}
	if req.RateLimitBurst <= 0 {
		req.RateLimitBurst = service.DefaultRateLimitBurst
	}
	if req.RequestTimeoutSec <= 0 {
		req.RequestTimeoutSec = 120
	}
	if req.MaxRetries < 0 {
		req.MaxRetries = 2
	}
	if req.CooldownSec <= 0 {
		req.CooldownSec = 30
	}
	if req.TokenRateLimitQPS < 0 {
		req.TokenRateLimitQPS = 0
	}
	if req.TokenRateLimitBurst < 0 {
		req.TokenRateLimitBurst = 0
	}
	if req.TokenMaxConcurrent < 0 {
		req.TokenMaxConcurrent = 0
	}
	if req.GroupMaxConcurrent < 0 {
		req.GroupMaxConcurrent = 0
	}
	if req.RefreshConcurrency <= 0 {
		req.RefreshConcurrency = 5 // 默认并发数
	} else if req.RefreshConcurrency > 50 {
		req.RefreshConcurrency = 50 // 最大限制
	}

	// 更新设置
	if err := GetSettingsManager().Update(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存设置失败: " + err.Error()})
		return
	}

	// 动态更新限流器
	if rl := GetRateLimiter(); rl != nil {
		rl.SetRate(req.RateLimitQPS, req.RateLimitBurst)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "设置已更新",
		"settings": GetSettingsManager().Get(),
	})
}
