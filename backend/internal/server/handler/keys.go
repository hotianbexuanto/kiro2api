package handler

import (
	"net/http"

	"kiro2api/internal/auth"
	"kiro2api/internal/utils"

	"github.com/gin-gonic/gin"
)

// GetAPIKeys GET /api/keys
func GetAPIKeys(c *gin.Context, keyManager *auth.APIKeyManager) {
	keys := keyManager.GetAll()
	// 隐藏实际 key 值，只返回前4位
	result := make([]gin.H, len(keys))
	for i, k := range keys {
		maskedKey := k.Key
		if len(maskedKey) > 4 {
			maskedKey = maskedKey[:4] + "****"
		}
		result[i] = gin.H{
			"key":            k.Key, // 完整 key 用于标识
			"masked_key":     maskedKey,
			"name":           k.Name,
			"allowed_groups": k.AllowedGroups,
		}
	}
	c.JSON(http.StatusOK, result)
}

// UpdateAPIKey PATCH /api/keys/:key
func UpdateAPIKey(c *gin.Context, keyManager *auth.APIKeyManager) {
	key := c.Param("key")
	var req struct {
		AllowedGroups []string `json:"allowed_groups"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求格式"})
		return
	}

	if !keyManager.UpdateAllowedGroups(key, req.AllowedGroups) {
		c.JSON(http.StatusNotFound, gin.H{"error": "API Key 不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// CreateAPIKey POST /api/keys
func CreateAPIKey(c *gin.Context, keyManager *auth.APIKeyManager) {
	var req struct {
		Key           string   `json:"key"`
		Name          string   `json:"name"`
		AllowedGroups []string `json:"allowed_groups"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求格式"})
		return
	}

	key := req.Key
	if key == "" {
		key = "k2a_" + utils.GenerateUUID()
	}

	if keyManager.Get(key) != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "API Key 已存在"})
		return
	}

	keyManager.AddKey(auth.APIKeyConfig{
		Key:           key,
		Name:          req.Name,
		AllowedGroups: req.AllowedGroups,
	})

	maskedKey := key
	if len(maskedKey) > 4 {
		maskedKey = maskedKey[:4] + "****"
	}
	c.JSON(http.StatusCreated, gin.H{
		"key":            key,
		"masked_key":     maskedKey,
		"name":           req.Name,
		"allowed_groups": req.AllowedGroups,
	})
}

// DeleteAPIKey DELETE /api/keys/:key
func DeleteAPIKey(c *gin.Context, keyManager *auth.APIKeyManager) {
	key := c.Param("key")

	if keyManager.Get(key) == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "API Key 不存在"})
		return
	}

	all := keyManager.GetAll()
	if len(all) <= 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少需要保留一个 API Key"})
		return
	}

	if !keyManager.DeleteKey(key) {
		c.JSON(http.StatusNotFound, gin.H{"error": "API Key 不存在"})
		return
	}

	c.Status(http.StatusNoContent)
}
