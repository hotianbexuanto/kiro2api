package handler

import (
	"fmt"
	"net/http"
	"strings"

	"kiro2api/internal/auth"
	"kiro2api/internal/logger"
	"kiro2api/internal/service"
	"kiro2api/internal/stats"
	"kiro2api/internal/types"
	"kiro2api/internal/utils"

	"github.com/gin-gonic/gin"
)

// HandleMessages POST /v1/messages - Anthropic API 代理
func HandleMessages(c *gin.Context, authService *auth.AuthService, group string) {
	service.SetAuthServiceInContext(c, authService)
	service.SetGroupInContext(c, group)

	// 使用统一管线创建请求上下文
	reqCtx := NewRequestContext(c, authService, "Anthropic", group)

	// 确保请求结束时记录 metrics
	success := false
	defer func() {
		reqCtx.Lifecycle.End(success)
	}()

	tokenWithUsage, body, err := reqCtx.GetTokenWithUsageAndBody()
	if err != nil {
		return
	}

	// 解析为通用 map 以便处理工具格式
	var rawReq map[string]any
	if err := utils.SafeUnmarshal(body, &rawReq); err != nil {
		logger.Error("解析请求体失败", logger.Err(err))
		service.RespondError(c, http.StatusBadRequest, "解析请求体失败: %v", err)
		return
	}

	// 标准化工具格式
	normalizeTools(rawReq)

	// 重新序列化并解析为 AnthropicRequest
	normalizedBody, err := utils.SafeMarshal(rawReq)
	if err != nil {
		logger.Error("重新序列化请求失败", logger.Err(err))
		service.RespondError(c, http.StatusBadRequest, "处理请求格式失败: %v", err)
		return
	}

	var anthropicReq types.AnthropicRequest
	if err := utils.SafeUnmarshal(normalizedBody, &anthropicReq); err != nil {
		logger.Error("解析标准化请求体失败", logger.Err(err))
		service.RespondError(c, http.StatusBadRequest, "解析请求体失败: %v", err)
		return
	}

	// 验证请求
	if err := validateAnthropicRequest(c, anthropicReq); err != nil {
		return
	}

	// 记录统计信息
	stats.SetRequestType(c, "anthropic")
	stats.SetModel(c, anthropicReq.Model)
	stats.SetGroup(c, group)
	stats.SetStream(c, anthropicReq.Stream)

	if anthropicReq.Stream {
		HandleAnthropicStream(c, anthropicReq, tokenWithUsage)
		success = true
		return
	}

	HandleAnthropicNonStream(c, anthropicReq, tokenWithUsage.TokenInfo)
	success = true
}

// normalizeTools 标准化工具格式
func normalizeTools(rawReq map[string]any) {
	tools, exists := rawReq["tools"]
	if !exists || tools == nil {
		return
	}

	toolsArray, ok := tools.([]any)
	if !ok {
		return
	}

	normalizedTools := make([]map[string]any, 0, len(toolsArray))
	for _, tool := range toolsArray {
		toolMap, ok := tool.(map[string]any)
		if !ok {
			continue
		}

		// 检查是否是简化格式（直接包含 name, description, input_schema）
		name, hasName := toolMap["name"]
		description, hasDesc := toolMap["description"]
		inputSchema, hasSchema := toolMap["input_schema"]

		if hasName && hasDesc && hasSchema {
			normalizedTools = append(normalizedTools, map[string]any{
				"name":         name,
				"description":  description,
				"input_schema": inputSchema,
			})
		} else {
			normalizedTools = append(normalizedTools, toolMap)
		}
	}
	rawReq["tools"] = normalizedTools
}

// validateAnthropicRequest 验证 Anthropic 请求
func validateAnthropicRequest(c *gin.Context, req types.AnthropicRequest) error {
	if len(req.Messages) == 0 {
		logger.Error("请求中没有消息")
		service.RespondError(c, http.StatusBadRequest, "%s", "messages 数组不能为空")
		return fmt.Errorf("messages empty")
	}

	lastMsg := req.Messages[len(req.Messages)-1]
	content, err := utils.GetMessageContent(lastMsg.Content)
	if err != nil {
		logger.Error("获取消息内容失败",
			logger.Err(err),
			logger.String("raw_content", fmt.Sprintf("%v", lastMsg.Content)))
		service.RespondError(c, http.StatusBadRequest, "获取消息内容失败: %v", err)
		return err
	}

	trimmedContent := strings.TrimSpace(content)
	if trimmedContent == "" || trimmedContent == "answer for user question" {
		logger.Error("消息内容为空或无效",
			logger.String("content", content),
			logger.String("trimmed_content", trimmedContent))
		service.RespondError(c, http.StatusBadRequest, "%s", "消息内容不能为空")
		return fmt.Errorf("content empty")
	}

	return nil
}
