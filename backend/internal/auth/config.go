package auth

import (
	"encoding/json"
	"fmt"
	"os"

	"kiro2api/internal/logger"
)

// TokenStatus token 状态
type TokenStatus string

const (
	TokenStatusActive    TokenStatus = ""          // 正常（默认）
	TokenStatusExhausted TokenStatus = "exhausted" // 额度耗尽
	TokenStatusBanned    TokenStatus = "banned"    // 封禁
)

// AuthConfig 简化的认证配置
type AuthConfig struct {
	AuthType     string      `json:"auth"`
	RefreshToken string      `json:"refreshToken"`
	ClientID     string      `json:"clientId,omitempty"`
	ClientSecret string      `json:"clientSecret,omitempty"`
	Disabled     bool        `json:"disabled,omitempty"`
	Group        string      `json:"group,omitempty"`  // 分组名称
	Name         string      `json:"name,omitempty"`   // 显示名称
	Status       TokenStatus `json:"status,omitempty"` // 状态：空=正常, exhausted=耗尽, banned=封禁
	TokenID      int64       `json:"-"`                // 数据库 ID（内部使用）
}

// defaultGroup 默认分组
var defaultGroup = "default"

// apiKeys 全局 API Keys 配置
var apiKeys []APIKeyConfig

// 认证方法常量
const (
	AuthMethodSocial = "Social"
	AuthMethodIdC    = "IdC"
)

// loadConfigs 从环境变量加载配置（仅用于初始导入，运行时使用数据库）
func loadConfigs() ([]AuthConfig, error) {
	// 检测并警告弃用的环境变量
	deprecatedVars := []string{
		"REFRESH_TOKEN",
		"AWS_REFRESHTOKEN",
		"IDC_REFRESH_TOKEN",
		"BULK_REFRESH_TOKENS",
		"KIRO_AUTH_TOKEN", // 已弃用，使用数据库管理
	}

	for _, envVar := range deprecatedVars {
		if os.Getenv(envVar) != "" {
			logger.Warn("检测到已弃用的环境变量",
				logger.String("变量名", envVar),
				logger.String("迁移说明", "请使用 ./kiro2api import <file> 导入到数据库"))
		}
	}

	// 不再从文件/环境变量加载，返回空配置
	// Token 管理统一通过数据库和 Web UI
	return []AuthConfig{}, nil
}

// APIKeyConfig API Key 配置
type APIKeyConfig struct {
	Key           string   `json:"key"`
	Name          string   `json:"name,omitempty"`
	AllowedGroups []string `json:"allowed_groups"` // 白名单，空=全权限
}

// GlobalConfig 全局配置结构（新格式）
type GlobalConfig struct {
	DefaultGroup string                  `json:"default_group,omitempty"`
	Groups       map[string]*GroupConfig `json:"groups,omitempty"`
	APIKeys      []APIKeyConfig          `json:"api_keys,omitempty"`
	Tokens       []AuthConfig            `json:"tokens"`
}

// parseJSONConfig 解析JSON配置字符串（支持新旧格式）
func parseJSONConfig(jsonData string) ([]AuthConfig, error) {
	// 先尝试新格式：{ "default_group": "x", "groups": {...}, "api_keys": [...], "tokens": [...] }
	var globalConfig GlobalConfig
	if err := json.Unmarshal([]byte(jsonData), &globalConfig); err == nil && len(globalConfig.Tokens) > 0 {
		defaultGroup = globalConfig.DefaultGroup
		apiKeys = globalConfig.APIKeys

		// 初始化分组管理器
		if globalConfig.Groups != nil {
			groupManager.Init(globalConfig.Groups)
		} else {
			groupManager.Init(nil)
		}

		logger.Info("使用新配置格式",
			logger.String("default_group", defaultGroup),
			logger.Int("api_keys", len(apiKeys)),
			logger.Int("groups", len(globalConfig.Groups)))
		return globalConfig.Tokens, nil
	}

	// 旧格式：数组
	var configs []AuthConfig
	if err := json.Unmarshal([]byte(jsonData), &configs); err != nil {
		// 尝试解析为单个对象
		var single AuthConfig
		if err := json.Unmarshal([]byte(jsonData), &single); err != nil {
			return nil, fmt.Errorf("JSON格式无效: %w", err)
		}
		configs = []AuthConfig{single}
	}

	return configs, nil
}

// processConfigs 处理和验证配置
func processConfigs(configs []AuthConfig) []AuthConfig {
	var validConfigs []AuthConfig

	for i, config := range configs {
		// 验证必要字段
		if config.RefreshToken == "" {
			continue
		}

		// 设置默认认证类型
		if config.AuthType == "" {
			config.AuthType = AuthMethodSocial
		}

		// 验证IdC认证的必要字段
		if config.AuthType == AuthMethodIdC {
			if config.ClientID == "" || config.ClientSecret == "" {
				continue
			}
		}

		// 跳过禁用的配置
		if config.Disabled {
			continue
		}

		validConfigs = append(validConfigs, config)
		_ = i // 避免未使用变量警告
	}

	return validConfigs
}

// GetDefaultGroup 获取默认分组
func GetDefaultGroup() string {
	return defaultGroup
}

// GetAPIKeys 获取 API Keys 配置
func GetAPIKeys() []APIKeyConfig {
	return apiKeys
}
