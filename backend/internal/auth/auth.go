package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"kiro2api/internal/logger"
	"kiro2api/internal/types"
)

// AuthService 认证服务（推荐使用依赖注入方式）
type AuthService struct {
	poolManager *TokenPoolManager
	repo        *TokenRepository
}

// NewAuthService 创建新的认证服务
func NewAuthService() (*AuthService, error) {
	logger.Info("创建AuthService实例")

	// 确定数据库路径
	dbPath := os.Getenv("KIRO_DB_PATH")
	if dbPath == "" {
		dbPath = "./data/kiro2api.db"
	}

	// 确定 JSON 配置文件路径（用于迁移）
	jsonPath := findConfigFile()

	// 初始化数据库
	if err := InitDB(dbPath); err != nil {
		return nil, fmt.Errorf("初始化数据库失败: %w", err)
	}

	// 如果存在 JSON 配置且数据库为空，执行迁移
	if jsonPath != "" {
		if err := MigrateFromJSON(jsonPath, dbPath); err != nil {
			logger.Warn("迁移JSON配置失败", logger.Err(err))
			// 继续执行，不阻塞
		}
	}

	// 创建仓库实例
	repo := NewTokenRepository(GetDB())

	// 启动时修复孤立的耗尽 Token
	if fixed, err := repo.FixOrphanedExhaustedTokens(); err == nil && fixed > 0 {
		logger.Info("启动时修复孤立的耗尽Token", logger.Int("count", fixed))
	}

	// 加载配置到内存（兼容现有 poolManager）
	configs, err := loadConfigsFromDB(repo)
	if err != nil {
		return nil, fmt.Errorf("从数据库加载配置失败: %w", err)
	}

	// 从数据库加载分组配置
	groups, err := repo.GetAllGroups()
	if err != nil {
		logger.Warn("从数据库加载分组失败", logger.Err(err))
		groups = nil
	}

	// 从 tokens 表中发现的分组，自动同步到 groups 表
	if stats, err := repo.GetGroupStats(); err == nil {
		for groupName := range stats {
			if groupName == "" {
				continue
			}
			if groups == nil {
				groups = make(map[string]*GroupConfig)
			}
			if _, exists := groups[groupName]; !exists {
				g := &GroupConfig{
					Name:     groupName,
					Settings: GroupSettings{},
				}
				groups[groupName] = g
				// 同步到数据库
				if err := repo.CreateGroup(g); err == nil {
					logger.Info("自动创建分组", logger.String("name", groupName))
				}
			}
		}
	}

	groupManager.Init(groups)

	// 创建 Token 池管理器
	poolManager := NewTokenPoolManager(configs, groupManager, repo)

	// 启动后台刷新任务（传入 poolManager 用于缓存同步）
	refresher := NewBackgroundRefresher(repo, poolManager)
	refresher.Start()

	logger.Info("AuthService创建完成", logger.Int("config_count", len(configs)))

	as := &AuthService{
		poolManager: poolManager,
		repo:        repo,
	}

	// 注册回调：Token刷新后同步更新poolManager缓存
	repo.onTokenUpdate = func(tokenID int64) {
		as.refreshPoolManager()
	}

	return as, nil
}

// findConfigFile 查找配置文件
func findConfigFile() string {
	candidates := []string{
		"./auth_config.json",
		"../auth_config.json",
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			absPath, _ := filepath.Abs(path)
			return absPath
		}
	}
	return ""
}

// loadConfigsFromDB 从数据库加载配置
func loadConfigsFromDB(repo *TokenRepository) ([]AuthConfig, error) {
	tokens, err := repo.ListAll(100000, 0) // 加载所有
	if err != nil {
		return nil, err
	}

	configs := make([]AuthConfig, len(tokens))
	for i, t := range tokens {
		configs[i] = t.ToAuthConfig()
	}
	return configs, nil
}

// GetToken 获取可用的token
func (as *AuthService) GetToken(group string) (types.TokenInfo, error) {
	if as.poolManager == nil {
		return types.TokenInfo{}, fmt.Errorf("token管理器未初始化")
	}
	return as.poolManager.GetBestToken(group)
}

// GetTokenWithUsage 获取可用的token（包含使用信息）
func (as *AuthService) GetTokenWithUsage(group string, sessionID string) (*types.TokenWithUsage, error) {
	if as.poolManager == nil {
		return nil, fmt.Errorf("token管理器未初始化")
	}
	return as.poolManager.GetBestTokenWithUsage(group, sessionID)
}

// GetPoolManager 获取底层的 TokenPoolManager
func (as *AuthService) GetPoolManager() *TokenPoolManager {
	return as.poolManager
}

// MarkTokenFailed 标记 token 失败
func (as *AuthService) MarkTokenFailed(token types.TokenInfo) {
	if as.poolManager != nil {
		as.poolManager.MarkTokenFailed(token)
	}
}

// RecordRequest 记录请求结果
func (as *AuthService) RecordRequest(token types.TokenInfo, latency time.Duration, success bool) {
	if as.poolManager != nil {
		as.poolManager.RecordRequest(token, latency, success)
	}
}

