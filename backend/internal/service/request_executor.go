package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"kiro2api/internal/config"
	"kiro2api/internal/converter"
	"kiro2api/internal/logger"
	"kiro2api/internal/stats"
	"kiro2api/internal/types"
	"kiro2api/internal/utils"

	"github.com/gin-gonic/gin"
)

// RespondError 简化封装
func RespondError(c *gin.Context, statusCode int, format string, args ...any) {
	code := types.ErrorCodeFromStatus(statusCode)
	RespondErrorWithCode(c, statusCode, code, format, args...)
}

// RespondErrorWithCode 标准化的错误响应结构
func RespondErrorWithCode(c *gin.Context, statusCode int, code string, format string, args ...any) {
	errType := types.ErrorTypeFromStatus(statusCode)
	c.JSON(statusCode, types.NewAPIError(errType, fmt.Sprintf(format, args...), code))
}

func HandleResponseReadError(c *gin.Context, err error) {
	logger.Error("读取响应体失败", AddReqFields(c, logger.Err(err))...)
	RespondError(c, http.StatusInternalServerError, "读取响应体失败: %v", err)
}

func handleRequestBuildError(c *gin.Context, err error) {
	logger.Error("构建请求失败", AddReqFields(c, logger.Err(err))...)
	RespondError(c, http.StatusInternalServerError, "构建请求失败: %v", err)
}

func handleRequestSendError(c *gin.Context, err error) {
	logger.Error("发送请求失败", AddReqFields(c, logger.Err(err))...)
	RespondError(c, http.StatusInternalServerError, "发送请求失败: %v", err)
}

func ExecuteCodeWhispererRequest(c *gin.Context, anthropicReq types.AnthropicRequest, tokenInfo types.TokenInfo, isStream bool) (*http.Response, error) {
	req, err := buildCodeWhispererRequest(c, anthropicReq, tokenInfo, isStream)
	if err != nil {
		if _, ok := err.(*types.ModelNotFoundErrorType); ok {
			return nil, err
		}
		handleRequestBuildError(c, err)
		return nil, err
	}

	settings := config.GetDefaultSettingsManager().Get()
	ctx, cancel := context.WithTimeout(req.Context(), time.Duration(settings.RequestTimeoutSec)*time.Second)
	req = req.WithContext(ctx)

	resp, err := utils.DoRequest(req)
	if err != nil {
		cancel()
		handleRequestSendError(c, err)
		return nil, err
	}

	resp.Body = &closeFuncReadCloser{
		ReadCloser: resp.Body,
		onClose:    cancel,
	}

	if handleCodeWhispererError(c, resp) {
		resp.Body.Close()
		return nil, fmt.Errorf("CodeWhisperer API error")
	}

	logger.Debug("上游响应成功",
		AddReqFields(c,
			logger.String("direction", "upstream_response"),
			logger.Int("status_code", resp.StatusCode),
		)...)

	return resp, nil
}

// ExecuteCWRequest 供测试覆盖的请求执行入口
var ExecuteCWRequest = executeWithRetry

// AuthServiceForRetry 重试所需的认证服务接口
type AuthServiceForRetry interface {
	GetToken(group string) (types.TokenInfo, error)
	MarkTokenFailed(token types.TokenInfo)
	RecordRequest(token types.TokenInfo, latency time.Duration, success bool)
}

const contextKeyAuthService = "auth_service_for_retry"
const contextKeyGroup = "token_group"

func SetAuthServiceInContext(c *gin.Context, authService AuthServiceForRetry) {
	c.Set(contextKeyAuthService, authService)
}

func SetGroupInContext(c *gin.Context, group string) {
	c.Set(contextKeyGroup, group)
}

func GetGroupFromContext(c *gin.Context) string {
	if group, exists := c.Get(contextKeyGroup); exists {
		if g, ok := group.(string); ok {
			return g
		}
	}
	return ""
}

// GetAuthServiceFromContext 从上下文获取 AuthService
func GetAuthServiceFromContext(c *gin.Context) AuthServiceForRetry {
	if authServiceVal, exists := c.Get(contextKeyAuthService); exists {
		if authService, ok := authServiceVal.(AuthServiceForRetry); ok {
			return authService
		}
	}
	return nil
}

