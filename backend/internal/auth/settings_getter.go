package auth

import "kiro2api/internal/config"

// SettingsGetter 获取设置的回调函数
var SettingsGetter func() config.Settings

// getSettings 获取设置（优先使用注入的 getter）
func getSettings() config.Settings {
	if SettingsGetter != nil {
		return SettingsGetter()
	}
	return config.GetDefaultSettingsManager().Get()
}
