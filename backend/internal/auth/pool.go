package auth

import (
	"context"
	"fmt"
	"kiro2api/internal/cache"
	"kiro2api/internal/config"
	"kiro2api/internal/logger"
	"kiro2api/internal/types"
	"sync"
	"sync/atomic"
	"time"
)

// CachedToken 缓存的 Token 信息
type CachedToken struct {
	Token     types.TokenInfo     // Token 信息
	UsageInfo *types.UsageLimits  // 使用限制信息
	CachedAt  time.Time           // 缓存时间
	Available float64             // 可用额度
	LastUsed  time.Time           // 最后使用时间
}

// IsUsable 检查缓存的 token 是否可用
func (c *CachedToken) IsUsable() bool {
	// 检查 token 是否过期
	if c.Token.ExpiresAt.Before(time.Now()) {
		return false
	}
	// 检查是否有 access token
	return c.Token.AccessToken != ""
}

// SimpleTokenCache Token 缓存
type SimpleTokenCache struct {
	mu     sync.RWMutex
	tokens map[string]*CachedToken
	ttl    time.Duration
}

// NewSimpleTokenCache 创建缓存
func NewSimpleTokenCache(ttl time.Duration) *SimpleTokenCache {
	return &SimpleTokenCache{
		tokens: make(map[string]*CachedToken),
		ttl:    ttl,
	}
}

// CalculateAvailableCount 计算可用额度 (base + free_trial)
func CalculateAvailableCount(usage *types.UsageLimits) float64 {
	if usage == nil || len(usage.UsageBreakdownList) == 0 {
		return 0
	}

	for _, breakdown := range usage.UsageBreakdownList {
		if breakdown.ResourceType == "CREDIT" {
			var total float64

			// 基础额度
			total += breakdown.UsageLimitWithPrecision - breakdown.CurrentUsageWithPrecision

			// 免费试用额度
			if breakdown.FreeTrialInfo != nil && breakdown.FreeTrialInfo.FreeTrialStatus == "ACTIVE" {
				total += breakdown.FreeTrialInfo.UsageLimitWithPrecision - breakdown.FreeTrialInfo.CurrentUsageWithPrecision
			}

			if total < 0 {
				return 0
			}
			return total
		}
	}
	return 0
}

// GroupPool 单个分组的 Token 池
type GroupPool struct {
	mu       sync.Mutex
	name     string
	tokens   []*PooledToken        // 该分组的所有 Token
	metrics  map[int]*TokenMetrics // configIndex -> metrics
	cooldown map[int]time.Time     // configIndex -> 冷却结束时间
	settings GroupSettings         // 分组级别设置

	// 轮询索引 (atomic 操作，无锁读取)
	roundRobinIndex uint64
}

// PooledToken 池中的 Token
type PooledToken struct {
	ConfigIndex   int          // 在 configs 中的索引
	Cached        *CachedToken // 缓存的 Token 信息
	Config        *AuthConfig  // 配置引用
	MaxConcurrent int32        // 最大并发数 (0=使用默认值5)
}

// TokenPoolManager 分片锁架构的 Token 池管理器
type TokenPoolManager struct {
	globalMu      sync.RWMutex
	pools         map[string]*GroupPool // group name -> pool
	configs       []AuthConfig          // 所有配置
	cache         *SimpleTokenCache     // 共享缓存
	lastRefresh   time.Time
	tokenIDToIdx  map[int64]int         // TokenID -> configIndex 映射
	refreshing    int32                 // 异步刷新标志 (atomic)
	groupMgr      *GroupManager         // 分组管理器
	repo          *TokenRepository      // Token 仓库
}

// NewTokenPoolManager 创建分片锁 Token 池管理器
func NewTokenPoolManager(configs []AuthConfig, groupMgr *GroupManager, repo *TokenRepository) *TokenPoolManager {
	tpm := &TokenPoolManager{
		pools:        make(map[string]*GroupPool),
		configs:      configs,
		cache:        NewSimpleTokenCache(config.TokenCacheTTL),
		tokenIDToIdx: make(map[int64]int),
		groupMgr:     groupMgr,
		repo:         repo,
	}
	tpm.rebuildPools()

	// 启动时同步初始化缓存（避免第一次请求失败）
	tpm.initCacheSync()

	return tpm
}

