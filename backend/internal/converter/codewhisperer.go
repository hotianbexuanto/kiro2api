package converter

import (
	"fmt"
	"strings"

	"kiro2api/internal/config"
	"kiro2api/internal/logger"
	"kiro2api/internal/types"
	"kiro2api/internal/utils"

	"github.com/gin-gonic/gin"
)

// ValidateAssistantResponseEvent 验证助手响应事件
// ConvertToAssistantResponseEvent 转换任意数据为标准的AssistantResponseEvent
// NormalizeAssistantResponseEvent 标准化助手响应事件（填充默认值等）
// normalizeWebLinks 标准化网页链接
// normalizeReferences 标准化引用
// CodeWhisperer格式转换器

// determineChatTriggerType 智能确定聊天触发类型 (SOLID-SRP: 单一责任)
func determineChatTriggerType(anthropicReq types.AnthropicRequest) string {
	// 如果有工具调用，通常是自动触发的
	if len(anthropicReq.Tools) > 0 {
		// 检查tool_choice是否强制要求使用工具
		if anthropicReq.ToolChoice != nil {
			if tc, ok := anthropicReq.ToolChoice.(*types.ToolChoice); ok && tc != nil {
				if tc.Type == "any" || tc.Type == "tool" {
					return "AUTO" // 自动工具调用
				}
			} else if tcMap, ok := anthropicReq.ToolChoice.(map[string]any); ok {
				if tcType, exists := tcMap["type"].(string); exists {
					if tcType == "any" || tcType == "tool" {
						return "AUTO" // 自动工具调用
					}
				}
			}
		}
	}

	// 默认为手动触发
	return "MANUAL"
}

// validateCodeWhispererRequest 验证CodeWhisperer请求的完整性 (SOLID-SRP: 单一责任验证)
func validateCodeWhispererRequest(cwReq *types.CodeWhispererRequest) error {
	// 验证必需字段
	if cwReq.ConversationState.CurrentMessage.UserInputMessage.ModelId == "" {
		return fmt.Errorf("ModelId不能为空")
	}

	if cwReq.ConversationState.ConversationId == "" {
		return fmt.Errorf("ConversationId不能为空")
	}

	// 验证内容完整性 (KISS: 简化内容验证)
	trimmedContent := strings.TrimSpace(cwReq.ConversationState.CurrentMessage.UserInputMessage.Content)
	hasImages := len(cwReq.ConversationState.CurrentMessage.UserInputMessage.Images) > 0
	hasTools := len(cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.Tools) > 0
	hasToolResults := len(cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.ToolResults) > 0

	// 如果有工具结果，允许内容为空（这是工具执行后的反馈请求）
	if hasToolResults {
		logger.Debug("检测到工具结果，允许内容为空",
			logger.String("conversation_id", cwReq.ConversationState.ConversationId),
			logger.Int("tool_results_count", len(cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.ToolResults)))
		return nil
	}

	// 如果没有内容但有工具，注入占位内容 (YAGNI: 只在需要时处理)
	if trimmedContent == "" && !hasImages && hasTools {
		placeholder := "执行工具任务"
		cwReq.ConversationState.CurrentMessage.UserInputMessage.Content = placeholder
		logger.Warn("注入占位内容以触发工具调用",
			logger.String("conversation_id", cwReq.ConversationState.ConversationId),
			logger.Int("tools_count", len(cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.Tools)))
		trimmedContent = placeholder
	}

	// 验证至少有内容或图片
	if trimmedContent == "" && !hasImages {
		return fmt.Errorf("用户消息内容和图片都为空")
	}

	return nil
}