func executeWithRetry(c *gin.Context, anthropicReq types.AnthropicRequest, tokenInfo types.TokenInfo, isStream bool) (*http.Response, error) {
	authServiceVal, exists := c.Get(contextKeyAuthService)
	if !exists {
		return ExecuteCodeWhispererRequest(c, anthropicReq, tokenInfo, isStream)
	}
	authService, ok := authServiceVal.(AuthServiceForRetry)
	if !ok {
		return ExecuteCodeWhispererRequest(c, anthropicReq, tokenInfo, isStream)
	}

	group := GetGroupFromContext(c)
	currentToken := tokenInfo
	var lastErr error

	settings := config.GetDefaultSettingsManager().Get()
	maxRetries := settings.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			logger.Info("重试请求",
				AddReqFields(c,
					logger.Int("attempt", attempt),
					logger.Int("max_retries", maxRetries),
				)...)
		}

		req, err := buildCodeWhispererRequest(c, anthropicReq, currentToken, isStream)
		if err != nil {
			if _, ok := err.(*types.ModelNotFoundErrorType); ok {
				return nil, err
			}
			handleRequestBuildError(c, err)
			return nil, err
		}

		settings = config.GetDefaultSettingsManager().Get()

		// 并发控制（可选）
		releaseGroup, ok := acquireSemaphore(req.Context(), groupSems, group, settings.GroupMaxConcurrent, &groupSemMu)
		if !ok {
			handleRequestSendError(c, context.Canceled)
			return nil, context.Canceled
		}
		releaseToken, ok := acquireSemaphore(req.Context(), tokenSems, currentToken.RefreshToken, settings.TokenMaxConcurrent, &tokenSemMu)
		if !ok {
			releaseGroup()
			handleRequestSendError(c, context.Canceled)
			return nil, context.Canceled
		}

		// 单 token 限流（可选）
		if rl := getTokenLimiter(currentToken.RefreshToken, settings.TokenRateLimitQPS, settings.TokenRateLimitBurst); rl != nil {
			if err := rl.Wait(req.Context()); err != nil {
				releaseToken()
				releaseGroup()
				handleRequestSendError(c, err)
				return nil, err
			}
		}

		ctx, cancel := context.WithTimeout(req.Context(), time.Duration(settings.RequestTimeoutSec)*time.Second)
		req = req.WithContext(ctx)

		resp, err := utils.DoRequest(req)
		if err != nil {
			cancel()
			releaseToken()
			releaseGroup()
			lastErr = err
			authService.MarkTokenFailed(currentToken)
			newToken, tokenErr := authService.GetToken(group)
			if tokenErr != nil {
				handleRequestSendError(c, err)
				return nil, err
			}
			currentToken = newToken
			continue
		}

		resp.Body = &closeFuncReadCloser{
			ReadCloser: resp.Body,
			onClose: func() {
				cancel()
				releaseToken()
				releaseGroup()
			},
		}

		if config.IsRetryableStatus(resp.StatusCode) {
			resp.Body.Close()
			lastErr = fmt.Errorf("retryable status: %d", resp.StatusCode)

			logger.Warn("收到可重试状态码，切换 token",
				AddReqFields(c,
					logger.Int("status_code", resp.StatusCode),
					logger.Int("attempt", attempt),
				)...)

			authService.MarkTokenFailed(currentToken)
			newToken, tokenErr := authService.GetToken(group)
			if tokenErr != nil {
				RespondError(c, resp.StatusCode, "所有 token 不可用")
				return nil, lastErr
			}
			currentToken = newToken
			continue
		}

		if handleCodeWhispererError(c, resp) {
			resp.Body.Close()
			return nil, fmt.Errorf("CodeWhisperer API error")
		}

		logger.Debug("上游响应成功",
			AddReqFields(c,
				logger.String("direction", "upstream_response"),
				logger.Int("status_code", resp.StatusCode),
				logger.Int("attempts_used", attempt+1),
			)...)

		return resp, nil
	}

	logger.Error("所有重试都失败",
		AddReqFields(c,
			logger.Int("total_attempts", maxRetries+1),
			logger.Err(lastErr),
		)...)

	if lastErr != nil {
		handleRequestSendError(c, lastErr)
	}
	return nil, lastErr
}

