package handler

import (
	"net/http"

	"kiro2api/internal/auth"

	"github.com/gin-gonic/gin"
)

// ListGroups 列出所有分组（含统计）
func ListGroups(c *gin.Context, authService *auth.AuthService) {
	gm := GetGroupManager()
	groups := gm.List()

	// 从数据库获取分组统计
	repo := authService.GetRepository()
	groupStats, err := repo.GetGroupStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取分组统计失败"})
		return
	}

	result := make([]gin.H, 0, len(groups))
	for _, g := range groups {
		stats := groupStats[g.Name]
		result = append(result, gin.H{
			"name":         g.Name,
			"display_name": g.DisplayName,
			"settings":     g.Settings,
			"token_count":  stats.Total,
			"active_count": stats.Active,
		})
	}

	c.JSON(http.StatusOK, gin.H{"groups": result})
}

// CreateGroup 创建分组
func CreateGroup(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		DisplayName string `json:"display_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求体: " + err.Error()})
		return
	}

	gm := GetGroupManager()
	if err := gm.Create(req.Name, req.DisplayName); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "分组已创建"})
}

// UpdateGroup 更新分组设置
func UpdateGroup(c *gin.Context) {
	name := c.Param("name")

	var req struct {
		DisplayName *string             `json:"display_name"`
		Settings    *auth.GroupSettings `json:"settings"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求体: " + err.Error()})
		return
	}

	gm := GetGroupManager()
	if err := gm.Update(name, req.DisplayName, req.Settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "分组已更新"})
}

// RenameGroup 重命名分组
func RenameGroup(c *gin.Context, authService *auth.AuthService) {
	oldName := c.Param("name")

	var req struct {
		NewName string `json:"new_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求体: " + err.Error()})
		return
	}

	// 使用 repository 批量更新（单个事务，同时更新 groups 和 tokens 表）
	repo := authService.GetRepository()
	if err := repo.RenameGroup(oldName, req.NewName); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 更新 GroupManager 内存状态
	gm := GetGroupManager()
	if err := gm.Rename(oldName, req.NewName); err != nil {
		// GroupManager 更新失败不阻塞，记录日志即可
		// 因为数据库已经更新成功，下次重启会从数据库加载正确状态
	}

	// 刷新 poolManager 缓存
	authService.GetPoolManager().UpdateConfigs(authService.GetConfigs())

	c.JSON(http.StatusOK, gin.H{"message": "分组已重命名"})
}

// DeleteGroup 删除分组
func DeleteGroup(c *gin.Context, authService *auth.AuthService) {
	name := c.Param("name")

	// 批量更新：将该分组的所有 Token 移至 default（单条 SQL）
	repo := authService.GetRepository()
	if err := repo.RenameGroup(name, "default"); err != nil {
		// 如果分组不存在也继续（幂等操作）
	}

	// 删除分组配置
	if err := repo.DeleteGroup(name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 更新 GroupManager 内存状态
	gm := GetGroupManager()
	if err := gm.Delete(name); err != nil {
		// GroupManager 更新失败不阻塞
	}

	// 刷新 poolManager 缓存
	authService.GetPoolManager().UpdateConfigs(authService.GetConfigs())

	c.Status(http.StatusNoContent)
}
