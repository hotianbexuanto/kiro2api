package auth

import (
	"context"
	"sync/atomic"
	"time"

	"kiro2api/internal/cache"
)

// TokenMetrics 单个 Token 的运行时统计
type TokenMetrics struct {
	tokenID      int64 // Token ID（用于 Redis 同步）
	requestCount int64 // 请求总数
	totalLatency int64 // 总延迟 (纳秒)
	failureCount int64 // 失败次数
	lastRequest  int64 // 最后请求时间 (Unix纳秒)
	inFlight     int64 // 当前正在处理的请求数
}

// RecordRequest 记录一次请求
func (m *TokenMetrics) RecordRequest(latency time.Duration, success bool) {
	atomic.AddInt64(&m.requestCount, 1)
	atomic.AddInt64(&m.totalLatency, int64(latency))
	atomic.StoreInt64(&m.lastRequest, time.Now().UnixNano())
	if !success {
		atomic.AddInt64(&m.failureCount, 1)
	}
}

// AvgLatency 平均延迟 (毫秒)
func (m *TokenMetrics) AvgLatency() float64 {
	count := atomic.LoadInt64(&m.requestCount)
	if count == 0 {
		return 0
	}
	total := atomic.LoadInt64(&m.totalLatency)
	return float64(total) / float64(count) / 1e6 // 纳秒转毫秒
}

// FailureRate 失败率 (0-1)
func (m *TokenMetrics) FailureRate() float64 {
	count := atomic.LoadInt64(&m.requestCount)
	if count == 0 {
		return 0
	}
	failures := atomic.LoadInt64(&m.failureCount)
	return float64(failures) / float64(count)
}

// RequestCount 请求总数
func (m *TokenMetrics) RequestCount() int64 {
	return atomic.LoadInt64(&m.requestCount)
}

// FailureCount 失败次数
func (m *TokenMetrics) FailureCount() int64 {
	return atomic.LoadInt64(&m.failureCount)
}

// IncrementInFlight 增加正在处理的请求数
func (m *TokenMetrics) IncrementInFlight() int64 {
	count := atomic.AddInt64(&m.inFlight, 1)
	// 异步同步到 Redis
	if m.tokenID > 0 && cache.GetDefault().Enabled() {
		go cache.IncrInFlight(context.Background(), m.tokenID)
	}
	return count
}

// DecrementInFlight 减少正在处理的请求数
func (m *TokenMetrics) DecrementInFlight() int64 {
	count := atomic.AddInt64(&m.inFlight, -1)
	// 异步同步到 Redis
	if m.tokenID > 0 && cache.GetDefault().Enabled() {
		go cache.DecrInFlight(context.Background(), m.tokenID)
	}
	return count
}

// InFlightCount 当前正在处理的请求数
func (m *TokenMetrics) InFlightCount() int64 {
	return atomic.LoadInt64(&m.inFlight)
}

// SetTokenID 设置 Token ID（用于 Redis 同步）
func (m *TokenMetrics) SetTokenID(id int64) {
	m.tokenID = id
}

// TokenScore 计算 Token 综合得分
// available: 剩余额度
// avgLatency: 平均延迟 (ms)
// failureRate: 失败率 (0-1)
// 返回: 0-1 之间的得分，越高越好
func TokenScore(available float64, avgLatency float64, failureRate float64) float64 {
	// 额度得分: 0-100 映射到 0-1
	availableScore := min(available/100, 1.0)

	// 速度得分: 0-5000ms 映射到 1-0
	speedScore := 1.0 - min(avgLatency/5000, 1.0)

	// 可靠性得分: 失败率越低越好
	reliableScore := 1.0 - failureRate

	// 加权: 额度 50%, 速度 30%, 可靠性 20%
	return 0.5*availableScore + 0.3*speedScore + 0.2*reliableScore
}

// Restore 从持久化数据恢复统计
func (m *TokenMetrics) Restore(requestCount, failureCount, totalLatencyMs int64) {
	atomic.StoreInt64(&m.requestCount, requestCount)
	atomic.StoreInt64(&m.failureCount, failureCount)
	// 毫秒转纳秒
	atomic.StoreInt64(&m.totalLatency, totalLatencyMs*1e6)
}