// mergeUserMessages 合并多个用户消息为一个 HistoryUserMessage
func mergeUserMessages(messages []types.AnthropicRequestMessage, modelId string) types.HistoryUserMessage {
	merged := types.HistoryUserMessage{}
	var contentParts []string
	var allImages []types.CodeWhispererImage
	var allToolResults []types.ToolResult

	for _, msg := range messages {
		content, images, err := processMessageContent(msg.Content)
		if err == nil && content != "" {
			contentParts = append(contentParts, content)
			allImages = append(allImages, images...)
		}
		toolResults := extractToolResultsFromMessage(msg.Content)
		allToolResults = append(allToolResults, toolResults...)
	}

	merged.UserInputMessage.Content = strings.Join(contentParts, "\n")
	if len(allImages) > 0 {
		merged.UserInputMessage.Images = allImages
	}
	if len(allToolResults) > 0 {
		merged.UserInputMessage.UserInputMessageContext.ToolResults = allToolResults
		merged.UserInputMessage.Content = "" // 工具结果请求 content 为空
	}
	merged.UserInputMessage.ModelId = modelId
	merged.UserInputMessage.Origin = "AI_EDITOR"

	return merged
}

// extractToolResultsFromMessage 从消息内容中提取工具结果
func extractToolResultsFromMessage(content any) []types.ToolResult {
	var toolResults []types.ToolResult

	switch v := content.(type) {
	case []any:
		for _, item := range v {
			if block, ok := item.(map[string]any); ok {
				if blockType, exists := block["type"]; exists {
					if typeStr, ok := blockType.(string); ok && typeStr == "tool_result" {
						toolResult := types.ToolResult{}

						// 提取 tool_use_id
						if toolUseId, ok := block["tool_use_id"].(string); ok {
							toolResult.ToolUseId = toolUseId
						}

						// 提取 content - 转换为数组格式
						if content, exists := block["content"]; exists {
							// 将 content 转换为 []map[string]any 格式
							var contentArray []map[string]any

							// 处理不同的 content 格式
							switch c := content.(type) {
							case string:
								// 如果是字符串，包装成标准格式
								contentArray = []map[string]any{
									{"text": c},
								}
							case []any:
								// 如果已经是数组，保持原样
								for _, item := range c {
									if m, ok := item.(map[string]any); ok {
										contentArray = append(contentArray, m)
									}
								}
							case map[string]any:
								// 如果是单个对象，包装成数组
								contentArray = []map[string]any{c}
							default:
								// 其他格式，尝试转换为字符串
								contentArray = []map[string]any{
									{"text": fmt.Sprintf("%v", c)},
								}
							}

							toolResult.Content = contentArray
						}

						// 提取 status (默认为 success)
						toolResult.Status = "success"
						if isError, ok := block["is_error"].(bool); ok && isError {
							toolResult.Status = "error"
							toolResult.IsError = true
						}

						toolResults = append(toolResults, toolResult)

						// 	logger.String("tool_use_id", toolResult.ToolUseId),
						// 	logger.String("status", toolResult.Status),
						// 	logger.Int("content_items", len(toolResult.Content)))
					}
				}
			}
		}
	case []types.ContentBlock:
		for _, block := range v {
			if block.Type == "tool_result" {
				toolResult := types.ToolResult{}

				if block.ToolUseId != nil {
					toolResult.ToolUseId = *block.ToolUseId
				}

				// 处理 content
				if block.Content != nil {
					var contentArray []map[string]any

					switch c := block.Content.(type) {
					case string:
						contentArray = []map[string]any{
							{"text": c},
						}
					case []any:
						for _, item := range c {
							if m, ok := item.(map[string]any); ok {
								contentArray = append(contentArray, m)
							}
						}
					case map[string]any:
						contentArray = []map[string]any{c}
					default:
						contentArray = []map[string]any{
							{"text": fmt.Sprintf("%v", c)},
						}
					}

					toolResult.Content = contentArray
				}

				// 设置 status
				toolResult.Status = "success"
				if block.IsError != nil && *block.IsError {
					toolResult.Status = "error"
					toolResult.IsError = true
				}

				toolResults = append(toolResults, toolResult)
			}
		}
	}

	return toolResults
}

