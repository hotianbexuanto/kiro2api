package service

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiter 全局请求限流器
type RateLimiter struct {
	limiter *rate.Limiter
}

// NewRateLimiter 创建限流器
// qps: 每秒允许的请求数
// burst: 突发容量（允许短时超过 qps 的请求数）
func NewRateLimiter(qps float64, burst int) *RateLimiter {
	return &RateLimiter{
		limiter: rate.NewLimiter(rate.Limit(qps), burst),
	}
}

// Middleware 返回 Gin 中间件
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 只对 AI API 请求限流（/v1/*），管理界面（/api/*）不限流
		if len(c.Request.URL.Path) > 4 && c.Request.URL.Path[:4] == "/api" {
			c.Next()
			return
		}

		// GET 请求不限流（只读操作）
		if c.Request.Method == http.MethodGet {
			c.Next()
			return
		}

		// 尝试获取令牌
		if !rl.limiter.Allow() {
			// 计算重试等待时间
			reservation := rl.limiter.Reserve()
			delay := reservation.Delay()
			reservation.Cancel() // 取消预约，不消耗令牌

			c.Header("Retry-After", strconv.Itoa(int(delay.Seconds())+1))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"type":    "error",
				"error":   gin.H{"type": "rate_limit_error", "message": "请求过于频繁，请稍后重试"},
				"message": "rate limit exceeded",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Wait 阻塞等待直到可以发送请求（用于内部调用）
func (rl *RateLimiter) Wait(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return rl.limiter.Wait(ctx)
}

// SetRate 动态调整限流速率
func (rl *RateLimiter) SetRate(qps float64, burst int) {
	rl.limiter.SetLimit(rate.Limit(qps))
	rl.limiter.SetBurst(burst)
}

// Stats 返回当前状态
func (rl *RateLimiter) Stats() map[string]interface{} {
	return map[string]interface{}{
		"qps":       float64(rl.limiter.Limit()),
		"burst":     rl.limiter.Burst(),
		"available": rl.limiter.Tokens(),
	}
}

// 默认配置
const (
	DefaultRateLimitQPS   = 50.0 // 每秒 50 个请求（提高全局限制，因为每个token已有限制）
	DefaultRateLimitBurst = 100   // 突发容量 100
)
