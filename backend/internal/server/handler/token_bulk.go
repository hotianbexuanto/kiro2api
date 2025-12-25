package handler

import (
	"net/http"

	"kiro2api/internal/auth"

	"github.com/gin-gonic/gin"
)

// AddTokensBulk POST /api/tokens/bulk
func AddTokensBulk(c *gin.Context, authService *auth.AuthService) {
	var req struct {
		Tokens []auth.AuthConfig `json:"tokens"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求体: " + err.Error()})
		return
	}
	if len(req.Tokens) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tokens 不能为空"})
		return
	}

	valid := make([]auth.AuthConfig, 0, len(req.Tokens))
	skipped := make([]gin.H, 0)
	for i, cfg := range req.Tokens {
		if cfg.RefreshToken == "" {
			skipped = append(skipped, gin.H{"index": i, "error": "refreshToken 是必需的"})
			continue
		}
		if cfg.AuthType == "" {
			cfg.AuthType = auth.AuthMethodSocial
		}
		valid = append(valid, cfg)
	}

	if len(valid) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "没有可添加的有效 token", "skipped": skipped})
		return
	}

	inserted, duplicates, err := authService.AddConfigs(valid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加配置失败: " + err.Error(), "skipped": skipped})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"added":      inserted,
		"duplicates": duplicates,
		"skipped":    skipped,
		"message":    "Token 已批量添加",
	})
}