// initCacheSync 同步初始化缓存（启动时调用）
func (tpm *TokenPoolManager) initCacheSync() {
	logger.Info("开始全量刷新Token缓存", logger.Int("config_count", len(tpm.configs)))

	// 收集需要刷新的配置
	type refreshTask struct {
		index int
		cfg   AuthConfig
	}
	var tasks []refreshTask
	for i, cfg := range tpm.configs {
		if cfg.Disabled || cfg.Status == TokenStatusBanned || cfg.Status == TokenStatusExhausted {
			continue
		}
		tasks = append(tasks, refreshTask{index: i, cfg: cfg})
	}

	if len(tasks) == 0 {
		logger.Warn("没有可用的Token需要刷新")
		return
	}

	// 并发刷新（20个goroutine）
	const concurrency = 20
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var refreshed, failed int32

	for _, task := range tasks {
		wg.Add(1)
		go func(t refreshTask) {
			defer wg.Done()

			// 限流信号量
			sem <- struct{}{}
			defer func() { <-sem }()

			cacheKey := fmt.Sprintf(config.TokenCacheKeyFormat, t.index)
			token, err := refreshSingleTokenStatic(t.cfg)
			if err != nil {
				atomic.AddInt32(&failed, 1)
				logger.Debug("刷新Token失败",
					logger.Int("index", t.index),
					logger.Int64("token_id", t.cfg.TokenID),
					logger.String("error", err.Error()))
				return
			}

			// 刷新成功，写入缓存
			token.ID = t.cfg.TokenID
			tpm.cache.mu.Lock()
			tpm.cache.tokens[cacheKey] = &CachedToken{
				Token:     token,
				CachedAt:  time.Now(),
				Available: 550,
			}
			tpm.cache.mu.Unlock()
			atomic.AddInt32(&refreshed, 1)

			// 限速：每个请求间隔 50ms
			time.Sleep(50 * time.Millisecond)
		}(task)
	}

	wg.Wait()

	logger.Info("Token缓存初始化完成",
		logger.Int("total", len(tasks)),
		logger.Int("refreshed", int(refreshed)),
		logger.Int("failed", int(failed)))
}

// rebuildPools 根据配置重建分组池
func (tpm *TokenPoolManager) rebuildPools() {
	// 重建 TokenID -> configIndex 映射
	tpm.tokenIDToIdx = make(map[int64]int)
	for i := range tpm.configs {
		if tpm.configs[i].TokenID > 0 {
			tpm.tokenIDToIdx[tpm.configs[i].TokenID] = i
		}
	}

	// 按分组分类 Token
	groupTokens := make(map[string][]*PooledToken)
	for i := range tpm.configs {
		cfg := &tpm.configs[i]
		group := cfg.Group
		if group == "" {
			group = GetDefaultGroup()
		}
		groupTokens[group] = append(groupTokens[group], &PooledToken{
			ConfigIndex: i,
			Config:      cfg,
		})
	}

	// 创建/更新分组池
	for groupName, tokens := range groupTokens {
		pool, exists := tpm.pools[groupName]
		if !exists {
			pool = &GroupPool{
				name:     groupName,
				metrics:  make(map[int]*TokenMetrics),
				cooldown: make(map[int]time.Time),
			}
			// 加载分组设置
			if tpm.groupMgr != nil {
				if gc := tpm.groupMgr.Get(groupName); gc != nil {
					pool.settings = gc.Settings
				}
			}
			tpm.pools[groupName] = pool
		}
		pool.tokens = tokens
	}

	// 清理空分组
	for name, pool := range tpm.pools {
		if len(pool.tokens) == 0 {
			delete(tpm.pools, name)
		}
	}

	logger.Info("Token池重建完成",
		logger.Int("group_count", len(tpm.pools)),
		logger.Int("total_tokens", len(tpm.configs)))
}

// GetBestToken 获取最优 Token (加权随机选择)
func (tpm *TokenPoolManager) GetBestToken(group string) (types.TokenInfo, error) {
	result, err := tpm.GetBestTokenWithUsage(group)
	if err != nil {
		return types.TokenInfo{}, err
	}
	return result.TokenInfo, nil
}

