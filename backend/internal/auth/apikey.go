package auth

import (
	"database/sql"
	"encoding/json"
	"os"
	"sync"

	"kiro2api/internal/logger"
)

// APIKeyManager 管理多个 API key
type APIKeyManager struct {
	mu   sync.RWMutex
	keys map[string]*APIKeyConfig
	db   *sql.DB
}

// NewAPIKeyManager 创建 API Key 管理器
func NewAPIKeyManager(db *sql.DB) *APIKeyManager {
	m := &APIKeyManager{
		keys: make(map[string]*APIKeyConfig),
		db:   db,
	}

	// 优先从数据库加载
	dbKeys := m.loadAPIKeysFromDB()
	for i := range dbKeys {
		cfg := &dbKeys[i]
		if cfg.Key != "" {
			m.keys[cfg.Key] = cfg
		}
	}

	// 向后兼容：如果数据库没有，使用 KIRO_CLIENT_TOKEN
	if len(m.keys) == 0 {
		envKey := os.Getenv("KIRO_CLIENT_TOKEN")
		if envKey != "" {
			cfg := &APIKeyConfig{
				Key:           envKey,
				Name:          "default",
				AllowedGroups: nil,
			}
			m.keys[envKey] = cfg
			// 保存到数据库
			m.saveAPIKeyToDB(cfg)
		}
	}

	return m
}

// Get 获取 API key 配置
func (m *APIKeyManager) Get(key string) *APIKeyConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.keys[key]
}

// HasGroupPermission 检查 key 是否有访问指定 group 的权限
func (m *APIKeyManager) HasGroupPermission(key, group string) bool {
	cfg := m.Get(key)
	if cfg == nil {
		return false
	}
	return cfg.HasGroupPermission(group)
}

// HasGroupPermission 检查是否有访问指定 group 的权限
func (cfg *APIKeyConfig) HasGroupPermission(group string) bool {
	// 空列表 = 全权限
	if len(cfg.AllowedGroups) == 0 {
		return true
	}
	for _, g := range cfg.AllowedGroups {
		if g == group {
			return true
		}
	}
	return false
}

// GetAllowedGroups 获取允许的分组列表
func (m *APIKeyManager) GetAllowedGroups(key string) []string {
	cfg := m.Get(key)
	if cfg == nil {
		return nil
	}
	return cfg.AllowedGroups
}

// IsEmpty 检查是否没有配置任何 API key
func (m *APIKeyManager) IsEmpty() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.keys) == 0
}

// AddKey 添加 API key
func (m *APIKeyManager) AddKey(cfg APIKeyConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keys[cfg.Key] = &cfg
	// 同步到数据库
	m.saveAPIKeyToDB(&cfg)
}

// DeleteKey 删除 API key
func (m *APIKeyManager) DeleteKey(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.keys[key]; !ok {
		return false
	}
	delete(m.keys, key)
	// 同步到数据库
	m.deleteAPIKeyFromDB(key)
	return true
}

// GetAll 获取所有 API key 配置
func (m *APIKeyManager) GetAll() []APIKeyConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]APIKeyConfig, 0, len(m.keys))
	for _, cfg := range m.keys {
		result = append(result, *cfg)
	}
	return result
}

// UpdateAllowedGroups 更新 key 的允许分组
func (m *APIKeyManager) UpdateAllowedGroups(key string, groups []string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg, ok := m.keys[key]
	if !ok {
		return false
	}
	cfg.AllowedGroups = groups
	// 同步到数据库
	m.saveAPIKeyToDB(cfg)
	return true
}

// saveAPIKeyToDB 保存单个 API Key 到数据库
func (m *APIKeyManager) saveAPIKeyToDB(cfg *APIKeyConfig) {
	if m.db == nil {
		return
	}

	allowedGroups := "[]"
	if len(cfg.AllowedGroups) > 0 {
		data, _ := json.Marshal(cfg.AllowedGroups)
		allowedGroups = string(data)
	}

	_, err := m.db.Exec(`INSERT OR REPLACE INTO api_keys (key, name, allowed_groups) VALUES (?, ?, ?)`,
		cfg.Key, cfg.Name, allowedGroups)
	if err != nil {
		logger.Warn("保存API Key到数据库失败", logger.Err(err))
	}
}

// deleteAPIKeyFromDB 从数据库删除 API Key
func (m *APIKeyManager) deleteAPIKeyFromDB(key string) {
	if m.db == nil {
		return
	}

	_, err := m.db.Exec(`DELETE FROM api_keys WHERE key = ?`, key)
	if err != nil {
		logger.Warn("从数据库删除API Key失败", logger.Err(err))
	}
}

// loadAPIKeysFromDB 从数据库加载 API Keys
func (m *APIKeyManager) loadAPIKeysFromDB() []APIKeyConfig {
	if m.db == nil {
		return nil
	}

	rows, err := m.db.Query(`SELECT key, name, allowed_groups FROM api_keys`)
	if err != nil {
		logger.Warn("从数据库加载API Keys失败", logger.Err(err))
		return nil
	}
	defer rows.Close()

	var keys []APIKeyConfig
	for rows.Next() {
		var key, name, allowedGroupsJSON string
		if err := rows.Scan(&key, &name, &allowedGroupsJSON); err != nil {
			continue
		}

		var allowedGroups []string
		json.Unmarshal([]byte(allowedGroupsJSON), &allowedGroups)

		keys = append(keys, APIKeyConfig{
			Key:           key,
			Name:          name,
			AllowedGroups: allowedGroups,
		})
	}
	return keys
}
