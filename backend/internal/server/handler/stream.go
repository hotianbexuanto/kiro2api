package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"kiro2api/internal/config"
	"kiro2api/internal/logger"
	"kiro2api/internal/parser"
	"kiro2api/internal/service"
	"kiro2api/internal/stats"
	"kiro2api/internal/types"
	"kiro2api/internal/utils"

	"github.com/gin-gonic/gin"
)

// HandleAnthropicStream 处理Anthropic流式请求
func HandleAnthropicStream(c *gin.Context, anthropicReq types.AnthropicRequest, tokenWithUsage *types.TokenWithUsage) {
	sender := &service.AnthropicStreamSender{}
	HandleGenericStreamRequest(c, anthropicReq, tokenWithUsage, sender, service.CreateAnthropicStreamEvents)
}

// HandleGenericStreamRequest 通用流式请求处理
func HandleGenericStreamRequest(c *gin.Context, anthropicReq types.AnthropicRequest, token *types.TokenWithUsage, sender service.StreamEventSender, eventCreator func(string, int, string) []map[string]any) {
	// 计算输入tokens（基于实际发送给上游的数据）
	estimator := utils.NewTokenEstimator()
	countReq := &types.CountTokensRequest{
		Model:    anthropicReq.Model,
		System:   anthropicReq.System,
		Messages: anthropicReq.Messages,
		Tools:    service.FilterSupportedTools(anthropicReq.Tools), // 过滤不支持的工具后计算
	}
	inputTokens := estimator.EstimateTokens(countReq)

	logger.Debug("计算输入tokens",
		logger.String("model", anthropicReq.Model),
		logger.Int("input_tokens", inputTokens))

	// 初始化SSE响应
	if err := service.InitializeSSEResponse(c); err != nil {
		sender.SendError(c, "连接不支持SSE刷新", err)
		return
	}

	// 生成消息ID并注入上下文
	messageID := fmt.Sprintf(config.MessageIDFormat, time.Now().Format(config.MessageIDTimeFormat))
	c.Set("message_id", messageID)

	// 执行CodeWhisperer请求
	// 需要 server 包暴露 ExecuteCWRequest
	resp, err := service.ExecuteCWRequest(c, anthropicReq, token.TokenInfo, true)
	if err != nil {
		var modelNotFoundErrorType *types.ModelNotFoundErrorType
		if errors.As(err, &modelNotFoundErrorType) {
			return
		}
		sender.SendError(c, "构建请求失败", err)
		return
	}
	defer resp.Body.Close()

	// 创建流处理上下文
	ctx := service.NewStreamProcessorContext(c, anthropicReq, token, sender, messageID, inputTokens)
	defer ctx.Cleanup()

	// 发送初始事件
	if err := ctx.SendInitialEvents(eventCreator); err != nil {
		return
	}

	// 处理事件流
	processor := service.NewEventStreamProcessor(ctx)
	if err := processor.ProcessEventStream(resp.Body); err != nil {
		logger.Error("事件流处理失败", logger.Err(err))
		return
	}

	// 发送结束事件
	if err := ctx.SendFinalEvents(); err != nil {
		logger.Error("发送结束事件失败", logger.Err(err))
		return
	}

	// 记录统计信息
	stats.SetTokens(c, ctx.InputTokens, ctx.TotalOutputTokens) // 这里的InputTokens和TotalOutputTokens是StreamProcessorContext的公开字段吗？ Wait, I need to check if they are exported.
	// In stream_processor.go, struct fields like inputTokens are lower case. I need to Export them.
	if ctx.CreditUsage > 0 { // CreditUsage also needs to be exported
		stats.SetCreditUsage(c, ctx.CreditUsage)
	}
	if ctx.ContextUsagePercent > 0 { // This too
		stats.SetContextUsage(c, ctx.ContextUsagePercent)
	}
	if ctx.TTFB > 0 {
		stats.SetTTFB(c, ctx.TTFB)
	}
}

