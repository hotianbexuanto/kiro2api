package service

import (
	"time"

	"kiro2api/internal/auth"
	"kiro2api/internal/logger"
	"kiro2api/internal/stats"
	"kiro2api/internal/types"

	"github.com/gin-gonic/gin"
)

// TokenRequestLifecycle 统一管理 token 请求生命周期
// 包括：获取 token、开始请求、结束请求、记录 metrics、标记失败
type TokenRequestLifecycle struct {
	c           *gin.Context
	authService *auth.AuthService
	token       types.TokenInfo
	tokenUsage  *types.TokenWithUsage
	group       string
	startTime   time.Time
	started     bool
	ended       bool
}

// NewTokenRequestLifecycle 创建 token 请求生命周期管理器
func NewTokenRequestLifecycle(c *gin.Context, authService *auth.AuthService, group string) *TokenRequestLifecycle {
	if group == "" {
		group = auth.GetDefaultGroup()
	}
	return &TokenRequestLifecycle{
		c:           c,
		authService: authService,
		group:       group,
	}
}

// GetToken 获取 token 并开始请求追踪
func (trl *TokenRequestLifecycle) GetToken() (types.TokenInfo, error) {
	tokenInfo, err := trl.authService.GetToken(trl.group)
	if err != nil {
		return types.TokenInfo{}, err
	}

	trl.token = tokenInfo
	trl.start()
	return tokenInfo, nil
}

// GetTokenWithUsage 获取 token（包含使用信息）并开始请求追踪
func (trl *TokenRequestLifecycle) GetTokenWithUsage() (*types.TokenWithUsage, error) {
	// 提取用户标识（IP + API Key）用于 token 粘性
	userID := trl.c.ClientIP()
	if apiKey, exists := trl.c.Get("api_key"); exists {
		if key, ok := apiKey.(string); ok {
			userID += "|" + key
		}
	}

	tokenWithUsage, err := trl.authService.GetTokenWithUsage(trl.group, userID)
	if err != nil {
		return nil, err
	}

	trl.token = tokenWithUsage.TokenInfo
	trl.tokenUsage = tokenWithUsage
	trl.start()
	return tokenWithUsage, nil
}

// start 内部方法：开始请求追踪
func (trl *TokenRequestLifecycle) start() {
	if trl.started {
		return
	}
	trl.started = true
	trl.startTime = time.Now()

	// 设置统计信息
	stats.SetTokenIndex(trl.c, int(trl.token.ID))
	stats.SetGroup(trl.c, trl.group)

	// 增加 in-flight 计数
	trl.authService.StartRequest(trl.token)

	logger.Debug("Token请求开始",
		logger.Int64("token_id", trl.token.ID),
		logger.String("group", trl.group))
}

// End 结束请求，记录 metrics
// success: 请求是否成功（用于统计成功率）
func (trl *TokenRequestLifecycle) End(success bool) {
	if !trl.started || trl.ended {
		return
	}
	trl.ended = true

	latency := time.Since(trl.startTime)

	// 减少 in-flight 计数
	trl.authService.EndRequest(trl.token)

	// 记录请求 metrics
	trl.authService.RecordRequest(trl.token, latency, success)

	logger.Debug("Token请求结束",
		logger.Int64("token_id", trl.token.ID),
		logger.Bool("success", success),
		logger.Duration("latency", latency))
}

// MarkFailed 标记 token 失败（触发冷却）
// 用于 429/5xx 等需要切换 token 的场景
func (trl *TokenRequestLifecycle) MarkFailed() {
	trl.authService.MarkTokenFailed(trl.token)
	logger.Warn("Token标记失败",
		logger.Int64("token_id", trl.token.ID),
		logger.String("group", trl.group))
}

// Token 获取当前 token
func (trl *TokenRequestLifecycle) Token() types.TokenInfo {
	return trl.token
}

// TokenWithUsage 获取当前 token（包含使用信息）
func (trl *TokenRequestLifecycle) TokenWithUsage() *types.TokenWithUsage {
	return trl.tokenUsage
}

// Group 获取当前分组
func (trl *TokenRequestLifecycle) Group() string {
	return trl.group
}

// Latency 获取当前请求延迟
func (trl *TokenRequestLifecycle) Latency() time.Duration {
	if !trl.started {
		return 0
	}
	return time.Since(trl.startTime)
}