// GetBestTokenWithUsage 获取最优 Token (包含使用信息)
func (tpm *TokenPoolManager) GetBestTokenWithUsage(group string) (*types.TokenWithUsage, error) {
	if group == "" {
		group = GetDefaultGroup()
	}

	// 获取分组池 (读锁) 并保存必要引用
	tpm.globalMu.RLock()
	pool, exists := tpm.pools[group]
	needRefresh := time.Since(tpm.lastRefresh) > config.TokenCacheTTL
	cacheRef := tpm.cache // 保存cache引用
	tpm.globalMu.RUnlock()

	if !exists || len(pool.tokens) == 0 {
		return nil, fmt.Errorf("分组 %s 没有可用的 Token", group)
	}

	// 异步刷新缓存（不阻塞）
	if needRefresh {
		tpm.triggerAsyncRefresh()
	}

	// 在分组池内选择 (轮询，使用保存的cacheRef)
	selected := pool.roundRobinSelect(cacheRef)
	if selected == nil {
		return nil, fmt.Errorf("分组 %s 没有可用的 Token", group)
	}

	// 更新使用信息（加锁保护Available修改）
	selected.Cached.LastUsed = time.Now()
	cacheRef.mu.Lock()
	if selected.Cached.Available > 0 {
		selected.Cached.Available--
	}
	cacheRef.mu.Unlock()

	// 设置 Token ID 用于追踪
	tokenInfo := selected.Cached.Token
	tokenInfo.ID = selected.Config.TokenID

	logger.Debug("选择Token",
		logger.Int64("token_id", tokenInfo.ID),
		logger.Int("config_index", selected.ConfigIndex))

	return &types.TokenWithUsage{
		TokenInfo:       tokenInfo,
		UsageLimits:     selected.Cached.UsageInfo,
		AvailableCount:  selected.Cached.Available,
		LastUsageCheck:  selected.Cached.LastUsed,
		IsUsageExceeded: selected.Cached.Available <= 0,
	}, nil
}

// roundRobinSelect 轮询选择 (无锁读取，原子操作)
// 调用者不需要持有 pool.mu
func (gp *GroupPool) roundRobinSelect(cache *SimpleTokenCache) *PooledToken {
	now := time.Now()
	tokenCount := len(gp.tokens)
	if tokenCount == 0 {
		return nil
	}

	const defaultMaxConcurrent = int32(5) // 默认最大并发
	const maxRetries = 5                  // 最多重试 5 次
	const retryDelay = 50 * time.Millisecond // 每次重试间隔 50ms

	// 多轮遍历，避免高并发时所有 token 都达到上限导致失败
	for retry := 0; retry < maxRetries; retry++ {
		// 遍历一轮所有 token
		for i := 0; i < tokenCount; i++ {
			// 原子获取并递增索引
			idx := atomic.AddUint64(&gp.roundRobinIndex, 1)
			pt := gp.tokens[int(idx)%tokenCount]

			// 跳过禁用/封禁/耗尽
			if pt.Config.Disabled || pt.Config.Status == TokenStatusBanned || pt.Config.Status == TokenStatusExhausted {
				continue
			}

			// 跳过冷却中
			if cooldownUntil, ok := gp.cooldown[pt.ConfigIndex]; ok && now.Before(cooldownUntil) {
				continue
			}

			// 获取缓存（加读锁保护并发访问）
			cacheKey := fmt.Sprintf(config.TokenCacheKeyFormat, pt.ConfigIndex)
			cache.mu.RLock()
			cached, exists := cache.tokens[cacheKey]
			cache.mu.RUnlock()
			if !exists || !cached.IsUsable() || cached.Available <= 0 {
				continue
			}
			pt.Cached = cached

			// 检查并发限制
			maxConcurrent := pt.MaxConcurrent
			if maxConcurrent <= 0 {
				maxConcurrent = defaultMaxConcurrent
			}

			currentInFlight := gp.getMetrics(pt.ConfigIndex, pt.Config.TokenID).InFlightCount()
			if currentInFlight >= int64(maxConcurrent) {
				continue
			}

			// 找到可用的 Token
			if retry > 0 {
				logger.Debug("轮询选择Token（重试后成功）",
					logger.Int("config_index", pt.ConfigIndex),
					logger.Int("retry", retry),
					logger.Int("max_concurrent", int(maxConcurrent)),
					logger.Int64("current_inflight", currentInFlight),
					logger.Float64("available", cached.Available))
			} else {
				logger.Debug("轮询选择Token",
					logger.Int("config_index", pt.ConfigIndex),
					logger.Int("max_concurrent", int(maxConcurrent)),
					logger.Int64("current_inflight", currentInFlight),
					logger.Float64("available", cached.Available))
			}
			return pt
		}

		// 一轮遍历后没找到，等待一小段时间后重试
		if retry < maxRetries-1 {
			logger.Warn("所有Token暂时不可用，等待后重试",
				logger.Int("retry", retry+1),
				logger.Int("token_count", tokenCount),
				logger.String("group", gp.name))
			time.Sleep(retryDelay)
			now = time.Now() // 更新时间，重新检查冷却状态
		}
	}

	// 多轮重试后仍然没有可用 token
	logger.Error("多轮重试后仍无可用Token",
		logger.Int("token_count", tokenCount),
		logger.Int("max_retries", maxRetries),
		logger.String("group", gp.name))
	return nil
}

