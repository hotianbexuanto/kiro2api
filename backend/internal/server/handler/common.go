package handler

import (
	"fmt"
	"net/http"

	"kiro2api/internal/auth"
	"kiro2api/internal/logger"
	"kiro2api/internal/service"
	"kiro2api/internal/types"

	"github.com/gin-gonic/gin"
)

// RequestContext 请求处理上下文，封装通用的请求处理逻辑
type RequestContext struct {
	GinContext  *gin.Context
	AuthService *auth.AuthService
	RequestType string // "anthropic" 或 "openai"
	Group       string // token 分组
	Lifecycle   *service.TokenRequestLifecycle // 统一生命周期管理
}

// NewRequestContext 创建请求上下文
func NewRequestContext(c *gin.Context, authService *auth.AuthService, requestType, group string) *RequestContext {
	lifecycle := service.NewTokenRequestLifecycle(c, authService, group)
	return &RequestContext{
		GinContext:  c,
		AuthService: authService,
		RequestType: requestType,
		Group:       group,
		Lifecycle:   lifecycle,
	}
}

// GetTokenAndBody 通用的token获取和请求体读取
func (rc *RequestContext) GetTokenAndBody() (types.TokenInfo, []byte, error) {
	// 通过统一管线获取 token
	tokenInfo, err := rc.Lifecycle.GetToken()
	if err != nil {
		logger.Error("获取token失败", logger.Err(err))
		service.RespondError(rc.GinContext, http.StatusInternalServerError, "获取token失败: %v", err)
		return types.TokenInfo{}, nil, err
	}

	// 读取请求体
	body, err := rc.GinContext.GetRawData()
	if err != nil {
		logger.Error("读取请求体失败", logger.Err(err))
		service.RespondError(rc.GinContext, http.StatusBadRequest, "读取请求体失败: %v", err)
		return types.TokenInfo{}, nil, err
	}

	// 记录请求日志
	logger.Debug(fmt.Sprintf("收到%s请求", rc.RequestType),
		logger.String("direction", "client_request"),
		logger.String("body", string(body)),
		logger.Int("body_size", len(body)),
		logger.String("remote_addr", rc.GinContext.ClientIP()),
		logger.String("user_agent", rc.GinContext.GetHeader("User-Agent")),
		logger.String("group", rc.Group))

	return tokenInfo, body, nil
}

// GetTokenWithUsageAndBody 获取token（包含使用信息）和请求体
func (rc *RequestContext) GetTokenWithUsageAndBody() (*types.TokenWithUsage, []byte, error) {
	// 通过统一管线获取 token
	tokenWithUsage, err := rc.Lifecycle.GetTokenWithUsage()
	if err != nil {
		logger.Error("获取token失败", logger.Err(err))
		service.RespondError(rc.GinContext, http.StatusInternalServerError, "获取token失败: %v", err)
		return nil, nil, err
	}

	// 读取请求体
	body, err := rc.GinContext.GetRawData()
	if err != nil {
		logger.Error("读取请求体失败", logger.Err(err))
		service.RespondError(rc.GinContext, http.StatusBadRequest, "读取请求体失败: %v", err)
		return nil, nil, err
	}

	// 记录请求日志
	logger.Debug(fmt.Sprintf("收到%s请求", rc.RequestType),
		logger.String("direction", "client_request"),
		logger.String("body", string(body)),
		logger.Int("body_size", len(body)),
		logger.String("remote_addr", rc.GinContext.ClientIP()),
		logger.String("user_agent", rc.GinContext.GetHeader("User-Agent")),
		logger.Float64("available_count", tokenWithUsage.AvailableCount),
		logger.String("group", rc.Group))

	return tokenWithUsage, body, nil
}
