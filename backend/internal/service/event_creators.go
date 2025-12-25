package service

// CreateAnthropicStreamEvents 创建Anthropic流式初始事件
func CreateAnthropicStreamEvents(messageId string, inputTokens int, model string) []map[string]any {
	// 创建基础初始事件序列，不包含content_block_start
	events := []map[string]any{
		{
			"type": "message_start",
			"message": map[string]any{
				"id":            messageId,
				"type":          "message",
				"role":          "assistant",
				"content":       []any{},
				"model":         model,
				"stop_reason":   nil,
				"stop_sequence": nil,
				"usage": map[string]any{
					"input_tokens":  inputTokens,
					"output_tokens": 0, // 初始输出tokens为0，最终在message_delta中更新
				},
			},
		},
		{
			"type": "ping",
		},
	}
	return events
}

// CreateAnthropicFinalEvents 创建Anthropic流式结束事件
func CreateAnthropicFinalEvents(outputTokens, inputTokens int, stopReason string) []map[string]any {
	// 构建符合Claude规范的完整usage信息
	usage := map[string]any{
		"output_tokens": outputTokens,
		"input_tokens":  inputTokens,
	}

	events := []map[string]any{
		{
			"type": "message_delta",
			"delta": map[string]any{
				"stop_reason":   stopReason,
				"stop_sequence": nil,
			},
			"usage": usage,
		},
		{
			"type": "message_stop",
		},
	}

	return events
}