// BuildCodeWhispererRequest 构建 CodeWhisperer 请求
func BuildCodeWhispererRequest(anthropicReq types.AnthropicRequest, ctx *gin.Context) (types.CodeWhispererRequest, error) {

	cwReq := types.CodeWhispererRequest{}

	// 设置代理相关字段 (基于参考文档的标准配置)
	// 使用稳定的代理延续ID生成器，保持会话连续性 (KISS + DRY原则)
	cwReq.ConversationState.AgentContinuationId = utils.GenerateStableAgentContinuationID(ctx)
	cwReq.ConversationState.AgentTaskType = "vibe" // 固定设置为"vibe"，符合参考文档

	// 智能设置ChatTriggerType (KISS: 简化逻辑但保持准确性)
	cwReq.ConversationState.ChatTriggerType = determineChatTriggerType(anthropicReq)

	// 使用稳定的会话ID生成器，基于客户端信息生成持久化的conversationId
	if ctx != nil {
		cwReq.ConversationState.ConversationId = utils.GenerateStableConversationID(ctx)

		// 调试日志：记录会话ID生成信息
		// clientInfo := utils.ExtractClientInfo(ctx)
		// 	logger.String("conversation_id", cwReq.ConversationState.ConversationId),
		// 	logger.String("agent_continuation_id", cwReq.ConversationState.AgentContinuationId),
		// 	logger.String("agent_task_type", cwReq.ConversationState.AgentTaskType),
		// 	logger.String("client_ip", clientInfo["client_ip"]),
		// 	logger.String("user_agent", clientInfo["user_agent"]),
		// 	logger.String("custom_conv_id", clientInfo["custom_conv_id"]),
	} else {
		// 向后兼容：如果没有提供context，仍使用UUID
		cwReq.ConversationState.ConversationId = utils.GenerateUUID()
		logger.Debug("使用随机UUID作为会话ID（向后兼容）",
			logger.String("conversation_id", cwReq.ConversationState.ConversationId),
			logger.String("agent_continuation_id", cwReq.ConversationState.AgentContinuationId),
			logger.String("agent_task_type", cwReq.ConversationState.AgentTaskType))
	}

	// 处理最后一条消息，包括图片
	if len(anthropicReq.Messages) == 0 {
		return cwReq, fmt.Errorf("消息列表为空")
	}

	lastMessage := anthropicReq.Messages[len(anthropicReq.Messages)-1]

	// 调试：记录原始消息内容
	// 	logger.String("role", lastMessage.Role),
	// 	logger.String("content_type", fmt.Sprintf("%T", lastMessage.Content)))

	textContent, images, err := processMessageContent(lastMessage.Content)
	if err != nil {
		return cwReq, fmt.Errorf("处理消息内容失败: %v", err)
	}

	cwReq.ConversationState.CurrentMessage.UserInputMessage.Content = textContent
	// 确保Images字段始终是数组，即使为空
	if len(images) > 0 {
		cwReq.ConversationState.CurrentMessage.UserInputMessage.Images = images
	} else {
		cwReq.ConversationState.CurrentMessage.UserInputMessage.Images = []types.CodeWhispererImage{}
	}

	// 新增：检查并处理 ToolResults
	if lastMessage.Role == "user" {
		toolResults := extractToolResultsFromMessage(lastMessage.Content)
		if len(toolResults) > 0 {
			cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.ToolResults = toolResults

			logger.Debug("已添加工具结果到请求",
				logger.Int("tool_results_count", len(toolResults)),
				logger.String("conversation_id", cwReq.ConversationState.ConversationId))

			// 对于包含 tool_result 的请求，content 应该为空字符串（符合 req2.json 的格式）
			cwReq.ConversationState.CurrentMessage.UserInputMessage.Content = ""
			logger.Debug("工具结果请求，设置 content 为空字符串")
		}
	}

	// 检查模型映射是否存在，如果不存在则返回错误
	modelId := config.ModelMap[anthropicReq.Model]
	if modelId == "" {
		logger.Warn("模型映射不存在",
			logger.String("requested_model", anthropicReq.Model),
			logger.String("request_id", cwReq.ConversationState.AgentContinuationId))

		// 返回模型未找到错误，使用已生成的AgentContinuationId
		return cwReq, types.NewModelNotFoundErrorType(anthropicReq.Model, cwReq.ConversationState.AgentContinuationId)
	}
	cwReq.ConversationState.CurrentMessage.UserInputMessage.ModelId = modelId
	cwReq.ConversationState.CurrentMessage.UserInputMessage.Origin = "AI_EDITOR" // v0.4兼容性：固定使用AI_EDITOR

	// 处理 tools 信息 - 根据req.json实际结构优化工具转换
	if len(anthropicReq.Tools) > 0 {
		// 	logger.Int("tools_count", len(anthropicReq.Tools)),
		// 	logger.String("conversation_id", cwReq.ConversationState.ConversationId))

		var tools []types.CodeWhispererTool
		for i, tool := range anthropicReq.Tools {
			// 验证工具定义的完整性 (SOLID-SRP: 单一责任验证)
			if tool.Name == "" {
				logger.Warn("跳过无名称的工具", logger.Int("tool_index", i))
				continue
			}

			// 过滤不支持的工具（静默过滤，不发送到上游）
			if config.IsUnsupportedTool(tool.Name) {
				continue
			}

			// 	logger.Int("tool_index", i),
			// 	logger.String("tool_name", tool.Name),
			// )

			// 根据req.json的实际结构，确保JSON Schema完整性
			cwTool := types.CodeWhispererTool{}
			cwTool.ToolSpecification.Name = tool.Name

			// 限制 description 长度为 10000 字符
			if len(tool.Description) > config.MaxToolDescriptionLength {
				cwTool.ToolSpecification.Description = tool.Description[:config.MaxToolDescriptionLength]
				logger.Debug("工具描述超长已截断",
					logger.String("tool_name", tool.Name),
					logger.Int("original_length", len(tool.Description)),
					logger.Int("max_length", config.MaxToolDescriptionLength))
			} else {
				cwTool.ToolSpecification.Description = tool.Description
			}

			// 直接使用原始的InputSchema，避免过度处理 (恢复v0.4兼容性)
			cwTool.ToolSpecification.InputSchema = types.InputSchema{
				Json: tool.InputSchema,
			}
			tools = append(tools, cwTool)
		}

		// 工具配置放在 UserInputMessageContext.Tools 中 (符合req.json结构)
		cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.Tools = tools
	}

	// 构建历史消息
	// 条件：有system消息、多条消息、有工具、或启用了thinking
	thinkingEnabled := anthropicReq.Thinking != nil && config.IsThinkingEnabled(anthropicReq.Thinking.Type)
	if len(anthropicReq.System) > 0 || len(anthropicReq.Messages) > 1 || len(anthropicReq.Tools) > 0 || thinkingEnabled {
		var history []any

		// 构建综合系统提示
		var systemContentBuilder strings.Builder

		// 添加原有的 system 消息
		if len(anthropicReq.System) > 0 {
			for _, sysMsg := range anthropicReq.System {
				content, err := utils.GetMessageContent(sysMsg)
				if err == nil {
					systemContentBuilder.WriteString(content)
					systemContentBuilder.WriteString("\n")
				}
			}
		}

		// 如果启用了 thinking，注入思考指令
		if anthropicReq.Thinking != nil && config.IsThinkingEnabled(anthropicReq.Thinking.Type) {
			systemContentBuilder.WriteString("\n")
			systemContentBuilder.WriteString(config.ThinkingPrompt)
			systemContentBuilder.WriteString("\n")
			logger.Debug("已注入 thinking prompt")
		}

		// 如果有系统内容，添加到历史记录 (恢复v0.4结构化类型)
		if systemContentBuilder.Len() > 0 {
			userMsg := types.HistoryUserMessage{}
			userMsg.UserInputMessage.Content = strings.TrimSpace(systemContentBuilder.String())
			userMsg.UserInputMessage.ModelId = modelId
			userMsg.UserInputMessage.Origin = "AI_EDITOR" // v0.4兼容性：固定使用AI_EDITOR
			history = append(history, userMsg)

			assistantMsg := types.HistoryAssistantMessage{}
			assistantMsg.AssistantResponseMessage.Content = "OK"
			assistantMsg.AssistantResponseMessage.ToolUses = nil
			history = append(history, assistantMsg)
		}

		// 然后处理常规消息历史 (修复配对逻辑：合并连续user消息，然后与assistant配对)
		// 关键修复：收集连续的user消息并合并，遇到assistant时配对添加
		var userMessagesBuffer []types.AnthropicRequestMessage // 累积连续的user消息

		// 决定历史消息的循环边界
		// 关键修复：如果最后一条消息是assistant，应该将它加入历史（与前面的user配对）
		// 如果最后一条是user，它作为currentMessage，不加入历史
		historyEndIndex := len(anthropicReq.Messages) - 1
		if lastMessage.Role == "assistant" {
			historyEndIndex = len(anthropicReq.Messages) // 包含最后一条assistant
		}

		for i := 0; i < historyEndIndex; i++ {
			msg := anthropicReq.Messages[i]

			if msg.Role == "user" {
				// 收集user消息到缓冲区
				userMessagesBuffer = append(userMessagesBuffer, msg)
				continue
			}
			if msg.Role == "assistant" {
				// 遇到assistant，只有当有对应的user消息时才处理（忽略孤立assistant）
				if len(userMessagesBuffer) > 0 {
					history = append(history, mergeUserMessages(userMessagesBuffer, modelId))
					userMessagesBuffer = nil

					// 添加assistant消息（只在有配对的user时添加）
					assistantMsg := types.HistoryAssistantMessage{}
					assistantContent, err := utils.GetMessageContent(msg.Content)
					if err == nil {
						assistantMsg.AssistantResponseMessage.Content = assistantContent
					} else {
						assistantMsg.AssistantResponseMessage.Content = ""
					}

					// 提取助手消息中的工具调用
					toolUses := extractToolUsesFromMessage(msg.Content)
					if len(toolUses) > 0 {
						assistantMsg.AssistantResponseMessage.ToolUses = toolUses
					} else {
						assistantMsg.AssistantResponseMessage.ToolUses = nil
					}

					history = append(history, assistantMsg)
				}
				// 如果buffer为空，孤立的assistant消息被忽略（不添加到history）
			}
		}

		// 处理结尾的孤立user消息
		// 如果最后一条是user（作为currentMessage），buffer中可能还有倒数第二条及之前的孤立user消息
		// 这些孤立的user消息应该配对一个"OK"的assistant
		if len(userMessagesBuffer) > 0 {
			history = append(history, mergeUserMessages(userMessagesBuffer, modelId))

			// 自动配对一个"OK"的assistant响应
			autoAssistantMsg := types.HistoryAssistantMessage{}
			autoAssistantMsg.AssistantResponseMessage.Content = "OK"
			autoAssistantMsg.AssistantResponseMessage.ToolUses = nil
			history = append(history, autoAssistantMsg)

			logger.Debug("历史消息末尾存在孤立的user消息，已自动配对assistant",
				logger.Int("orphan_messages", len(userMessagesBuffer)))
		}

		// 限制历史消息长度，防止超过 AWS 上限
		// AWS 实际上限约 163k，为安全起见限制在 180k
		const maxTotalTokens = 180000

		// 估算总 tokens（粗略：1 token ≈ 3 字符，混合中英文）
		estimatedTokens := estimateHistoryTokens(history)

		// 如果超过限制，删除最老的对话（保留 system）
		if estimatedTokens > maxTotalTokens {
			logger.Warn("历史消息超过限制，开始删除最老的对话",
				logger.Int("estimated_tokens", estimatedTokens),
				logger.Int("max_tokens", maxTotalTokens),
				logger.Int("history_count", len(history)))

			// 保留 system 部分（前 2 条：system user + assistant）
			systemSize := 0
			if len(history) >= 2 {
				// 检查前两条是否是 system
				if _, ok := history[0].(types.HistoryUserMessage); ok {
					if _, ok := history[1].(types.HistoryAssistantMessage); ok {
						systemSize = 2
					}
				}
			}

			// 从最老的对话开始删除（每次删除一对 user+assistant）
			for estimatedTokens > maxTotalTokens && len(history) > systemSize+2 {
				// 删除 system 后的第一对
				if len(history) > systemSize+2 {
					history = append(history[:systemSize], history[systemSize+2:]...)
					estimatedTokens = estimateHistoryTokens(history)

					logger.Debug("已删除一轮对话",
						logger.Int("remaining_tokens", estimatedTokens),
						logger.Int("remaining_count", len(history)))
				} else {
					break
				}
			}

			logger.Info("历史消息限制完成",
				logger.Int("final_tokens", estimatedTokens),
				logger.Int("final_count", len(history)))
		}

		cwReq.ConversationState.History = history
	}

	// 最终验证请求完整性 (KISS: 简化验证逻辑)
	if err := validateCodeWhispererRequest(&cwReq); err != nil {
		return cwReq, fmt.Errorf("请求验证失败: %v", err)
	}

	return cwReq, nil
}

