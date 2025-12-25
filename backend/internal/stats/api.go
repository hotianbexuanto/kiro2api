package stats

import (
	"fmt"
	"net/http"
	"strconv"

	"kiro2api/internal/auth"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册统计 API 路由
func RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/stats", handleGetStats)
	r.GET("/stats/records", handleGetRecords)

	// 请求日志 API（持久化）
	r.GET("/logs", handleQueryLogs)
	r.GET("/logs/stats", handleLogStats)
	r.DELETE("/logs", handleClearLogs)
}

func handleGetStats(c *gin.Context) {
	stats, err := GetStatsFromDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 按权限过滤
	keyConfig := getAPIKeyConfig(c)
	if keyConfig != nil && len(keyConfig.AllowedGroups) > 0 {
		stats.RecentRecords = filterRecordsByGroups(stats.RecentRecords, keyConfig.AllowedGroups)
	}

	c.JSON(http.StatusOK, stats)
}

func handleGetRecords(c *gin.Context) {
	records, err := getRecentRecords(100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	keyConfig := getAPIKeyConfig(c)
	if keyConfig != nil && len(keyConfig.AllowedGroups) > 0 {
		records = filterRecordsByGroups(records, keyConfig.AllowedGroups)
	}
	c.JSON(http.StatusOK, records)
}

// getAPIKeyConfig 从 context 获取 API key 配置
func getAPIKeyConfig(c *gin.Context) *auth.APIKeyConfig {
	v, exists := c.Get("api_key_config")
	if !exists {
		return nil
	}
	keyConfig, ok := v.(*auth.APIKeyConfig)
	if !ok {
		return nil
	}
	return keyConfig
}

// filterRecordsByGroups 按 group 白名单过滤记录
func filterRecordsByGroups(records []RequestRecord, allowedGroups []string) []RequestRecord {
	groupSet := make(map[string]bool, len(allowedGroups))
	for _, g := range allowedGroups {
		groupSet[g] = true
	}

	var filtered []RequestRecord
	for _, r := range records {
		if groupSet[r.Group] {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// === 持久化日志 API ===

// handleQueryLogs 查询持久化日志
func handleQueryLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "100"))

	result, err := queryLogs(queryLogsParams{
		Page:     page,
		PageSize: pageSize,
		Model:    c.Query("model"),
		Group:    c.Query("group"),
		DateFrom: c.Query("date_from"),
		DateTo:   c.Query("date_to"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// handleLogStats 获取日志统计
func handleLogStats(c *gin.Context) {
	stats, err := getLogStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// handleClearLogs 清理日志
func handleClearLogs(c *gin.Context) {
	daysStr := c.Query("days")
	if daysStr == "" {
		// 清空所有
		if err := clearAllLogs(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "所有日志已清空"})
		return
	}

	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 days 参数"})
		return
	}

	deleted, err := clearLogs(days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted, "message": fmt.Sprintf("已删除 %d 天前的日志", days)})
}