// getMetrics 获取或创建 metrics
func (gp *GroupPool) getMetrics(configIndex int, tokenID int64) *TokenMetrics {
	if m, ok := gp.metrics[configIndex]; ok {
		return m
	}
	m := &TokenMetrics{}
	m.SetTokenID(tokenID)
	gp.metrics[configIndex] = m
	return m
}

// RecordRequest 记录请求结果
func (tpm *TokenPoolManager) RecordRequest(token types.TokenInfo, latency time.Duration, success bool) {
	// 1. 复制pools快照（短暂持读锁）
	tpm.globalMu.RLock()
	poolsSnapshot := make([]*GroupPool, 0, len(tpm.pools))
	for _, pool := range tpm.pools {
		poolsSnapshot = append(poolsSnapshot, pool)
	}
	cacheSnapshot := tpm.cache
	tpm.globalMu.RUnlock()

	// 2. 无globalMu的情况下操作pool.mu
	for _, pool := range poolsSnapshot {
		pool.mu.Lock()
		for _, pt := range pool.tokens {
			cacheKey := fmt.Sprintf(config.TokenCacheKeyFormat, pt.ConfigIndex)
			cacheSnapshot.mu.RLock()
			cached, ok := cacheSnapshot.tokens[cacheKey]
			cacheSnapshot.mu.RUnlock()
			if ok {
				if cached.Token.AccessToken == token.AccessToken {
					pool.getMetrics(pt.ConfigIndex, pt.Config.TokenID).RecordRequest(latency, success)
					pool.mu.Unlock()
					return
				}
			}
		}
		pool.mu.Unlock()
	}
}

// MarkTokenFailed 标记 Token 失败 (触发冷却)
func (tpm *TokenPoolManager) MarkTokenFailed(token types.TokenInfo) {
	cooldownDuration := time.Duration(config.GetDefaultSettingsManager().Get().CooldownSec) * time.Second
	if cooldownDuration <= 0 {
		cooldownDuration = config.TokenCooldownDuration
	}

	// 1. 复制pools快照（短暂持读锁）
	tpm.globalMu.RLock()
	poolsSnapshot := make([]*GroupPool, 0, len(tpm.pools))
	for _, pool := range tpm.pools {
		poolsSnapshot = append(poolsSnapshot, pool)
	}
	cacheSnapshot := tpm.cache
	tpm.globalMu.RUnlock()

	// 2. 无globalMu的情况下操作pool.mu
	for _, pool := range poolsSnapshot {
		pool.mu.Lock()
		for _, pt := range pool.tokens {
			cacheKey := fmt.Sprintf(config.TokenCacheKeyFormat, pt.ConfigIndex)
			cacheSnapshot.mu.RLock()
			cached, ok := cacheSnapshot.tokens[cacheKey]
			cacheSnapshot.mu.RUnlock()
			if ok {
				if cached.Token.AccessToken == token.AccessToken {
					pool.cooldown[pt.ConfigIndex] = time.Now().Add(cooldownDuration)
					pool.getMetrics(pt.ConfigIndex, pt.Config.TokenID).RecordRequest(0, false)
					logger.Warn("Token标记冷却",
						logger.Int("config_index", pt.ConfigIndex),
						logger.Duration("cooldown", cooldownDuration))
					pool.mu.Unlock()
					return
				}
			}
		}
		pool.mu.Unlock()
	}
}

