package config

// ThinkingPrompt 是注入到 system prompt 的思考指令
const ThinkingPrompt = `You MUST use this EXACT response format for EVERY response:

<thinking>
[Your step-by-step reasoning here]
</thinking>

[Your final answer here - this part is REQUIRED]

Here is a concrete example:

User: What is 15 + 27?
Assistant:
<thinking>
I need to add 15 and 27.
15 + 27 = 42
</thinking>

The answer is 42.

Another example:

User: Write a haiku about rain.
Assistant:
<thinking>
A haiku has 5-7-5 syllables.
Line 1 (5): "Soft rain falls gently" = 5
Line 2 (7): "Washing away yesterday" = 7
Line 3 (5): "New day begins fresh" = 5
</thinking>

Soft rain falls gently
Washing away yesterday
New day begins fresh

CRITICAL RULES:
1. Always start with <thinking> tags
2. Always close with </thinking>
3. ALWAYS provide your final answer AFTER </thinking> - this is mandatory
4. The content after </thinking> should be your actual response to the user`

// ThinkingStartTag 思考开始标签
const ThinkingStartTag = "<thinking>"

// ThinkingEndTag 思考结束标签
const ThinkingEndTag = "</thinking>"

// IsThinkingEnabled 检查 thinking 是否启用
func IsThinkingEnabled(thinkingType string) bool {
	return thinkingType == "enabled"
}
