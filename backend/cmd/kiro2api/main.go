package main

import (
	"fmt"
	"os"
	"path/filepath"

	"kiro2api/internal/auth"
	"kiro2api/internal/logger"
	"kiro2api/internal/server"
	"kiro2api/internal/stats"

	"github.com/joho/godotenv"
)

func main() {
	// 加载 .env 文件（可选，文件不存在时静默忽略）
	if err := godotenv.Load(); err != nil {
		// .env 不存在是正常情况，不报错
	}

	// 初始化日志
	logger.Info("kiro2api 启动中...")

	// 确定端口
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// 初始化请求日志数据库
	logDBPath := os.Getenv("KIRO_LOG_DB_PATH")
	if logDBPath == "" {
		dbPath := os.Getenv("KIRO_DB_PATH")
		if dbPath == "" {
			dbPath = "./data/kiro2api.db"
		}
		logDBPath = filepath.Join(filepath.Dir(dbPath), "request_logs.db")
	}
	if err := stats.InitLogDB(logDBPath); err != nil {
		logger.Warn("初始化请求日志数据库失败", logger.Err(err))
	}

	// 创建认证服务
	authService, err := auth.NewAuthService()
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建认证服务失败: %v\n", err)
		os.Exit(1)
	}

	// 启动 HTTP 服务器
	logger.Info("启动服务器", logger.String("port", port))
	server.Start(port, authService)
}