// triggerAsyncRefresh 触发异步刷新（不阻塞）
func (tpm *TokenPoolManager) triggerAsyncRefresh() {
	// CAS 防止重复刷新
	if !atomic.CompareAndSwapInt32(&tpm.refreshing, 0, 1) {
		return
	}

	go func() {
		defer atomic.StoreInt32(&tpm.refreshing, 0)
		tpm.doRefresh()
	}()
}

// doRefresh 执行刷新（后台 goroutine）
func (tpm *TokenPoolManager) doRefresh() {
	logger.Debug("开始异步刷新Token缓存")

	configChanged := false

	// 读取配置快照（短暂持锁）
	tpm.globalMu.RLock()
	configs := make([]AuthConfig, len(tpm.configs))
	copy(configs, tpm.configs)
	tpm.globalMu.RUnlock()

	for i, cfg := range configs {
		if cfg.Disabled || cfg.Status == TokenStatusBanned {
			continue
		}

		// 刷新 Token（无锁）
		token, err := refreshSingleTokenStatic(cfg)
		if err != nil {
			if isBannedError(err) {
				logger.Warn("检测到Token被封禁", logger.Int("config_index", i))
				tpm.globalMu.Lock()
				tpm.configs[i].Status = TokenStatusBanned
				tpm.configs[i].Group = "banned"
				tpm.globalMu.Unlock()
				// 更新数据库
				if cfg.TokenID > 0 && tpm.repo != nil {
					tpm.repo.UpdateTokenStatus(cfg.TokenID, string(TokenStatusBanned), "banned")
				}
				configChanged = true
			}
			continue
		}

		// 检查使用限制（无锁）
		var usageInfo *types.UsageLimits
		var available float64

		checker := NewUsageLimitsChecker()
		if usage, checkErr := checker.CheckUsageLimits(token); checkErr == nil {
			usageInfo = usage
			available = CalculateAvailableCount(usage)

			if available <= 0 {
				tpm.globalMu.Lock()
				if tpm.configs[i].Status != TokenStatusExhausted {
					logger.Warn("检测到Token额度耗尽", logger.Int("config_index", i))
					tpm.configs[i].Status = TokenStatusExhausted
					tpm.configs[i].Group = "exhausted"
					configChanged = true
					// 更新数据库
					if cfg.TokenID > 0 && tpm.repo != nil {
						tpm.repo.UpdateTokenStatus(cfg.TokenID, string(TokenStatusExhausted), "exhausted")
					}
				}
				tpm.globalMu.Unlock()
			} else if available > 0 {
				// 检测额度恢复
				tpm.globalMu.Lock()
				if tpm.configs[i].Status == TokenStatusExhausted {
					logger.Info("Token额度已恢复", logger.Int("config_index", i), logger.Float64("available", available))
					tpm.configs[i].Status = TokenStatusActive
					tpm.configs[i].Group = GetDefaultGroup()
					configChanged = true
					// 更新数据库
					if cfg.TokenID > 0 && tpm.repo != nil {
						tpm.repo.UpdateTokenStatus(cfg.TokenID, string(TokenStatusActive), GetDefaultGroup())
					}
				}
				tpm.globalMu.Unlock()
			}
		} else if isSuspendedError(checkErr) {
			logger.Warn("检测到Token被暂停", logger.Int("config_index", i))
			tpm.globalMu.Lock()
			tpm.configs[i].Status = TokenStatusBanned
			tpm.configs[i].Group = "banned"
			tpm.globalMu.Unlock()
			// 更新数据库
			if cfg.TokenID > 0 && tpm.repo != nil {
				tpm.repo.UpdateTokenStatus(cfg.TokenID, string(TokenStatusBanned), "banned")
			}
			configChanged = true
		}

		// 更新内存缓存（使用cache.mu保护并发访问）
		cacheKey := fmt.Sprintf(config.TokenCacheKeyFormat, i)
		tpm.cache.mu.Lock()
		tpm.cache.tokens[cacheKey] = &CachedToken{
			Token:     token,
			UsageInfo: usageInfo,
			CachedAt:  time.Now(),
			Available: available,
		}
		tpm.cache.mu.Unlock()
	}

	if configChanged {
		tpm.globalMu.Lock()
		tpm.rebuildPools()
		tpm.globalMu.Unlock()
	}

	tpm.globalMu.Lock()
	tpm.lastRefresh = time.Now()
	tpm.globalMu.Unlock()

	logger.Debug("异步刷新Token缓存完成")
}