// HandleAnthropicNonStream 处理非流式请求
func HandleAnthropicNonStream(c *gin.Context, anthropicReq types.AnthropicRequest, token types.TokenInfo) {
	// 计算输入tokens
	estimator := utils.NewTokenEstimator()
	countReq := &types.CountTokensRequest{
		Model:    anthropicReq.Model,
		System:   anthropicReq.System,
		Messages: anthropicReq.Messages,
		Tools:    service.FilterSupportedTools(anthropicReq.Tools),
	}
	inputTokens := estimator.EstimateTokens(countReq)

	resp, err := service.ExecuteCWRequest(c, anthropicReq, token, false)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := utils.ReadHTTPResponse(resp.Body)
	if err != nil {
		service.HandleResponseReadError(c, err)
		return
	}

	// 解析响应
	compliantParser := parser.NewCompliantEventStreamParser()
	compliantParser.SetMaxErrors(config.ParserMaxErrors)

	// 需要 server 暴露 ParseWithTimeout
	result, err := service.ParseWithTimeout(compliantParser, body, 10*time.Second)

	if err != nil {
		logger.Error("非流式解析失败",
			logger.Err(err),
			logger.String("model", anthropicReq.Model),
			logger.Int("response_size", len(body)))

		errorResp := gin.H{
			"error":   "响应解析失败",
			"type":    "parsing_error",
			"message": "无法解析AWS CodeWhisperer响应格式",
		}

		statusCode := http.StatusInternalServerError
		if strings.Contains(err.Error(), "解析超时") {
			statusCode = http.StatusRequestTimeout
			errorResp["message"] = "请求处理超时，请稍后重试"
		} else if strings.Contains(err.Error(), "格式错误") {
			statusCode = http.StatusBadRequest
			errorResp["message"] = "请求格式不正确"
		}

		c.JSON(statusCode, errorResp)
		return
	}

	// 转换为Anthropic格式
	var contexts []map[string]any
	textAgg := result.GetCompletionText()

	toolManager := compliantParser.GetToolManager()
	allTools := make([]*parser.ToolExecution, 0)

	// 获取活跃工具
	for _, tool := range toolManager.GetActiveTools() {
		allTools = append(allTools, tool)
	}

	// 获取已完成工具
	for _, tool := range toolManager.GetCompletedTools() {
		allTools = append(allTools, tool)
	}

	sawToolUse := len(allTools) > 0

	// 添加文本内容
	if textAgg != "" {
		contexts = append(contexts, map[string]any{
			"type": "text",
			"text": textAgg,
		})
	}

	// 添加工具调用
	for _, tool := range allTools {
		toolUseBlock := map[string]any{
			"type":  "tool_use",
			"id":    tool.ID,
			"name":  tool.Name,
			"input": tool.Arguments,
		}

		if tool.Arguments == nil {
			toolUseBlock["input"] = map[string]any{}
		}

		contexts = append(contexts, toolUseBlock)
	}

	// 使用新的stop_reason管理器
	stopReasonManager := service.NewStopReasonManager(anthropicReq)

	outputTokens := 0
	for _, contentBlock := range contexts {
		blockType, _ := contentBlock["type"].(string)

		switch blockType {
		case "text":
			if text, ok := contentBlock["text"].(string); ok {
				outputTokens += estimator.EstimateTextTokens(text)
			}

		case "tool_use":
			toolName, _ := contentBlock["name"].(string)
			toolInput, _ := contentBlock["input"].(map[string]any)
			outputTokens += estimator.EstimateToolUseTokens(toolName, toolInput)
		}
	}

	if outputTokens < 1 && len(contexts) > 0 {
		outputTokens = 1
	}

	stopReasonManager.UpdateToolCallStatus(sawToolUse, sawToolUse)
	stopReason := stopReasonManager.DetermineStopReason()

	anthropicResp := map[string]any{
		"content":       contexts,
		"model":         anthropicReq.Model,
		"role":          "assistant",
		"stop_reason":   stopReason,
		"stop_sequence": nil,
		"type":          "message",
		"usage": map[string]any{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
		},
	}

	logger.Debug("下发非流式响应",
		service.AddReqFields(c,
			logger.String("direction", "downstream_send"),
			logger.Any("contexts", contexts),
			logger.Bool("saw_tool_use", sawToolUse),
			logger.Int("content_count", len(contexts)),
		)...)

	// 记录统计信息
	stats.SetTokens(c, inputTokens, outputTokens)

	// 从解析结果中提取 credit 和 context 信息
	for _, event := range result.Events {
		switch event.Event {
		case "metering":
			if data, ok := event.Data.(map[string]any); ok {
				if credit, ok := data["credit_usage"].(float64); ok {
					stats.SetCreditUsage(c, credit)
				}
			}
		case "context_usage":
			if data, ok := event.Data.(map[string]any); ok {
				if percent, ok := data["context_usage_percent"].(float64); ok {
					stats.SetContextUsage(c, percent)
				}
			}
		}
	}

	c.JSON(http.StatusOK, anthropicResp)
}
