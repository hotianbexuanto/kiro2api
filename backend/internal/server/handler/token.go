package handler

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"kiro2api/internal/auth"
	"kiro2api/internal/config"

	"github.com/gin-gonic/gin"
)

// ListTokens 处理Token池API请求 - 使用数据库分页
func ListTokens(c *gin.Context, authService *auth.AuthService) {
	// 分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "100"))
	group := c.Query("group")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 1000 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	repo := authService.GetRepository()

	// 获取统计
	total, _ := repo.CountAll()
	active, _ := repo.CountActive()

	// 获取 Token 列表
	var tokens []*auth.Token
	var err error
	if group != "" {
		tokens, err = repo.ListByGroup(group, pageSize, offset)
	} else {
		tokens, err = repo.ListAll(pageSize, offset)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加载失败: " + err.Error()})
		return
	}

	// 获取全局 in-flight 统计
	globalInFlight, tokensWithInFlight := authService.GetGlobalInFlightStats()

	// 转换为响应格式
	tokenList := make([]any, 0, len(tokens))
	for _, t := range tokens {
		tokenData := map[string]any{
			"id":              t.ID,
			"index":           t.ID, // 兼容旧接口
			"user_email":      maskEmail(t.UserEmail),
			"token_preview":   createTokenPreview(t.RefreshToken),
			"auth_type":       strings.ToLower(t.AuthType),
			"remaining_usage": t.AvailableUsage,
			"expires_at":      formatTime(t.AccessTokenExpiresAt),
			"last_used":       formatTime(t.LastUsedAt),
			"last_verified":   formatTime(t.LastVerifiedAt),
			"status":          getStatus(t),
			"group":           t.GroupName,
			"name":            t.Name,
		}

		if t.ErrorMsg != "" {
			tokenData["error"] = t.ErrorMsg
		}

		if t.TotalLimit > 0 {
			tokenData["usage_limits"] = map[string]any{
				"total_limit":   t.TotalLimit,
				"current_usage": t.CurrentUsage,
				"is_exceeded":   t.AvailableUsage <= 0,
			}
		}

		// 添加运行时统计
		if metrics := authService.GetMetricsByTokenID(t.ID); metrics != nil {
			tokenData["request_count"] = metrics.RequestCount
			tokenData["success_count"] = metrics.RequestCount - metrics.FailureCount
			tokenData["failure_count"] = metrics.FailureCount
			tokenData["in_flight"] = metrics.InFlight
			tokenData["avg_latency"] = metrics.AvgLatency
		} else {
			// 没有 metrics 记录，填充默认值
			tokenData["request_count"] = int64(0)
			tokenData["success_count"] = int64(0)
			tokenData["failure_count"] = int64(0)
			tokenData["in_flight"] = int64(0)
			tokenData["avg_latency"] = float64(0)
		}

		tokenList = append(tokenList, tokenData)
	}

	c.JSON(http.StatusOK, gin.H{
		"timestamp":     time.Now().Format(time.RFC3339),
		"total_tokens":  total,
		"active_tokens": active,
		"tokens":        tokenList,
		"page":          page,
		"page_size":     pageSize,
		"pool_stats": map[string]any{
			"total_tokens":          total,
			"active_tokens":         active,
			"global_in_flight":      globalInFlight,
			"tokens_with_in_flight": tokensWithInFlight,
		},
	})
}

// AddToken 添加新 token
func AddToken(c *gin.Context, authService *auth.AuthService) {
	var config auth.AuthConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求体: " + err.Error()})
		return
	}

	if config.RefreshToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refreshToken 是必需的"})
		return
	}
	if config.AuthType == "" {
		config.AuthType = auth.AuthMethodSocial
	}

	// 检查是否已存在
	repo := authService.GetRepository()
	existing, _ := repo.GetByRefreshToken(config.RefreshToken)
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error":     "Token 已存在",
			"duplicate": true,
			"existing": gin.H{
				"id":    existing.ID,
				"name":  existing.Name,
				"group": existing.GroupName,
			},
		})
		return
	}

	if err := authService.AddConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Token 已添加"})
}

// DeleteToken 删除 token
func DeleteToken(c *gin.Context, authService *auth.AuthService) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	if err := authService.RemoveConfig(int(id)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// UpdateToken 更新 token
func UpdateToken(c *gin.Context, authService *auth.AuthService) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	repo := authService.GetRepository()
	token, err := repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Token不存在"})
		return
	}

	var update struct {
		Disabled *bool   `json:"disabled"`
		Group    *string `json:"group"`
		Name     *string `json:"name"`
	}
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求体: " + err.Error()})
		return
	}

	if update.Disabled != nil {
		token.Disabled = *update.Disabled
	}
	if update.Group != nil {
		token.GroupName = *update.Group
	}
	if update.Name != nil {
		token.Name = *update.Name
	}

	if err := repo.Update(token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Token 已更新"})
}

