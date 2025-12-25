package server

import (
	"net/http"
	"os"
	"time"

	"kiro2api/internal/auth"
	"kiro2api/internal/config"
	"kiro2api/internal/logger"
	"kiro2api/internal/server/handler"
	"kiro2api/internal/service"
	"kiro2api/internal/stats"

	"github.com/gin-gonic/gin"
)

var startTime = time.Now()

// corsMiddleware CORS中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, PATCH, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, x-api-key")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// ========== 新版：依赖注入 ==========

// Start 启动服务器（依赖注入版本）
func Start(port string, authService *auth.AuthService) {
	// 设置 gin 模式
	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "" {
		ginMode = gin.ReleaseMode
	}
	gin.SetMode(ginMode)

	// 创建限流器
	qps := service.DefaultRateLimitQPS
	burst := service.DefaultRateLimitBurst
	rateLimiter := service.NewRateLimiter(qps, burst)

	// 创建设置管理器
	settingsMgr := config.NewSettingsManager(auth.GetDB(), qps, burst)
	if err := settingsMgr.Load(); err != nil {
		logger.Warn("加载设置失败，使用默认值", logger.Err(err))
	} else {
		settings := settingsMgr.Get()
		if settings.RateLimitQPS > 0 && settings.RateLimitBurst > 0 {
			rateLimiter.SetRate(settings.RateLimitQPS, settings.RateLimitBurst)
			qps = settings.RateLimitQPS
			burst = settings.RateLimitBurst
		}
	}

	// 创建分组管理器
	groupMgr := auth.NewGroupManager(auth.NewTokenRepository(auth.GetDB()))

	// 创建统计收集器
	statsCollector := stats.NewCollector(stats.GetLogDB())

	// 初始化 handler context
	handler.InitContext(&handler.Context{
		RateLimiter:    rateLimiter,
		SettingsMgr:    settingsMgr,
		GroupMgr:       groupMgr,
		StatsCollector: statsCollector,
	})

	// 创建 API Key 管理器
	keyMgr := auth.NewAPIKeyManager(auth.GetDB())

	r := gin.New()

	// 添加中间件
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(RequestIDMiddleware())
	r.Use(corsMiddleware())
	r.Use(PathBasedAuthMiddleware(keyMgr, []string{"/v1", "/api/tokens", "/api/groups", "/api/settings", "/api/stats", "/api/keys"}))
	r.Use(rateLimiter.Middleware())
	r.Use(StatsMiddleware())

	// 静态资源
	r.Static("/static", "./static")
	r.Static("/assets", "./static/assets")
	r.StaticFile("/vite.svg", "./static/vite.svg")
	r.GET("/", func(c *gin.Context) {
		c.File("./static/index.html")
	})

	// 健康检查端点（无需认证）
	r.GET("/health", func(c *gin.Context) {
		uptime := time.Since(startTime).Seconds()

		// 检查数据库连接
		dbStatus := "connected"
		if err := auth.GetDB().Ping(); err != nil {
			dbStatus = "disconnected"
		}

		// 获取 token 统计
		repo := authService.GetRepository()
		activeTokens := 0
		groupStats := make(map[string]int)

		if repo != nil {
			activeTokens, _ = repo.CountActive()
			stats, _ := repo.GetGroupStats()
			for group, stat := range stats {
				groupStats[group] = stat.Total
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"status":        "healthy",
			"version":       "1.0.0",
			"uptime":        int64(uptime),
			"timestamp":     time.Now().Format(time.RFC3339),
			"database":      dbStatus,
			"active_tokens": activeTokens,
			"token_pools":   groupStats,
		})
	})

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if len(path) > 4 && (path[:4] == "/api" || path[:3] == "/v1") {
			service.RespondError(c, http.StatusNotFound, "%s", "404 未找到")
			return
		}
		c.File("./static/index.html")
	})

	// API 路由
	registerAPIRoutes(r, authService, keyMgr)

	logger.Info("启动服务器",
		logger.String("port", port),
		logger.Float64("qps", qps),
		logger.Int("burst", burst))

	server := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("启动服务器失败", logger.Err(err))
		os.Exit(1)
	}
}

