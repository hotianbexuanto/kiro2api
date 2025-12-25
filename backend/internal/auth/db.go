package auth

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"kiro2api/internal/logger"

	_ "modernc.org/sqlite"
)

var (
	globalDB   *sql.DB
	dbOnce     sync.Once
	dbInitErr  error
	dbPath     string
)

const schema = `
CREATE TABLE IF NOT EXISTS tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    auth_type TEXT NOT NULL DEFAULT 'Social',
    refresh_token TEXT NOT NULL UNIQUE,
    client_id TEXT,
    client_secret TEXT,
    disabled INTEGER DEFAULT 0,
    group_name TEXT DEFAULT 'default',
    name TEXT,
    status TEXT DEFAULT '',

    -- 缓存字段
    user_email TEXT,
    access_token TEXT,
    access_token_expires_at DATETIME,
    available_usage REAL DEFAULT 0,
    base_usage REAL DEFAULT 0,
    free_trial_usage REAL DEFAULT 0,
    total_limit REAL DEFAULT 0,
    current_usage REAL DEFAULT 0,
    last_verified_at DATETIME,
    last_used_at DATETIME,
    error_msg TEXT,

    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tokens_group ON tokens(group_name);
CREATE INDEX IF NOT EXISTS idx_tokens_status ON tokens(status);
CREATE INDEX IF NOT EXISTS idx_tokens_disabled ON tokens(disabled);
CREATE INDEX IF NOT EXISTS idx_tokens_last_verified ON tokens(last_verified_at);

CREATE TABLE IF NOT EXISTS groups (
    name TEXT PRIMARY KEY,
    display_name TEXT,
    priority INTEGER DEFAULT 0,
    rate_limit_qps REAL DEFAULT 0,
    rate_limit_burst INTEGER DEFAULT 0,
    cooldown_sec INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS api_keys (
    key TEXT PRIMARY KEY,
    name TEXT,
    allowed_groups TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS migrations (
    version INTEGER PRIMARY KEY,
    applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

// InitDB 初始化数据库连接
func InitDB(path string) error {
	dbOnce.Do(func() {
		dbPath = path
		dbInitErr = initDatabase()
	})
	return dbInitErr
}

func initDatabase() error {
	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建数据库目录失败: %w", err)
	}

	// 打开数据库连接
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("数据库连接失败: %w", err)
	}

	// 设置连接池（WAL 模式支持 1写+多读并发）
	db.SetMaxOpenConns(5)                        // 允许 5 个并发连接
	db.SetMaxIdleConns(2)                        // 保持 2 个空闲连接
	db.SetConnMaxLifetime(time.Hour)             // 连接最大生命周期

	// 执行 schema
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return fmt.Errorf("初始化数据库表失败: %w", err)
	}

	globalDB = db
	logger.Info("SQLite数据库初始化完成", logger.String("path", dbPath))
	return nil
}

// GetDB 获取数据库连接
func GetDB() *sql.DB {
	return globalDB
}

// CloseDB 关闭数据库连接
func CloseDB() error {
	if globalDB != nil {
		return globalDB.Close()
	}
	return nil
}

// GetDBPath 获取数据库路径
func GetDBPath() string {
	return dbPath
}

// IsDBInitialized 检查数据库是否已初始化
func IsDBInitialized() bool {
	return globalDB != nil
}