// MoveToken 移动 Token 到其他分组
func MoveToken(c *gin.Context, authService *auth.AuthService) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var req struct {
		Group string `json:"group" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求体: " + err.Error()})
		return
	}

	repo := authService.GetRepository()
	token, err := repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Token不存在"})
		return
	}

	token.GroupName = req.Group
	if err := repo.Update(token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "移动失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Token 已移动"})
}

// RefreshTokens 批量刷新 Token（支持并发）
func RefreshTokens(c *gin.Context, authService *auth.AuthService) {
	var req struct {
		Group string `json:"group"`
		Limit int    `json:"limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许空 body
	}

	// 不限制数量，刷新分组内所有token
	if req.Limit <= 0 {
		req.Limit = 10000
	}

	repo := authService.GetRepository()
	var tokens []*auth.Token
	var err error

	if req.Group != "" {
		tokens, err = repo.ListByGroup(req.Group, req.Limit, 0)
	} else {
		tokens, err = repo.FindOldestUnverified(req.Limit)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取Token失败: " + err.Error()})
		return
	}

	// 获取并发数配置
	settings := config.GetDefaultSettingsManager().Get()
	concurrency := settings.RefreshConcurrency
	if concurrency <= 0 {
		concurrency = 5
	}

	// 结果收集
	type refreshResult struct {
		index  int
		result gin.H
		ok     bool
	}

	results := make([]gin.H, len(tokens))
	resultChan := make(chan refreshResult, len(tokens))

	// 信号量控制并发
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, t := range tokens {
		wg.Add(1)
		go func(idx int, token *auth.Token) {
			defer wg.Done()
			sem <- struct{}{}        // 获取信号量
			defer func() { <-sem }() // 释放信号量

			_, refreshErr := repo.RefreshSingle(token)
			if refreshErr != nil {
				resultChan <- refreshResult{
					index: idx,
					result: gin.H{
						"id":     token.ID,
						"name":   token.Name,
						"status": "error",
						"error":  refreshErr.Error(),
					},
					ok: false,
				}
			} else {
				resultChan <- refreshResult{
					index: idx,
					result: gin.H{
						"id":              token.ID,
						"name":            token.Name,
						"status":          getStatus(token),
						"user_email":      token.UserEmail,
						"remaining_usage": token.AvailableUsage,
					},
					ok: true,
				}
			}
		}(i, t)
	}

	// 等待所有完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集结果
	refreshed := 0
	failed := 0
	for r := range resultChan {
		results[r.index] = r.result
		if r.ok {
			refreshed++
		} else {
			failed++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"refreshed":   refreshed,
		"failed":      failed,
		"total":       len(tokens),
		"concurrency": concurrency,
		"results":     results,
		"message":     "已执行刷新验证",
	})
}

// getStatus 获取状态字符串
func getStatus(t *auth.Token) string {
	if t.Disabled {
		return "disabled"
	}
	if t.Status == string(auth.TokenStatusBanned) {
		return "banned"
	}
	if t.Status == string(auth.TokenStatusExhausted) || t.AvailableUsage <= 0 {
		return "exhausted"
	}
	if t.ErrorMsg != "" {
		return "error"
	}
	return "active"
}

// formatTime 格式化时间
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// createTokenPreview 创建 token 预览
func createTokenPreview(token string) string {
	if len(token) <= 10 {
		return "***"
	}
	return "***" + token[len(token)-10:]
}

// maskEmail 对邮箱进行脱敏处理
func maskEmail(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return email
	}
	username := parts[0]
	domain := parts[1]

	var maskedUsername string
	if len(username) <= 4 {
		maskedUsername = strings.Repeat("*", len(username))
	} else {
		prefix := username[:2]
		suffix := username[len(username)-2:]
		middleLen := len(username) - 4
		maskedUsername = prefix + strings.Repeat("*", middleLen) + suffix
	}

	domainParts := strings.Split(domain, ".")
	var maskedDomain string
	if len(domainParts) == 1 {
		maskedDomain = strings.Repeat("*", len(domain))
	} else if len(domainParts) == 2 {
		maskedDomain = strings.Repeat("*", len(domainParts[0])) + "." + domainParts[1]
	} else {
		maskedParts := make([]string, len(domainParts))
		for i := 0; i < len(domainParts)-2; i++ {
			maskedParts[i] = strings.Repeat("*", len(domainParts[i]))
		}
		maskedParts[len(domainParts)-2] = domainParts[len(domainParts)-2]
		maskedParts[len(domainParts)-1] = domainParts[len(domainParts)-1]
		maskedDomain = strings.Join(maskedParts, ".")
	}

	return maskedUsername + "@" + maskedDomain
}
