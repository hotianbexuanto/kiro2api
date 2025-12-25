package service

import (
	"strings"

	"kiro2api/internal/config"
)

// ThinkingState 思考状态
type ThinkingState int

const (
	StateText     ThinkingState = iota // 普通文本状态
	StateThinking                      // 思考内容状态
)

// ThinkingParser 流式 thinking 标签解析器
type ThinkingParser struct {
	state       ThinkingState
	buffer      strings.Builder // 缓冲区，用于处理跨块标签
	enabled     bool            // 是否启用 thinking 解析
}

// NewThinkingParser 创建解析器
func NewThinkingParser(enabled bool) *ThinkingParser {
	return &ThinkingParser{
		state:   StateText,
		enabled: enabled,
	}
}

// ParsedChunk 解析后的内容块
type ParsedChunk struct {
	Type    string // "thinking" 或 "text"
	Content string
}

// Parse 解析文本块，返回解析后的内容
func (p *ThinkingParser) Parse(text string) []ParsedChunk {
	if !p.enabled {
		// 未启用时直接返回文本
		return []ParsedChunk{{Type: "text", Content: text}}
	}

	var results []ParsedChunk
	p.buffer.WriteString(text)
	content := p.buffer.String()

	for len(content) > 0 {
		if p.state == StateText {
			// 查找 <thinking> 开始标签
			idx := strings.Index(content, config.ThinkingStartTag)
			if idx == -1 {
				// 没有找到开始标签
				// 检查是否可能是不完整的标签
				if p.mightBePartialTag(content, config.ThinkingStartTag) {
					// 保留在缓冲区等待更多数据
					p.buffer.Reset()
					p.buffer.WriteString(content)
					break
				}
				// 输出所有内容作为文本
				if content != "" {
					results = append(results, ParsedChunk{Type: "text", Content: content})
				}
				p.buffer.Reset()
				break
			}

			// 输出标签前的文本
			if idx > 0 {
				results = append(results, ParsedChunk{Type: "text", Content: content[:idx]})
			}
			// 切换到 thinking 状态
			p.state = StateThinking
			content = content[idx+len(config.ThinkingStartTag):]
		} else {
			// StateThinking: 查找 </thinking> 结束标签
			idx := strings.Index(content, config.ThinkingEndTag)
			if idx == -1 {
				// 没有找到结束标签
				if p.mightBePartialTag(content, config.ThinkingEndTag) {
					p.buffer.Reset()
					p.buffer.WriteString(content)
					break
				}
				// 输出所有内容作为 thinking
				if content != "" {
					results = append(results, ParsedChunk{Type: "thinking", Content: content})
				}
				p.buffer.Reset()
				break
			}

			// 输出标签前的 thinking 内容
			if idx > 0 {
				results = append(results, ParsedChunk{Type: "thinking", Content: content[:idx]})
			}
			// 切换回文本状态
			p.state = StateText
			content = content[idx+len(config.ThinkingEndTag):]
		}
	}

	return results
}

// mightBePartialTag 检查内容末尾是否可能是不完整的标签
func (p *ThinkingParser) mightBePartialTag(content, tag string) bool {
	// 检查内容末尾是否是标签的前缀
	for i := 1; i < len(tag) && i <= len(content); i++ {
		suffix := content[len(content)-i:]
		prefix := tag[:i]
		if suffix == prefix {
			return true
		}
	}
	return false
}

// IsInThinking 返回当前是否在 thinking 状态
func (p *ThinkingParser) IsInThinking() bool {
	return p.state == StateThinking
}

// Reset 重置解析器状态
func (p *ThinkingParser) Reset() {
	p.state = StateText
	p.buffer.Reset()
}