// UpdateConfigs 热更新配置
func (tpm *TokenPoolManager) UpdateConfigs(configs []AuthConfig) {
	tpm.globalMu.Lock()
	defer tpm.globalMu.Unlock()

	tpm.configs = configs
	tpm.cache.tokens = make(map[string]*CachedToken)
	tpm.lastRefresh = time.Time{}
	tpm.rebuildPools()

	logger.Info("TokenPoolManager配置已热更新",
		logger.Int("config_count", len(configs)))
}

// GetConfigs 获取当前配置
func (tpm *TokenPoolManager) GetConfigs() []AuthConfig {
	tpm.globalMu.RLock()
	defer tpm.globalMu.RUnlock()
	return tpm.configs
}

// GetPoolStats 获取池统计信息
func (tpm *TokenPoolManager) GetPoolStats() map[string]map[string]interface{} {
	tpm.globalMu.RLock()
	defer tpm.globalMu.RUnlock()

	stats := make(map[string]map[string]interface{})
	for name, pool := range tpm.pools {
		pool.mu.Lock()
		poolStats := map[string]interface{}{
			"token_count":    len(pool.tokens),
			"cooldown_count": len(pool.cooldown),
		}
		pool.mu.Unlock()
		stats[name] = poolStats
	}
	return stats
}

// TokenMetricsInfo 单个 Token 的 metrics 信息
type TokenMetricsInfo struct {
	ConfigIndex  int
	RequestCount int64
	FailureCount int64
	InFlight     int64
	AvgLatency   float64
}

// GetAllMetrics 获取所有 Token 的 metrics 统计
func (tpm *TokenPoolManager) GetAllMetrics() map[int]TokenMetricsInfo {
	tpm.globalMu.RLock()
	defer tpm.globalMu.RUnlock()

	result := make(map[int]TokenMetricsInfo)
	for _, pool := range tpm.pools {
		pool.mu.Lock()
		for idx, m := range pool.metrics {
			result[idx] = TokenMetricsInfo{
				ConfigIndex:  idx,
				RequestCount: m.RequestCount(),
				FailureCount: m.FailureCount(),
				InFlight:     m.InFlightCount(),
				AvgLatency:   m.AvgLatency(),
			}
		}
		pool.mu.Unlock()
	}
	return result
}

// GetGlobalInFlightStats 获取全局 in-flight 统计
func (tpm *TokenPoolManager) GetGlobalInFlightStats() (inFlight int64, activeTokens int) {
	// 优先从 Redis 读取（无锁）
	if cache.GetDefault().Enabled() {
		stats, err := cache.GetAllInFlight(context.Background())
		if err == nil && len(stats) > 0 {
			for _, count := range stats {
				inFlight += count
				if count > 0 {
					activeTokens++
				}
			}
			return inFlight, activeTokens
		}
	}

	// 回退到内存读取
	tpm.globalMu.RLock()
	defer tpm.globalMu.RUnlock()

	for _, pool := range tpm.pools {
		pool.mu.Lock()
		for _, m := range pool.metrics {
			count := m.InFlightCount()
			inFlight += count
			if count > 0 {
				activeTokens++
			}
		}
		pool.mu.Unlock()
	}
	return inFlight, activeTokens
}

// GetMetricsByTokenID 通过 TokenID 获取 metrics
func (tpm *TokenPoolManager) GetMetricsByTokenID(tokenID int64) *TokenMetricsInfo {
	tpm.globalMu.RLock()
	defer tpm.globalMu.RUnlock()

	configIdx, ok := tpm.tokenIDToIdx[tokenID]
	if !ok {
		// TokenID 不在映射中，返回默认值
		return &TokenMetricsInfo{}
	}

	for _, pool := range tpm.pools {
		pool.mu.Lock()
		if m, exists := pool.metrics[configIdx]; exists {
			info := &TokenMetricsInfo{
				ConfigIndex:  configIdx,
				RequestCount: m.RequestCount(),
				FailureCount: m.FailureCount(),
				InFlight:     m.InFlightCount(),
				AvgLatency:   m.AvgLatency(),
			}
			pool.mu.Unlock()
			return info
		}
		pool.mu.Unlock()
	}
	// metrics 不存在（从未被请求），返回默认值
	return &TokenMetricsInfo{ConfigIndex: configIdx}
}

