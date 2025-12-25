package auth

import (
	"fmt"
	"sync"

	"kiro2api/internal/logger"
)

// GroupSettings 分组级别设置
type GroupSettings struct {
	Priority       int     `json:"priority,omitempty"`         // 分组优先级
	RateLimitQPS   float64 `json:"rate_limit_qps,omitempty"`   // 0 = 使用全局
	RateLimitBurst int     `json:"rate_limit_burst,omitempty"` // 0 = 使用全局
	CooldownSec    int     `json:"cooldown_sec,omitempty"`     // 0 = 使用全局
}

// GroupConfig 分组配置
type GroupConfig struct {
	Name        string        `json:"name"`
	DisplayName string        `json:"display_name,omitempty"`
	Settings    GroupSettings `json:"settings"`
}

// GroupManager 分组管理器
type GroupManager struct {
	mu     sync.RWMutex
	groups map[string]*GroupConfig
	repo   *TokenRepository // 依赖注入
}

var groupManager = &GroupManager{
	groups: make(map[string]*GroupConfig),
}

// NewGroupManager 创建分组管理器（依赖注入版本）
func NewGroupManager(repo *TokenRepository) *GroupManager {
	gm := &GroupManager{
		groups: make(map[string]*GroupConfig),
		repo:   repo,
	}

	// 从数据库加载分组
	if repo != nil {
		groups, err := repo.GetAllGroups()
		if err != nil {
			logger.Warn("从数据库加载分组失败", logger.Err(err))
		} else {
			gm.groups = groups
			logger.Info("分组加载完成", logger.Int("count", len(groups)))
		}
	}

	// 确保默认分组存在
	if _, ok := gm.groups["default"]; !ok {
		gm.groups["default"] = &GroupConfig{
			Name:        "default",
			DisplayName: "默认",
			Settings:    GroupSettings{},
		}
	}

	return gm
}

// Init 初始化分组（从配置加载）
func (gm *GroupManager) Init(groups map[string]*GroupConfig) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	gm.groups = make(map[string]*GroupConfig)
	for name, cfg := range groups {
		gm.groups[name] = cfg
	}

	// 确保 default 分组存在
	if _, ok := gm.groups["default"]; !ok {
		gm.groups["default"] = &GroupConfig{
			Name:        "default",
			DisplayName: "默认",
			Settings:    GroupSettings{},
		}
	}
}

// Get 获取分组配置
func (gm *GroupManager) Get(name string) *GroupConfig {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	return gm.groups[name]
}

// List 列出所有分组
func (gm *GroupManager) List() []*GroupConfig {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	result := make([]*GroupConfig, 0, len(gm.groups))
	for _, g := range gm.groups {
		result = append(result, g)
	}
	return result
}

// Create 创建分组
func (gm *GroupManager) Create(name, displayName string) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if name == "" {
		return fmt.Errorf("分组名称不能为空")
	}
	if name == "banned" || name == "exhausted" {
		return fmt.Errorf("不能使用保留名称: %s", name)
	}
	if _, exists := gm.groups[name]; exists {
		return fmt.Errorf("分组已存在: %s", name)
	}

	g := &GroupConfig{
		Name:        name,
		DisplayName: displayName,
		Settings:    GroupSettings{},
	}
	gm.groups[name] = g

	// 同步到数据库
	if gm.repo != nil {
		if err := gm.repo.CreateGroup(g); err != nil {
			logger.Warn("保存分组到数据库失败", logger.Err(err), logger.String("name", name))
		}
	}

	logger.Info("创建分组", logger.String("name", name))
	return nil
}

// Update 更新分组设置
func (gm *GroupManager) Update(name string, displayName *string, settings *GroupSettings) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	g, exists := gm.groups[name]
	if !exists {
		return fmt.Errorf("分组不存在: %s", name)
	}

	if displayName != nil {
		g.DisplayName = *displayName
	}
	if settings != nil {
		g.Settings = *settings
	}

	// 同步到数据库
	if gm.repo != nil {
		if err := gm.repo.UpdateGroup(g); err != nil {
			logger.Warn("更新分组到数据库失败", logger.Err(err), logger.String("name", name))
		}
	}

	logger.Info("更新分组", logger.String("name", name))
	return nil
}

// Rename 重命名分组
func (gm *GroupManager) Rename(oldName, newName string) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if oldName == "default" {
		return fmt.Errorf("不能重命名默认分组")
	}
	if newName == "" {
		return fmt.Errorf("新名称不能为空")
	}
	if newName == "banned" || newName == "exhausted" {
		return fmt.Errorf("不能使用保留名称: %s", newName)
	}

	g, exists := gm.groups[oldName]
	if !exists {
		return fmt.Errorf("分组不存在: %s", oldName)
	}
	if _, exists := gm.groups[newName]; exists {
		return fmt.Errorf("目标分组已存在: %s", newName)
	}

	// 同步到数据库
	if gm.repo != nil {
		if err := gm.repo.RenameGroup(oldName, newName); err != nil {
			logger.Warn("重命名分组到数据库失败", logger.Err(err))
		}
	}

	delete(gm.groups, oldName)
	g.Name = newName
	gm.groups[newName] = g

	logger.Info("重命名分组", logger.String("old", oldName), logger.String("new", newName))
	return nil
}

// Delete 删除分组（返回需要移动的 Token 数量）
func (gm *GroupManager) Delete(name string) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if name == "default" {
		return fmt.Errorf("不能删除默认分组")
	}
	if name == "banned" || name == "exhausted" {
		return fmt.Errorf("不能删除系统分组: %s", name)
	}

	if _, exists := gm.groups[name]; !exists {
		return fmt.Errorf("分组不存在: %s", name)
	}

	// 同步到数据库
	if gm.repo != nil {
		if err := gm.repo.DeleteGroup(name); err != nil {
			logger.Warn("从数据库删除分组失败", logger.Err(err), logger.String("name", name))
		}
	}

	delete(gm.groups, name)
	logger.Info("删除分组", logger.String("name", name))
	return nil
}

// Exists 检查分组是否存在
func (gm *GroupManager) Exists(name string) bool {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	_, exists := gm.groups[name]
	return exists
}

// ToMap 导出为 map（用于保存配置）
func (gm *GroupManager) ToMap() map[string]*GroupConfig {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	result := make(map[string]*GroupConfig, len(gm.groups))
	for k, v := range gm.groups {
		result[k] = v
	}
	return result
}

// SaveGroups 保存分组配置（已废弃，分组数据通过 GroupManager 自动保存到数据库）
func SaveGroups() error {
	// 分组数据已通过 GroupManager 的 Create/Update/Delete 方法保存到数据库
	// 此函数保留以兼容现有调用，直接返回 nil
	return nil
}