func buildCodeWhispererRequest(c *gin.Context, anthropicReq types.AnthropicRequest, tokenInfo types.TokenInfo, isStream bool) (*http.Request, error) {
	cwReq, err := converter.BuildCodeWhispererRequest(anthropicReq, c)
	if err != nil {
		if modelNotFoundErr, ok := err.(*types.ModelNotFoundErrorType); ok {
			c.JSON(http.StatusBadRequest, modelNotFoundErr.ErrorData)
			return nil, err
		}
		return nil, fmt.Errorf("构建CodeWhisperer请求失败: %v", err)
	}

	// 记录会话ID到stats
	stats.SetConversationId(c, cwReq.ConversationState.ConversationId)

	cwReqBody, err := utils.SafeMarshal(cwReq)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	var toolNamesPreview string
	if len(cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.Tools) > 0 {
		names := make([]string, 0, len(cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.Tools))
		for _, t := range cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.Tools {
			if t.ToolSpecification.Name != "" {
				names = append(names, t.ToolSpecification.Name)
			}
		}
		toolNamesPreview = strings.Join(names, ",")
	}

	logger.Debug("发送给CodeWhisperer的请求",
		logger.String("direction", "upstream_request"),
		logger.Int("request_size", len(cwReqBody)),
		logger.String("request_body", string(cwReqBody)),
		logger.Int("tools_count", len(cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.Tools)),
		logger.String("tools_names", toolNamesPreview))

	req, err := http.NewRequest("POST", config.CodeWhispererURL, bytes.NewReader(cwReqBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+tokenInfo.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	if isStream {
		req.Header.Set("Accept", "text/event-stream")
	}

	tokenID := tokenInfo.RefreshToken
	if len(tokenID) > 16 {
		tokenID = tokenID[:16]
	}
	fingerprint := config.GetFingerprintManager().GenerateFingerprint(tokenID)
	kiroID := fmt.Sprintf("KiroIDE-%s-%s", config.KiroIDEVersion, fingerprint)
	req.Header.Set("User-Agent", fmt.Sprintf("aws-sdk-js/%s ua/2.1 os/%s lang/js md/nodejs#%s api/codewhispererstreaming#%s m/E %s",
		config.KiroSDKVersion, config.KiroOS, config.KiroNodeVersion, config.KiroSDKVersion, kiroID))
	req.Header.Set("x-amz-user-agent", fmt.Sprintf("aws-sdk-js/%s %s", config.KiroSDKVersion, kiroID))
	req.Header.Set("x-amzn-kiro-agent-mode", "vibe")
	req.Header.Set("x-amzn-codewhisperer-optout", "true")
	req.Header.Set("amz-sdk-invocation-id", utils.GenerateUUID())
	req.Header.Set("amz-sdk-request", "attempt=1; max=3")

	return req, nil
}

func handleCodeWhispererError(c *gin.Context, resp *http.Response) bool {
	if resp.StatusCode == http.StatusOK {
		return false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("读取错误响应失败",
			AddReqFields(c,
				logger.String("direction", "upstream_response"),
				logger.Err(err),
			)...)
		RespondError(c, http.StatusInternalServerError, "%s", "读取响应失败")
		return true
	}

	logger.Error("上游响应错误",
		AddReqFields(c,
			logger.String("direction", "upstream_response"),
			logger.Int("status_code", resp.StatusCode),
			logger.Int("response_len", len(body)),
			logger.String("response_body", string(body)),
		)...)

	if resp.StatusCode == http.StatusForbidden {
		logger.Warn("收到403错误，token可能已失效")
		RespondErrorWithCode(c, http.StatusUnauthorized, "unauthorized", "%s", "Token已失效，请重试")
		return true
	}

	errorMapper := NewErrorMapper()
	claudeError := errorMapper.MapCodeWhispererError(resp.StatusCode, body)

	if claudeError.StopReason == "max_tokens" {
		logger.Info("内容长度超限，映射为max_tokens stop_reason",
			AddReqFields(c,
				logger.String("upstream_reason", "CONTENT_LENGTH_EXCEEDS_THRESHOLD"),
				logger.String("claude_stop_reason", "max_tokens"),
			)...)
		errorMapper.SendClaudeError(c, claudeError)
	} else {
		RespondErrorWithCode(c, http.StatusInternalServerError, "cw_error", "CodeWhisperer Error: %s", string(body))
	}

	return true
}

func FilterSupportedTools(tools []types.AnthropicTool) []types.AnthropicTool {
	if len(tools) == 0 {
		return tools
	}

	filtered := make([]types.AnthropicTool, 0, len(tools))
	for _, tool := range tools {
		if config.IsUnsupportedTool(tool.Name) {
			logger.Debug("过滤不支持的工具（token计算）",
				logger.String("tool_name", tool.Name))
			continue
		}
		filtered = append(filtered, tool)
	}

	return filtered
}