// registerAPIRoutes 注册 API 路由
func registerAPIRoutes(r *gin.Engine, authService *auth.AuthService, keyMgr *auth.APIKeyManager) {
	// Token 管理
	r.GET("/api/tokens", func(c *gin.Context) { handler.ListTokens(c, authService) })
	r.POST("/api/tokens", func(c *gin.Context) { handler.AddToken(c, authService) })
	r.POST("/api/tokens/bulk", func(c *gin.Context) { handler.AddTokensBulk(c, authService) })
	r.DELETE("/api/tokens/:id", func(c *gin.Context) { handler.DeleteToken(c, authService) })
	r.PATCH("/api/tokens/:id", func(c *gin.Context) { handler.UpdateToken(c, authService) })
	r.PUT("/api/tokens/:id/move", func(c *gin.Context) { handler.MoveToken(c, authService) })

	// 分组管理
	r.GET("/api/groups", func(c *gin.Context) { handler.ListGroups(c, authService) })
	r.POST("/api/groups", handler.CreateGroup)
	r.PUT("/api/groups/:name", handler.UpdateGroup)
	r.POST("/api/groups/:name/rename", func(c *gin.Context) { handler.RenameGroup(c, authService) })
	r.DELETE("/api/groups/:name", func(c *gin.Context) { handler.DeleteGroup(c, authService) })

	// 设置
	r.GET("/api/settings", func(c *gin.Context) { handler.GetSettingsWithAuth(c, authService) })
	r.POST("/api/settings", handler.UpdateSettings)

	// Token 刷新
	r.POST("/api/tokens/refresh", func(c *gin.Context) { handler.RefreshTokens(c, authService) })

	// API Key 管理
	r.GET("/api/keys", func(c *gin.Context) { handler.GetAPIKeys(c, keyMgr) })
	r.POST("/api/keys", func(c *gin.Context) { handler.CreateAPIKey(c, keyMgr) })
	r.PATCH("/api/keys/:key", func(c *gin.Context) { handler.UpdateAPIKey(c, keyMgr) })
	r.DELETE("/api/keys/:key", func(c *gin.Context) { handler.DeleteAPIKey(c, keyMgr) })

	// 统计
	apiGroup := r.Group("/api")
	stats.RegisterRoutes(apiGroup)

	// AI API
	r.GET("/v1/models", handler.HandleModels)
	r.POST("/v1/messages", func(c *gin.Context) { handler.HandleMessages(c, authService, "") })
	r.POST("/v1/messages/count_tokens", handler.HandleCountTokens)
	r.POST("/v1/chat/completions", func(c *gin.Context) { handler.HandleChatCompletions(c, authService, "") })

	// 分组 AI API
	r.POST("/:group/v1/messages", func(c *gin.Context) {
		group := c.Param("group")
		if group == "api" || group == "static" {
			c.Next()
			return
		}
		if !CheckGroupPermission(c, group) {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该分组"})
			return
		}
		handler.HandleMessages(c, authService, group)
	})
	r.POST("/:group/v1/chat/completions", func(c *gin.Context) {
		group := c.Param("group")
		if group == "api" || group == "static" {
			c.Next()
			return
		}
		if !CheckGroupPermission(c, group) {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该分组"})
			return
		}
		handler.HandleChatCompletions(c, authService, group)
	})
	r.GET("/:group/v1/models", func(c *gin.Context) {
		group := c.Param("group")
		if group == "api" || group == "static" {
			c.Next()
			return
		}
		if !CheckGroupPermission(c, group) {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该分组"})
			return
		}
		handler.HandleModels(c)
	})
}