// estimateHistoryTokens 估算历史消息的 token 数量
// 使用简单规则：1 token ≈ 3 字符（混合中英文）
func estimateHistoryTokens(history []any) int {
	totalChars := 0

	for _, item := range history {
		switch msg := item.(type) {
		case types.HistoryUserMessage:
			totalChars += len(msg.UserInputMessage.Content)
			// 估算图片 tokens（每张图约 1000 tokens）
			totalChars += len(msg.UserInputMessage.Images) * 3000
			// 估算工具结果
			for _, tr := range msg.UserInputMessage.UserInputMessageContext.ToolResults {
				for _, c := range tr.Content {
					if text, ok := c["text"].(string); ok {
						totalChars += len(text)
					}
				}
			}
		case types.HistoryAssistantMessage:
			totalChars += len(msg.AssistantResponseMessage.Content)
			// 估算工具调用
			for _, tu := range msg.AssistantResponseMessage.ToolUses {
				totalChars += len(tu.Name) * 3
				// 粗略估算 input 的大小
				totalChars += len(fmt.Sprintf("%v", tu.Input))
			}
		}
	}

	// 转换为 tokens（1 token ≈ 3 字符）
	return totalChars / 3
}

// extractToolUsesFromMessage 从助手消息内容中提取工具调用
func extractToolUsesFromMessage(content any) []types.ToolUseEntry {
	var toolUses []types.ToolUseEntry

	switch v := content.(type) {
	case []any:
		for _, item := range v {
			if block, ok := item.(map[string]any); ok {
				if blockType, exists := block["type"]; exists {
					if typeStr, ok := blockType.(string); ok && typeStr == "tool_use" {
						toolUse := types.ToolUseEntry{}

						// 提取 id 作为 ToolUseId
						if id, ok := block["id"].(string); ok {
							toolUse.ToolUseId = id
						}

						// 提取 name
						if name, ok := block["name"].(string); ok {
							toolUse.Name = name
						}

						// 过滤不支持的工具（静默过滤）
						if config.IsUnsupportedTool(toolUse.Name) {
							continue
						}

						// 提取 input
						if input, ok := block["input"].(map[string]any); ok {
							toolUse.Input = input
						} else {
							// 如果 input 不是 map 或不存在，设置为空对象
							toolUse.Input = map[string]any{}
						}

						toolUses = append(toolUses, toolUse)

					}
				}
			}
		}
	case []types.ContentBlock:
		for _, block := range v {
			if block.Type == "tool_use" {
				toolUse := types.ToolUseEntry{}

				if block.ID != nil {
					toolUse.ToolUseId = *block.ID
				}

				if block.Name != nil {
					toolUse.Name = *block.Name
				}

				// 过滤不支持的工具（静默过滤）
				if config.IsUnsupportedTool(toolUse.Name) {
					continue
				}

				if block.Input != nil {
					switch inp := (*block.Input).(type) {
					case map[string]any:
						toolUse.Input = inp
					default:
						toolUse.Input = map[string]any{
							"value": inp,
						}
					}
				} else {
					toolUse.Input = map[string]any{}
				}

				toolUses = append(toolUses, toolUse)
			}
		}
	case string:
		// 如果是纯文本，不包含工具调用
		return nil
	}

	return toolUses
}
