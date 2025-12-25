package auth

import (
	"sync"
	"sync/atomic"
	"time"

	"kiro2api/internal/logger"
)

// BackgroundRefresher 后台刷新任务
type BackgroundRefresher struct {
	repo        *TokenRepository
	poolManager *TokenPoolManager // 用于触发缓存同步
	interval    time.Duration
	batchSize   int
	stopChan    chan struct{}
	running     bool
	mu          sync.Mutex
}

// NewBackgroundRefresher 创建后台刷新器
func NewBackgroundRefresher(repo *TokenRepository, poolManager *TokenPoolManager) *BackgroundRefresher {
	return &BackgroundRefresher{
		repo:        repo,
		poolManager: poolManager,
		interval:    time.Minute, // 每分钟运行一次
		batchSize:   50,          // 每次刷新 50 个
		stopChan:    make(chan struct{}),
	}
}

// Start 启动后台刷新
func (r *BackgroundRefresher) Start() {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return
	}
	r.running = true
	r.mu.Unlock()

	go r.run()
	logger.Info("后台Token刷新任务已启动",
		logger.Duration("interval", r.interval),
		logger.Int("batch_size", r.batchSize))
}

// Stop 停止后台刷新
func (r *BackgroundRefresher) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return
	}

	close(r.stopChan)
	r.running = false
	logger.Info("后台Token刷新任务已停止")
}

func (r *BackgroundRefresher) run() {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.refreshBatch()
		case <-r.stopChan:
			return
		}
	}
}

func (r *BackgroundRefresher) refreshBatch() {
	tokens, err := r.repo.FindOldestUnverified(r.batchSize)
	if err != nil {
		logger.Error("获取待刷新Token失败", logger.Err(err))
		return
	}

	if len(tokens) == 0 {
		// 即使没有待刷新的 Token，也要修复孤立的耗尽 Token
		if fixed, err := r.repo.FixOrphanedExhaustedTokens(); err == nil && fixed > 0 {
			logger.Info("修复孤立的耗尽Token", logger.Int("count", fixed))
		}
		return
	}

	// 并发刷新（10个goroutine）
	const concurrency = 10
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var refreshed, failed int32 // 使用 atomic 操作

	for _, t := range tokens {
		wg.Add(1)
		go func(token *Token) {
			defer wg.Done()

			// 限流信号量
			sem <- struct{}{}
			defer func() { <-sem }()

			_, err := r.repo.RefreshSingle(token)
			if err != nil {
				atomic.AddInt32(&failed, 1)
				logger.Debug("后台刷新Token失败",
					logger.Int64("id", token.ID),
					logger.String("error", err.Error()))
			} else {
				atomic.AddInt32(&refreshed, 1)
			}

			// 限速：每个请求间隔 100ms
			time.Sleep(100 * time.Millisecond)
		}(t)
	}

	wg.Wait()

	// 刷新完成后，修复孤立的耗尽 Token
	if fixed, err := r.repo.FixOrphanedExhaustedTokens(); err == nil && fixed > 0 {
		logger.Info("修复孤立的耗尽Token", logger.Int("count", fixed))
	}

	// 触发 poolManager 缓存同步
	if refreshed > 0 && r.poolManager != nil {
		r.poolManager.triggerAsyncRefresh()
		logger.Debug("已触发poolManager缓存同步", logger.Int("refreshed_count", int(refreshed)))
	}

	logger.Debug("后台刷新批次完成",
		logger.Int("total", len(tokens)),
		logger.Int("success", int(refreshed)),
		logger.Int("failed", int(failed)))
}

// RefreshNow 立即刷新一批（手动触发）
func (r *BackgroundRefresher) RefreshNow(limit int) (int, int, error) {
	tokens, err := r.repo.FindOldestUnverified(limit)
	if err != nil {
		return 0, 0, err
	}

	refreshed := 0
	failed := 0

	for _, t := range tokens {
		_, err := r.repo.RefreshSingle(t)
		if err != nil {
			failed++
		} else {
			refreshed++
		}
		time.Sleep(100 * time.Millisecond)
	}

	return refreshed, failed, nil
}
