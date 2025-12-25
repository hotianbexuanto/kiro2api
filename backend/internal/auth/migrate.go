package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"kiro2api/internal/logger"
)

// MigrateFromJSON 从 JSON 文件迁移到 SQLite
func MigrateFromJSON(jsonPath, dbPath string) error {
	// 检查数据库是否已有数据
	if err := InitDB(dbPath); err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}

	repo := NewTokenRepository(GetDB())
	count, err := repo.CountAll()
	if err != nil {
		return fmt.Errorf("检查数据库失败: %w", err)
	}

	if count > 0 {
		logger.Info("数据库已有数据，跳过迁移", logger.Int("token_count", count))
		return nil
	}

	// 检查 JSON 文件是否存在
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		logger.Info("JSON配置文件不存在，跳过迁移", logger.String("path", jsonPath))
		return nil
	}

	// 读取 JSON 文件
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("读取JSON文件失败: %w", err)
	}

	// 解析配置
	var configs []AuthConfig
	var globalCfg struct {
		DefaultGroup string            `json:"default_group"`
		Groups       map[string]any    `json:"groups"`
		APIKeys      []APIKeyConfig    `json:"api_keys"`
		Tokens       []AuthConfig      `json:"tokens"`
	}

	// 尝试新格式
	if err := json.Unmarshal(data, &globalCfg); err == nil && len(globalCfg.Tokens) > 0 {
		configs = globalCfg.Tokens

		// 迁移 API Keys
		if len(globalCfg.APIKeys) > 0 {
			if err := migrateAPIKeys(globalCfg.APIKeys); err != nil {
				logger.Warn("迁移API Keys失败", logger.Err(err))
			}
		}

		// 迁移 Groups
		if len(globalCfg.Groups) > 0 {
			if err := migrateGroups(globalCfg.Groups); err != nil {
				logger.Warn("迁移Groups失败", logger.Err(err))
			}
		}
	} else {
		// 尝试旧格式（数组）
		if err := json.Unmarshal(data, &configs); err != nil {
			// 尝试单个对象
			var single AuthConfig
			if err := json.Unmarshal(data, &single); err != nil {
				return fmt.Errorf("解析JSON失败: %w", err)
			}
			configs = []AuthConfig{single}
		}
	}

	if len(configs) == 0 {
		logger.Info("JSON文件中没有Token配置")
		return nil
	}

	// 转换并批量插入
	tokens := make([]*Token, 0, len(configs))
	for _, cfg := range configs {
		if cfg.RefreshToken == "" {
			continue
		}

		t := FromAuthConfig(cfg)
		if t.AuthType == "" {
			t.AuthType = AuthMethodSocial
		}
		if t.GroupName == "" {
			t.GroupName = "default"
		}
		tokens = append(tokens, t)
	}

	inserted, _, err := repo.BulkInsert(tokens)
	if err != nil {
		return fmt.Errorf("批量插入失败: %w", err)
	}

	logger.Info("JSON迁移完成",
		logger.Int("total", len(tokens)),
		logger.Int("inserted", inserted),
		logger.String("source", jsonPath))

	// 备份原文件
	backupPath := jsonPath + ".migrated." + time.Now().Format("20060102_150405")
	if err := os.Rename(jsonPath, backupPath); err != nil {
		logger.Warn("备份JSON文件失败", logger.Err(err), logger.String("path", jsonPath))
	} else {
		logger.Info("已备份原JSON文件", logger.String("backup", backupPath))
	}

	return nil
}

// migrateAPIKeys 迁移 API Keys
func migrateAPIKeys(keys []APIKeyConfig) error {
	db := GetDB()
	if db == nil {
		return fmt.Errorf("数据库未初始化")
	}

	for _, key := range keys {
		allowedGroups := "[]"
		if len(key.AllowedGroups) > 0 {
			data, _ := json.Marshal(key.AllowedGroups)
			allowedGroups = string(data)
		}

		_, err := db.Exec(`INSERT OR IGNORE INTO api_keys (key, name, allowed_groups) VALUES (?, ?, ?)`,
			key.Key, key.Name, allowedGroups)
		if err != nil {
			logger.Warn("迁移API Key失败", logger.Err(err), logger.String("key", key.Key[:8]+"..."))
		}
	}
	return nil
}

// migrateGroups 迁移 Groups
func migrateGroups(groups map[string]any) error {
	db := GetDB()
	if db == nil {
		return fmt.Errorf("数据库未初始化")
	}

	for name, cfg := range groups {
		displayName := name
		var priority int
		var rateLimitQPS float64
		var rateLimitBurst, cooldownSec int

		if cfgMap, ok := cfg.(map[string]any); ok {
			if dn, ok := cfgMap["display_name"].(string); ok {
				displayName = dn
			}
			if settings, ok := cfgMap["settings"].(map[string]any); ok {
				if p, ok := settings["priority"].(float64); ok {
					priority = int(p)
				}
				if qps, ok := settings["rate_limit_qps"].(float64); ok {
					rateLimitQPS = qps
				}
				if burst, ok := settings["rate_limit_burst"].(float64); ok {
					rateLimitBurst = int(burst)
				}
				if cd, ok := settings["cooldown_sec"].(float64); ok {
					cooldownSec = int(cd)
				}
			}
		}

		_, err := db.Exec(`INSERT OR IGNORE INTO groups (name, display_name, priority, rate_limit_qps, rate_limit_burst, cooldown_sec)
			VALUES (?, ?, ?, ?, ?, ?)`,
			name, displayName, priority, rateLimitQPS, rateLimitBurst, cooldownSec)
		if err != nil {
			logger.Warn("迁移Group失败", logger.Err(err), logger.String("name", name))
		}
	}
	return nil
}

// CheckMigrationNeeded 检查是否需要迁移
func CheckMigrationNeeded(jsonPath, dbPath string) (bool, error) {
	// 检查 JSON 文件是否存在
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		return false, nil
	}

	// 检查数据库是否存在且有数据
	if err := InitDB(dbPath); err != nil {
		return true, nil // 数据库不存在，需要迁移
	}

	repo := NewTokenRepository(GetDB())
	count, err := repo.CountAll()
	if err != nil {
		return true, nil // 查询失败，假设需要迁移
	}

	return count == 0, nil // 数据库为空则需要迁移
}
