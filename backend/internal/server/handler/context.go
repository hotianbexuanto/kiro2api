package handler

import (
	"kiro2api/internal/auth"
	"kiro2api/internal/config"
	"kiro2api/internal/service"
	"kiro2api/internal/stats"
)

// Context 保存 handler 需要的依赖
type Context struct {
	RateLimiter    *service.RateLimiter
	SettingsMgr    *config.SettingsManager
	GroupMgr       *auth.GroupManager
	StatsCollector *stats.Collector
}

var globalCtx *Context

// InitContext 初始化全局 handler context
func InitContext(ctx *Context) {
	globalCtx = ctx
}

// GetRateLimiter 获取限流器
func GetRateLimiter() *service.RateLimiter {
	if globalCtx == nil {
		return nil
	}
	return globalCtx.RateLimiter
}

// GetSettingsManager 获取设置管理器
func GetSettingsManager() *config.SettingsManager {
	if globalCtx == nil {
		return nil
	}
	return globalCtx.SettingsMgr
}

// GetGroupManager 获取分组管理器
func GetGroupManager() *auth.GroupManager {
	if globalCtx == nil {
		return nil
	}
	return globalCtx.GroupMgr
}

// GetStatsCollector 获取统计收集器
func GetStatsCollector() *stats.Collector {
	if globalCtx == nil {
		return nil
	}
	return globalCtx.StatsCollector
}