// GetConfigs 获取认证配置（兼容旧接口）
func (as *AuthService) GetConfigs() []AuthConfig {
	tokens, err := as.repo.ListAll(100000, 0)
	if err != nil {
		return nil
	}
	configs := make([]AuthConfig, len(tokens))
	for i, t := range tokens {
		configs[i] = t.ToAuthConfig()
	}
	return configs
}

// GetAllConfigs 获取所有配置
func (as *AuthService) GetAllConfigs() []AuthConfig {
	return as.GetConfigs()
}

// AddConfig 添加单个配置
func (as *AuthService) AddConfig(config AuthConfig) error {
	token := FromAuthConfig(config)
	if err := as.repo.Create(token); err != nil {
		return err
	}

	// 更新 poolManager
	as.refreshPoolManager()
	return nil
}

// AddConfigs 批量添加配置，返回 (inserted, duplicates, error)
func (as *AuthService) AddConfigs(configs []AuthConfig) (int, int, error) {
	tokens := make([]*Token, len(configs))
	for i, cfg := range configs {
		tokens[i] = FromAuthConfig(cfg)
	}

	inserted, duplicates, err := as.repo.BulkInsert(tokens)
	if err != nil {
		return 0, 0, err
	}

	as.refreshPoolManager()
	return inserted, duplicates, nil
}

// RemoveConfig 删除配置（按 ID）
func (as *AuthService) RemoveConfig(id int) error {
	if err := as.repo.Delete(int64(id)); err != nil {
		return err
	}

	as.refreshPoolManager()
	return nil
}

// UpdateConfig 更新配置（按 ID）
func (as *AuthService) UpdateConfig(id int, config AuthConfig) error {
	token := FromAuthConfig(config)
	token.ID = int64(id)
	if err := as.repo.Update(token); err != nil {
		return err
	}

	as.refreshPoolManager()
	return nil
}

// ToggleConfig 切换启用/禁用状态（按 ID）
func (as *AuthService) ToggleConfig(id int) error {
	token, err := as.repo.GetByID(int64(id))
	if err != nil {
		return err
	}

	token.Disabled = !token.Disabled
	if err := as.repo.Update(token); err != nil {
		return err
	}

	as.refreshPoolManager()
	return nil
}

// GetRepository 获取仓库实例
func (as *AuthService) GetRepository() *TokenRepository {
	return as.repo
}

// refreshPoolManager 刷新 poolManager 配置
func (as *AuthService) refreshPoolManager() {
	configs, err := loadConfigsFromDB(as.repo)
	if err != nil {
		logger.Error("刷新poolManager失败", logger.Err(err))
		return
	}
	as.poolManager.UpdateConfigs(configs)
}

// GetTokenByID 通过 ID 获取 Token（懒刷新）
func (as *AuthService) GetTokenByID(id int64) (*Token, error) {
	return as.repo.GetValidToken(id)
}

// ListTokens 分页获取 Token 列表
func (as *AuthService) ListTokens(limit, offset int) ([]*Token, error) {
	return as.repo.ListAll(limit, offset)
}

// ListTokensByGroup 分页获取分组内的 Token
func (as *AuthService) ListTokensByGroup(group string, limit, offset int) ([]*Token, error) {
	return as.repo.ListByGroup(group, limit, offset)
}

// CountTokens 获取 Token 总数
func (as *AuthService) CountTokens() (int, error) {
	return as.repo.CountAll()
}

// CountActiveTokens 获取活跃 Token 数量
func (as *AuthService) CountActiveTokens() (int, error) {
	return as.repo.CountActive()
}

// GetGroupStats 获取分组统计
func (as *AuthService) GetGroupStats() (map[string]struct{ Total, Active int }, error) {
	return as.repo.GetGroupStats()
}

// GetTokenMetrics 获取所有 Token 的运行时统计
func (as *AuthService) GetTokenMetrics() map[int]TokenMetricsInfo {
	if as.poolManager == nil {
		return nil
	}
	return as.poolManager.GetAllMetrics()
}

// GetMetricsByTokenID 通过 TokenID 获取单个 Token 的 metrics
func (as *AuthService) GetMetricsByTokenID(tokenID int64) *TokenMetricsInfo {
	if as.poolManager == nil {
		return nil
	}
	return as.poolManager.GetMetricsByTokenID(tokenID)
}

// GetGlobalInFlightStats 获取全局 in-flight 统计
func (as *AuthService) GetGlobalInFlightStats() (inFlight int64, activeTokens int) {
	if as.poolManager == nil {
		return 0, 0
	}
	return as.poolManager.GetGlobalInFlightStats()
}

// StartRequest 标记开始处理请求
func (as *AuthService) StartRequest(token types.TokenInfo) {
	if as.poolManager != nil {
		as.poolManager.StartRequest(token)
	}
}

// EndRequest 标记请求结束
func (as *AuthService) EndRequest(token types.TokenInfo) {
	if as.poolManager != nil {
		as.poolManager.EndRequest(token)
	}
}

// RestoreTokenMetrics 从持久化数据恢复 token 统计
func (as *AuthService) RestoreTokenMetrics(statsMap map[int]struct {
	RequestCount int64
	FailureCount int64
	TotalLatency int64
}) {
	if as.poolManager != nil {
		as.poolManager.RestoreTokenMetrics(statsMap)
	}
}
