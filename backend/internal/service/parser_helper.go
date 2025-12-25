package service

import (
	"fmt"
	"time"

	"kiro2api/internal/logger"
	"kiro2api/internal/parser"
)

// ParseWithTimeout 带超时保护的响应解析
func ParseWithTimeout(p *parser.CompliantEventStreamParser, body []byte, timeout time.Duration) (*parser.ParseResult, error) {
	done := make(chan struct{})
	var result *parser.ParseResult
	var err error

	go func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("解析器panic: %v", r)
			}
			close(done)
		}()
		result, err = p.ParseResponse(body)
	}()

	select {
	case <-done:
		return result, err
	case <-time.After(timeout):
		logger.Error("非流式解析超时")
		return nil, fmt.Errorf("解析超时")
	}
}
