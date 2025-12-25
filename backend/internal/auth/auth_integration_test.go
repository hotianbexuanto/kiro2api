package auth

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// setupTestDB 创建内存数据库并初始化表结构
func setupTestDB(t *testing.T) (*sql.DB, func()) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}

	// 执行 schema 初始化
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		t.Fatalf("初始化数据库表结构失败: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

// createTestAuthService 创建测试用的 AuthService
func createTestAuthService(t *testing.T, db *sql.DB, tokens []Token) *AuthService {
	// 创建 Repository
	repo := NewTokenRepository(db)

	// 插入测试 tokens
	for _, token := range tokens {
		if err := repo.Create(&token); err != nil {
			t.Fatalf("插入测试 token 失败: %v", err)
		}
	}

	// 加载配置
	configs, err := loadConfigsFromDB(repo)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 创建 GroupManager
	groupMgr := NewGroupManager(repo)
	groupMgr.Init(map[string]*GroupConfig{
		"default": {
			Name:        "default",
			DisplayName: "默认分组",
			Settings:    GroupSettings{},
		},
		"pro": {
			Name:        "pro",
			DisplayName: "专业版",
			Settings:    GroupSettings{},
		},
	})

	// 创建 PoolManager
	poolMgr := NewTokenPoolManager(configs, groupMgr, repo)

	// 创建 AuthService
	return &AuthService{
		poolManager: poolMgr,
		repo:        repo,
	}
}

// TestAuthService_GetRepository 测试获取 Repository
func TestAuthService_GetRepository(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	tokens := []Token{
		{
			AuthType:     string(AuthMethodSocial),
			RefreshToken: "test_token_1",
			Disabled:     false,
			GroupName:    "default",
		},
	}

	service := createTestAuthService(t, db, tokens)

	// 测试 GetRepository
	repo := service.GetRepository()
	if repo == nil {
		t.Fatal("GetRepository 返回 nil")
	}

	// 验证 repo 能正常工作
	count, err := repo.CountAll()
	if err != nil {
		t.Fatalf("CountAll 失败: %v", err)
	}
	if count != 1 {
		t.Errorf("期望 1 个 token，实际 %d", count)
	}
}

// TestAuthService_GetPoolManager 测试获取 PoolManager
func TestAuthService_GetPoolManager(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	tokens := []Token{
		{
			AuthType:     string(AuthMethodSocial),
			RefreshToken: "test_token_1",
			Disabled:     false,
			GroupName:    "default",
		},
		{
			AuthType:     string(AuthMethodSocial),
			RefreshToken: "test_token_2",
			Disabled:     false,
			GroupName:    "pro",
		},
	}

	service := createTestAuthService(t, db, tokens)

	// 测试 GetPoolManager
	pm := service.GetPoolManager()
	if pm == nil {
		t.Fatal("GetPoolManager 返回 nil")
	}

	// 验证 PoolManager 能正常工作
	poolStats := pm.GetPoolStats()
	if len(poolStats) != 2 {
		t.Errorf("期望 2 个分组，实际 %d", len(poolStats))
	}

	// 验证分组统计
	if poolStats["default"]["token_count"].(int) != 1 {
		t.Errorf("期望 default 分组有 1 个 token，实际 %d", poolStats["default"]["token_count"])
	}

	if poolStats["pro"]["token_count"].(int) != 1 {
		t.Errorf("期望 pro 分组有 1 个 token，实际 %d", poolStats["pro"]["token_count"])
	}
}

// TestAuthService_GetConfigs 测试获取配置列表
func TestAuthService_GetConfigs(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	tokens := []Token{
		{
			AuthType:     string(AuthMethodSocial),
			RefreshToken: "social_token",
			Disabled:     false,
			GroupName:    "default",
			Name:         "Social测试",
		},
		{
			AuthType:     string(AuthMethodIdC),
			RefreshToken: "idc_token",
			ClientID:     "client_123",
			ClientSecret: "secret_456",
			Disabled:     false,
			GroupName:    "pro",
			Name:         "IdC测试",
		},
	}

	service := createTestAuthService(t, db, tokens)

	// 测试 GetConfigs
	configs := service.GetConfigs()
	if len(configs) != 2 {
		t.Fatalf("期望 2 个配置，实际 %d", len(configs))
	}

	// 验证第一个配置 (Social)
	found := false
	for _, cfg := range configs {
		if cfg.RefreshToken == "social_token" {
			found = true
			if cfg.AuthType != AuthMethodSocial {
				t.Errorf("期望 AuthType 为 %s，实际 %s", AuthMethodSocial, cfg.AuthType)
			}
			if cfg.Group != "default" {
				t.Errorf("期望 Group 为 default，实际 %s", cfg.Group)
			}
			break
		}
	}
	if !found {
		t.Error("未找到 social_token 配置")
	}

	// 验证第二个配置 (IdC)
	found = false
	for _, cfg := range configs {
		if cfg.RefreshToken == "idc_token" {
			found = true
			if cfg.AuthType != AuthMethodIdC {
				t.Errorf("期望 AuthType 为 %s，实际 %s", AuthMethodIdC, cfg.AuthType)
			}
			if cfg.ClientID != "client_123" {
				t.Errorf("期望 ClientID 为 client_123，实际 %s", cfg.ClientID)
			}
			if cfg.ClientSecret != "secret_456" {
				t.Errorf("期望 ClientSecret 为 secret_456，实际 %s", cfg.ClientSecret)
			}
			if cfg.Group != "pro" {
				t.Errorf("期望 Group 为 pro，实际 %s", cfg.Group)
			}
			break
		}
	}
	if !found {
		t.Error("未找到 idc_token 配置")
	}
}
