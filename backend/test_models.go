package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	baseURL = "http://localhost:36600"
	apiKey  = "master-key"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	Model      string    `json:"model"`
	MaxTokens  int       `json:"max_tokens"`
	Messages   []Message `json:"messages"`
	Stream     bool      `json:"stream"`
	Temperature float64  `json:"temperature,omitempty"`
}

type Response struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model       string `json:"model"`
	StopReason  string `json:"stop_reason"`
	Usage       *Usage `json:"usage,omitempty"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// 生成长文本（约5000 tokens）
func generateLongText() string {
	base := `The history of artificial intelligence began in antiquity, with myths, stories and rumors of artificial beings endowed with intelligence or consciousness by master craftsmen. The seeds of modern AI were planted by philosophers who attempted to describe the process of human thinking as the mechanical manipulation of symbols. This work culminated in the invention of the programmable digital computer in the 1940s, a machine based on the abstract essence of mathematical reasoning. This device and the ideas behind it inspired a handful of scientists to begin seriously discussing the possibility of building an electronic brain.

The field of AI research was founded at a workshop held on the campus of Dartmouth College during the summer of 1956. The attendees, including John McCarthy, Marvin Minsky, Allen Newell and Herbert Simon, became the leaders of AI research for many decades. They and their students wrote programs that were, to most people, simply astonishing: computers were learning checkers strategies, solving word problems in algebra, proving logical theorems and speaking English.

By the middle of the 1960s, research in the U.S. was heavily funded by the Department of Defense and laboratories had been established around the world. AI's founders were optimistic about the future: Herbert Simon predicted that "machines will be capable, within twenty years, of doing any work a man can do" and Marvin Minsky agreed, writing that "within a generation... the problem of creating 'artificial intelligence' will substantially be solved".

They failed to recognize the difficulty of some of the remaining tasks. Progress slowed and in 1974, in response to the criticism of Sir James Lighthill and ongoing pressure from the US Congress to fund more productive projects, both the U.S. and British governments cut off exploratory research in AI. The next few years would later be called an "AI winter", a period when obtaining funding for AI projects was difficult.`

	// 重复文本达到约5000 tokens
	var sb strings.Builder
	for i := 0; i < 10; i++ {
		sb.WriteString(base)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func testNonStreamRequest(model string, inputText string, maxTokens int) error {
	fmt.Printf("\n=== 测试 %s (非流式, 输入≈%d字符, 最大输出%d tokens) ===\n",
		model, len(inputText), maxTokens)

	req := Request{
		Model:     model,
		MaxTokens: maxTokens,
		Messages: []Message{
			{Role: "user", Content: inputText},
		},
		Stream: false,
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", baseURL+"/v1/messages", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	start := time.Now()
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	duration := time.Since(start)

	var outputText string
	if len(result.Content) > 0 {
		outputText = result.Content[0].Text
	}

	fmt.Printf("✅ 成功 | 耗时: %v\n", duration)
	fmt.Printf("   输入tokens: %d | 输出tokens: %d\n",
		result.Usage.InputTokens, result.Usage.OutputTokens)
	fmt.Printf("   输出长度: %d字符\n", len(outputText))
	if len(outputText) > 100 {
		fmt.Printf("   输出预览: %s...\n", outputText[:100])
	} else {
		fmt.Printf("   输出: %s\n", outputText)
	}

	return nil
}

func testStreamRequest(model string, inputText string, maxTokens int) error {
	fmt.Printf("\n=== 测试 %s (流式, 输入≈%d字符, 最大输出%d tokens) ===\n",
		model, len(inputText), maxTokens)

	req := Request{
		Model:     model,
		MaxTokens: maxTokens,
		Messages: []Message{
			{Role: "user", Content: inputText},
		},
		Stream: true,
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", baseURL+"/v1/messages", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	start := time.Now()
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	var firstTokenTime time.Duration
	var chunks int
	var outputText strings.Builder
	eventCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		eventCount++
		if eventCount == 1 {
			firstTokenTime = time.Since(start)
		}

		var event map[string]interface{}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		eventType, _ := event["type"].(string)
		if eventType == "content_block_delta" {
			delta, ok := event["delta"].(map[string]interface{})
			if ok {
				if text, ok := delta["text"].(string); ok {
					outputText.WriteString(text)
					chunks++
				}
			}
		}
	}

	duration := time.Since(start)
	output := outputText.String()

	fmt.Printf("✅ 成功 | 总耗时: %v | TTFB: %v\n", duration, firstTokenTime)
	fmt.Printf("   事件数: %d | 内容块: %d\n", eventCount, chunks)
	fmt.Printf("   输出长度: %d字符\n", len(output))
	if len(output) > 100 {
		fmt.Printf("   输出预览: %s...\n", output[:100])
	} else {
		fmt.Printf("   输出: %s\n", output)
	}

	return nil
}

func main() {
	fmt.Println("🚀 开始模型测试")
	fmt.Println("目标服务:", baseURL)
	fmt.Println("=" + strings.Repeat("=", 60))

	longText := generateLongText()
	shortText := "请用一句话解释量子纠缠"

	tests := []struct {
		name      string
		model     string
		input     string
		maxTokens int
		stream    bool
	}{
		// Opus 4.5 测试
		{"Opus 4.5 长输入短输出", "claude-opus-4-5-20251101", longText, 50, false},
		{"Opus 4.5 流式", "claude-opus-4-5-20251101", shortText, 100, true},

		// Sonnet 4.5 测试
		{"Sonnet 4.5 长输入短输出", "claude-sonnet-4-5-20250929", longText, 50, false},
		{"Sonnet 4.5 流式", "claude-sonnet-4-5-20250929", shortText, 100, true},

		// Haiku 4.5 测试
		{"Haiku 4.5 长输入短输出", "claude-haiku-4-5", longText, 50, false},
		{"Haiku 4.5 流式", "claude-haiku-4-5", shortText, 100, true},

		// Sonnet 4 测试（对比）
		{"Sonnet 4 长输入短输出", "claude-sonnet-4-20250514", longText, 50, false},
		{"Sonnet 4 流式", "claude-sonnet-4-20250514", shortText, 100, true},
	}

	successCount := 0
	failCount := 0

	for i, test := range tests {
		fmt.Printf("\n[%d/%d] %s\n", i+1, len(tests), test.name)

		var err error
		if test.stream {
			err = testStreamRequest(test.model, test.input, test.maxTokens)
		} else {
			err = testNonStreamRequest(test.model, test.input, test.maxTokens)
		}

		if err != nil {
			fmt.Printf("❌ 失败: %v\n", err)
			failCount++
		} else {
			successCount++
		}

		// 请求间隔
		if i < len(tests)-1 {
			time.Sleep(time.Second)
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Printf("📊 测试完成: 成功 %d/%d, 失败 %d/%d\n",
		successCount, len(tests), failCount, len(tests))

	if failCount > 0 {
		os.Exit(1)
	}
}
