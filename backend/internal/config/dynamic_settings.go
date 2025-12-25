package config

import (
	"database/sql"
	"encoding/json"
	"sync"
)

// Settings 可调参数
type Settings struct {
	RateLimitQPS      float64 `json:"rate_limit_qps"`
	RateLimitBurst    int     `json:"rate_limit_burst"`
	RequestTimeoutSec int     `json:"request_timeout_sec"`
	MaxRetries        int     `json:"max_retries"`
	CooldownSec       int     `json:"cooldown_sec"`

	// 单 token 限流与并发控制（0/负数=关闭）
	TokenRateLimitQPS   float64 `json:"token_rate_limit_qps"`
	TokenRateLimitBurst int     `json:"token_rate_limit_burst"`
	TokenMaxConcurrent  int     `json:"token_max_concurrent"`
	GroupMaxConcurrent  int     `json:"group_max_concurrent"`

	// Token 刷新并发数（默认 5）
	RefreshConcurrency int `json:"refresh_concurrency"`

	// 会话 ID 持续时间（分钟，默认 60）
	SessionDurationMin int `json:"session_duration_min"`
}

const settingsKey = "global_settings"

// DefaultSettings 返回默认设置
func DefaultSettings(qps float64, burst int) Settings {
	return Settings{
		RateLimitQPS:      qps,
		RateLimitBurst:    burst,
		RequestTimeoutSec: 120,
		MaxRetries:        2,
		CooldownSec:       30,

		TokenRateLimitQPS:   0,
		TokenRateLimitBurst: 0,
		TokenMaxConcurrent:  2, // 每个token最多2个并发
		GroupMaxConcurrent:  0,

		RefreshConcurrency: 20,
	}
}

// ========== 新版：依赖注入 ==========

var defaultSettingsManager *SettingsManager

// GetDefaultSettingsManager 获取默认的 SettingsManager（向后兼容）
func GetDefaultSettingsManager() *SettingsManager {
	if defaultSettingsManager == nil {
		// 如果没有初始化，返回一个使用默认值的 manager
		defaultSettingsManager = &SettingsManager{
			settings: DefaultSettings(50, 100),
			defaults: DefaultSettings(50, 100),
		}
	}
	return defaultSettingsManager
}

// InitDefaultSettingsManager 初始化默认 SettingsManager
func InitDefaultSettingsManager(db *sql.DB, qps float64, burst int) {
	defaultSettingsManager = NewSettingsManager(db, qps, burst)
}

// SettingsManager 设置管理器（依赖注入版本）
type SettingsManager struct {
	mu       sync.RWMutex
	db       *sql.DB
	settings Settings
	defaults Settings
}

// NewSettingsManager 创建设置管理器
func NewSettingsManager(db *sql.DB, defaultQPS float64, defaultBurst int) *SettingsManager {
	return &SettingsManager{
		db:       db,
		settings: DefaultSettings(defaultQPS, defaultBurst),
		defaults: DefaultSettings(defaultQPS, defaultBurst),
	}
}

// Get 获取当前设置
func (m *SettingsManager) Get() Settings {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings
}

// Update 更新设置（同时持久化）
func (m *SettingsManager) Update(s Settings) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.settings = s

	if m.db != nil {
		data, err := json.Marshal(s)
		if err != nil {
			return err
		}
		_, err = m.db.Exec(`
			INSERT INTO settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
		`, settingsKey, string(data))
		return err
	}
	return nil
}

// Load 从数据库加载设置
func (m *SettingsManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.db == nil {
		return nil
	}

	var value string
	err := m.db.QueryRow("SELECT value FROM settings WHERE key = ?", settingsKey).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil // 使用默认值
		}
		return err
	}

	var saved Settings
	if err := json.Unmarshal([]byte(value), &saved); err != nil {
		return err
	}

	m.settings = saved
	return nil
}