// StartRequest 标记开始处理请求 (增加 in-flight)
func (tpm *TokenPoolManager) StartRequest(token types.TokenInfo) {
	// 1. 复制pools快照（短暂持读锁）
	tpm.globalMu.RLock()
	poolsSnapshot := make([]*GroupPool, 0, len(tpm.pools))
	for _, pool := range tpm.pools {
		poolsSnapshot = append(poolsSnapshot, pool)
	}
	cacheSnapshot := tpm.cache
	tpm.globalMu.RUnlock()

	// 2. 无globalMu的情况下操作pool.mu
	for _, pool := range poolsSnapshot {
		pool.mu.Lock()
		for _, pt := range pool.tokens {
			cacheKey := fmt.Sprintf(config.TokenCacheKeyFormat, pt.ConfigIndex)
			cacheSnapshot.mu.RLock()
			cached, ok := cacheSnapshot.tokens[cacheKey]
			cacheSnapshot.mu.RUnlock()
			if ok {
				if cached.Token.AccessToken == token.AccessToken {
					pool.getMetrics(pt.ConfigIndex, pt.Config.TokenID).IncrementInFlight()
					pool.mu.Unlock()
					return
				}
			}
		}
		pool.mu.Unlock()
	}
}

// EndRequest 标记请求结束 (减少 in-flight)
func (tpm *TokenPoolManager) EndRequest(token types.TokenInfo) {
	// 1. 复制pools快照（短暂持读锁）
	tpm.globalMu.RLock()
	poolsSnapshot := make([]*GroupPool, 0, len(tpm.pools))
	for _, pool := range tpm.pools {
		poolsSnapshot = append(poolsSnapshot, pool)
	}
	cacheSnapshot := tpm.cache
	tpm.globalMu.RUnlock()

	// 2. 无globalMu的情况下操作pool.mu
	for _, pool := range poolsSnapshot {
		pool.mu.Lock()
		for _, pt := range pool.tokens {
			cacheKey := fmt.Sprintf(config.TokenCacheKeyFormat, pt.ConfigIndex)
			cacheSnapshot.mu.RLock()
			cached, ok := cacheSnapshot.tokens[cacheKey]
			cacheSnapshot.mu.RUnlock()
			if ok {
				if cached.Token.AccessToken == token.AccessToken {
					pool.getMetrics(pt.ConfigIndex, pt.Config.TokenID).DecrementInFlight()
					pool.mu.Unlock()
					return
				}
			}
		}
		pool.mu.Unlock()
	}
}

// 辅助函数
func isBannedError(err error) bool {
	s := err.Error()
	return (contains(s, "401") && contains(s, "Bad credentials"))
}

func isSuspendedError(err error) bool {
	s := err.Error()
	return contains(s, "TEMPORARILY_SUSPENDED") || contains(s, "suspended")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// refreshSingleTokenStatic 静态版本的刷新单个 Token
func refreshSingleTokenStatic(cfg AuthConfig) (types.TokenInfo, error) {
	// 根据认证类型选择刷新方法
	if cfg.AuthType == AuthMethodIdC {
		return RefreshIdCToken(cfg)
	}
	return RefreshSocialToken(cfg.RefreshToken)
}

// RestoreTokenMetrics 从持久化数据恢复 token 统计
// statsMap: configIndex -> {RequestCount, FailureCount, TotalLatency}
func (tpm *TokenPoolManager) RestoreTokenMetrics(statsMap map[int]struct {
	RequestCount int64
	FailureCount int64
	TotalLatency int64
}) {
	tpm.globalMu.Lock()
	defer tpm.globalMu.Unlock()

	restored := 0
	for _, pool := range tpm.pools {
		pool.mu.Lock()
		for _, pt := range pool.tokens {
			if s, ok := statsMap[pt.ConfigIndex]; ok {
				m := pool.getMetrics(pt.ConfigIndex, pt.Config.TokenID)
				m.Restore(s.RequestCount, s.FailureCount, s.TotalLatency)
				restored++
			}
		}
		pool.mu.Unlock()
	}

	logger.Info("Token统计已恢复", logger.Int("restored_count", restored))
}
