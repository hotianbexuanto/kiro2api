package auth

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"kiro2api/internal/config"
	"kiro2api/internal/logger"
	"kiro2api/internal/types"
)

// Token 数据库模型
type Token struct {
	ID                    int64
	AuthType              string
	RefreshToken          string
	ClientID              string
	ClientSecret          string
	Disabled              bool
	GroupName             string
	Name                  string
	Status                string
	UserEmail             string
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	AvailableUsage        float64
	BaseUsage             float64
	FreeTrialUsage        float64
	TotalLimit            float64
	CurrentUsage          float64
	LastVerifiedAt        time.Time
	LastUsedAt            time.Time
	ErrorMsg              string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// TokenRepository Token 仓库
type TokenRepository struct {
	db            *sql.DB
	mu            sync.RWMutex
	onTokenUpdate func(tokenID int64) // 回调：Token更新后通知poolManager刷新缓存
}

// NewTokenRepository 创建 Token 仓库（依赖注入版本）
func NewTokenRepository(db *sql.DB) *TokenRepository {
	return &TokenRepository{db: db}
}

// Create 创建 Token
func (r *TokenRepository) Create(t *Token) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	query := `
		INSERT INTO tokens (auth_type, refresh_token, client_id, client_secret, disabled, group_name, name, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`
	result, err := r.db.Exec(query, t.AuthType, t.RefreshToken, t.ClientID, t.ClientSecret, t.Disabled, t.GroupName, t.Name, t.Status)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("token已存在")
		}
		return err
	}

	id, _ := result.LastInsertId()
	t.ID = id
	return nil
}

// Update 更新 Token
func (r *TokenRepository) Update(t *Token) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	query := `
		UPDATE tokens SET
			auth_type = ?, client_id = ?, client_secret = ?, disabled = ?,
			group_name = ?, name = ?, status = ?, user_email = ?, access_token = ?,
			access_token_expires_at = ?, available_usage = ?, base_usage = ?, free_trial_usage = ?,
			total_limit = ?, current_usage = ?, last_verified_at = ?, last_used_at = ?, error_msg = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	_, err := r.db.Exec(query,
		t.AuthType, t.ClientID, t.ClientSecret, t.Disabled,
		t.GroupName, t.Name, t.Status, t.UserEmail, t.AccessToken,
		t.AccessTokenExpiresAt, t.AvailableUsage, t.BaseUsage, t.FreeTrialUsage,
		t.TotalLimit, t.CurrentUsage, t.LastVerifiedAt, t.LastUsedAt, t.ErrorMsg,
		t.ID,
	)
	return err
}

// UpdateTokenStatus 只更新 Token 状态和分组
func (r *TokenRepository) UpdateTokenStatus(id int64, status string, groupName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.db.Exec(`UPDATE tokens SET status = ?, group_name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, groupName, id)
	return err
}

// Delete 删除 Token
func (r *TokenRepository) Delete(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.db.Exec("DELETE FROM tokens WHERE id = ?", id)
	return err
}

// GetByID 根据 ID 获取 Token
func (r *TokenRepository) GetByID(id int64) (*Token, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.scanToken(r.db.QueryRow(`SELECT * FROM tokens WHERE id = ?`, id))
}

// GetByRefreshToken 根据 refresh_token 获取 Token
func (r *TokenRepository) GetByRefreshToken(refreshToken string) (*Token, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.scanToken(r.db.QueryRow(`SELECT * FROM tokens WHERE refresh_token = ?`, refreshToken))
}

// ListByGroup 分页获取分组内的 Token
func (r *TokenRepository) ListByGroup(group string, limit, offset int) ([]*Token, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `SELECT * FROM tokens WHERE group_name = ? ORDER BY id LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, group, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTokens(rows)
}

// ListAll 分页获取所有 Token
func (r *TokenRepository) ListAll(limit, offset int) ([]*Token, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `SELECT * FROM tokens ORDER BY id LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTokens(rows)
}

// CountByGroup 统计分组内的 Token 数量
func (r *TokenRepository) CountByGroup(group string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM tokens WHERE group_name = ?`, group).Scan(&count)
	return count, err
}

// CountAll 统计所有 Token 数量
func (r *TokenRepository) CountAll() (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM tokens`).Scan(&count)
	return count, err
}

// CountActive 统计活跃 Token 数量
func (r *TokenRepository) CountActive() (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM tokens WHERE disabled = 0 AND status = ''`).Scan(&count)
	return count, err
}

// GetActiveByGroup 获取分组内可用的 Token（用于选择）
func (r *TokenRepository) GetActiveByGroup(group string) ([]*Token, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT * FROM tokens
		WHERE group_name = ? AND disabled = 0 AND status = ''
		ORDER BY last_used_at ASC NULLS FIRST
	`
	rows, err := r.db.Query(query, group)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTokens(rows)
}

// FindOldestUnverified 找出最久未验证的 Token
func (r *TokenRepository) FindOldestUnverified(limit int) ([]*Token, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT * FROM tokens
		WHERE disabled = 0 AND status = ''
		ORDER BY last_verified_at ASC NULLS FIRST
		LIMIT ?
	`
	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTokens(rows)
}

// GetValidToken 获取有效 Token（懒加载刷新）
func (r *TokenRepository) GetValidToken(id int64) (*Token, error) {
	token, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}

	// 缓存有效期检查
	if time.Since(token.LastVerifiedAt) < config.TokenCacheTTL {
		return token, nil
	}

	// 过期则刷新
	return r.RefreshSingle(token)
}

// RefreshSingle 刷新单个 Token
func (r *TokenRepository) RefreshSingle(t *Token) (*Token, error) {
	var tokenInfo types.TokenInfo
	var refreshErr error

	switch t.AuthType {
	case AuthMethodSocial:
		tokenInfo, refreshErr = RefreshSocialToken(t.RefreshToken)
	case AuthMethodIdC:
		cfg := AuthConfig{
			AuthType:     t.AuthType,
			RefreshToken: t.RefreshToken,
			ClientID:     t.ClientID,
			ClientSecret: t.ClientSecret,
		}
		tokenInfo, refreshErr = RefreshIdCToken(cfg)
	default:
		t.Status = string(TokenStatusBanned)
		t.ErrorMsg = "不支持的认证类型"
		r.Update(t)
		return t, fmt.Errorf("不支持的认证类型: %s", t.AuthType)
	}

	if refreshErr != nil {
		t.ErrorMsg = refreshErr.Error()
		t.LastVerifiedAt = time.Now()

		// 检测封禁
		errMsg := refreshErr.Error()
		if strings.Contains(errMsg, "Bad credentials") ||
			strings.Contains(errMsg, "TEMPORARILY_SUSPENDED") ||
			strings.Contains(errMsg, "suspended") {
			t.Status = string(TokenStatusBanned)
			t.GroupName = "banned"
		}

		r.Update(t)
		return t, refreshErr
	}

	// 刷新成功，获取使用限制
	checker := NewUsageLimitsChecker()
	usage, checkErr := checker.CheckUsageLimits(tokenInfo)

	t.AccessToken = tokenInfo.AccessToken
	t.AccessTokenExpiresAt = tokenInfo.ExpiresAt
	t.LastVerifiedAt = time.Now()
	t.ErrorMsg = ""

	if checkErr == nil && usage != nil {
		t.AvailableUsage = CalculateAvailableCount(usage)
		if usage.UserInfo.Email != "" {
			t.UserEmail = usage.UserInfo.Email
		}

		// 提取使用限制信息
		for _, breakdown := range usage.UsageBreakdownList {
			if breakdown.ResourceType == "CREDIT" {
				// base 额度
				t.BaseUsage = breakdown.UsageLimitWithPrecision - breakdown.CurrentUsageWithPrecision
				t.TotalLimit = breakdown.UsageLimitWithPrecision
				t.CurrentUsage = breakdown.CurrentUsageWithPrecision

				// free_trial 额度
				if breakdown.FreeTrialInfo != nil && breakdown.FreeTrialInfo.FreeTrialStatus == "ACTIVE" {
					t.FreeTrialUsage = breakdown.FreeTrialInfo.UsageLimitWithPrecision - breakdown.FreeTrialInfo.CurrentUsageWithPrecision
					t.TotalLimit += breakdown.FreeTrialInfo.UsageLimitWithPrecision
					t.CurrentUsage += breakdown.FreeTrialInfo.CurrentUsageWithPrecision
				}
				break
			}
		}

		// 检测配额耗尽
		if t.AvailableUsage <= 0 {
			t.Status = string(TokenStatusExhausted)
			t.GroupName = "exhausted"
		} else if t.Status == string(TokenStatusExhausted) || t.Status == string(TokenStatusBanned) {
			// 恢复
			t.Status = string(TokenStatusActive)
			if t.GroupName == "exhausted" || t.GroupName == "banned" {
				t.GroupName = GetDefaultGroup()
			}
		}
	}

	if err := r.Update(t); err != nil {
		return t, err
	}

	// 触发缓存同步回调
	if r.onTokenUpdate != nil {
		r.onTokenUpdate(t.ID)
	}

	return t, nil
}

// MarkUsed 标记 Token 被使用
func (r *TokenRepository) MarkUsed(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.db.Exec(`UPDATE tokens SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

// BulkInsert 批量插入 Token，返回 (inserted, duplicates, error)
func (r *TokenRepository) BulkInsert(tokens []*Token) (int, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tx, err := r.db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO tokens (auth_type, refresh_token, client_id, client_secret, disabled, group_name, name, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, 0, err
	}
	defer stmt.Close()

	inserted := 0
	duplicates := 0
	for _, t := range tokens {
		result, err := stmt.Exec(t.AuthType, t.RefreshToken, t.ClientID, t.ClientSecret, t.Disabled, t.GroupName, t.Name, t.Status)
		if err != nil {
			logger.Warn("批量插入跳过", logger.Err(err))
			continue
		}
		affected, _ := result.RowsAffected()
		if affected > 0 {
			inserted++
		} else {
			duplicates++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}

	return inserted, duplicates, nil
}

// GetAllGroups 从数据库加载所有分组配置
func (r *TokenRepository) GetAllGroups() (map[string]*GroupConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `SELECT name, display_name, priority, rate_limit_qps, rate_limit_burst, cooldown_sec FROM groups`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := make(map[string]*GroupConfig)
	for rows.Next() {
		var name string
		var displayName sql.NullString
		var priority int
		var rateLimitQPS float64
		var rateLimitBurst, cooldownSec int

		if err := rows.Scan(&name, &displayName, &priority, &rateLimitQPS, &rateLimitBurst, &cooldownSec); err != nil {
			continue
		}

		groups[name] = &GroupConfig{
			Name:        name,
			DisplayName: displayName.String,
			Settings: GroupSettings{
				Priority:       priority,
				RateLimitQPS:   rateLimitQPS,
				RateLimitBurst: rateLimitBurst,
				CooldownSec:    cooldownSec,
			},
		}
	}
	return groups, nil
}

// CreateGroup 创建分组到数据库
func (r *TokenRepository) CreateGroup(g *GroupConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.db.Exec(`INSERT INTO groups (name, display_name, priority, rate_limit_qps, rate_limit_burst, cooldown_sec) VALUES (?, ?, ?, ?, ?, ?)`,
		g.Name, g.DisplayName, g.Settings.Priority, g.Settings.RateLimitQPS, g.Settings.RateLimitBurst, g.Settings.CooldownSec)
	return err
}

// UpdateGroup 更新分组
func (r *TokenRepository) UpdateGroup(g *GroupConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.db.Exec(`UPDATE groups SET display_name = ?, priority = ?, rate_limit_qps = ?, rate_limit_burst = ?, cooldown_sec = ? WHERE name = ?`,
		g.DisplayName, g.Settings.Priority, g.Settings.RateLimitQPS, g.Settings.RateLimitBurst, g.Settings.CooldownSec, g.Name)
	return err
}

// DeleteGroup 删除分组
func (r *TokenRepository) DeleteGroup(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.db.Exec(`DELETE FROM groups WHERE name = ?`, name)
	return err
}

// RenameGroup 重命名分组（同时更新 tokens 表）
func (r *TokenRepository) RenameGroup(oldName, newName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 更新 groups 表
	if _, err := tx.Exec(`UPDATE groups SET name = ? WHERE name = ?`, newName, oldName); err != nil {
		return err
	}

	// 更新 tokens 表中的 group_name
	if _, err := tx.Exec(`UPDATE tokens SET group_name = ? WHERE group_name = ?`, newName, oldName); err != nil {
		return err
	}

	return tx.Commit()
}

// GetGroupStats 获取分组统计
func (r *TokenRepository) GetGroupStats() (map[string]struct{ Total, Active int }, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := `
		SELECT group_name,
			COUNT(*) as total,
			SUM(CASE WHEN disabled = 0 AND status = '' THEN 1 ELSE 0 END) as active
		FROM tokens
		GROUP BY group_name
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]struct{ Total, Active int })
	for rows.Next() {
		var group string
		var total, active int
		if err := rows.Scan(&group, &total, &active); err != nil {
			continue
		}
		stats[group] = struct{ Total, Active int }{total, active}
	}
	return stats, nil
}

// scanToken 扫描单行到 Token
func (r *TokenRepository) scanToken(row *sql.Row) (*Token, error) {
	t := &Token{}
	var accessTokenExpiresAt, lastVerifiedAt, lastUsedAt, createdAt, updatedAt sql.NullTime
	var clientID, clientSecret, name, userEmail, accessToken, errorMsg sql.NullString

	err := row.Scan(
		&t.ID, &t.AuthType, &t.RefreshToken, &clientID, &clientSecret,
		&t.Disabled, &t.GroupName, &name, &t.Status,
		&userEmail, &accessToken, &accessTokenExpiresAt,
		&t.AvailableUsage, &t.TotalLimit, &t.CurrentUsage,
		&lastVerifiedAt, &lastUsedAt, &errorMsg,
		&createdAt, &updatedAt,
		&t.BaseUsage, &t.FreeTrialUsage,
	)
	if err != nil {
		return nil, err
	}

	t.ClientID = clientID.String
	t.ClientSecret = clientSecret.String
	t.Name = name.String
	t.UserEmail = userEmail.String
	t.AccessToken = accessToken.String
	t.ErrorMsg = errorMsg.String
	t.AccessTokenExpiresAt = accessTokenExpiresAt.Time
	t.LastVerifiedAt = lastVerifiedAt.Time
	t.LastUsedAt = lastUsedAt.Time
	t.CreatedAt = createdAt.Time
	t.UpdatedAt = updatedAt.Time

	return t, nil
}

// scanTokens 扫描多行到 Token 列表
func (r *TokenRepository) scanTokens(rows *sql.Rows) ([]*Token, error) {
	var tokens []*Token
	for rows.Next() {
		t := &Token{}
		var accessTokenExpiresAt, lastVerifiedAt, lastUsedAt, createdAt, updatedAt sql.NullTime
		var clientID, clientSecret, name, userEmail, accessToken, errorMsg sql.NullString

		err := rows.Scan(
			&t.ID, &t.AuthType, &t.RefreshToken, &clientID, &clientSecret,
			&t.Disabled, &t.GroupName, &name, &t.Status,
			&userEmail, &accessToken, &accessTokenExpiresAt,
			&t.AvailableUsage, &t.TotalLimit, &t.CurrentUsage,
			&lastVerifiedAt, &lastUsedAt, &errorMsg,
			&createdAt, &updatedAt,
			&t.BaseUsage, &t.FreeTrialUsage,
		)
		if err != nil {
			return nil, err
		}

		t.ClientID = clientID.String
		t.ClientSecret = clientSecret.String
		t.Name = name.String
		t.UserEmail = userEmail.String
		t.AccessToken = accessToken.String
		t.ErrorMsg = errorMsg.String
		t.AccessTokenExpiresAt = accessTokenExpiresAt.Time
		t.LastVerifiedAt = lastVerifiedAt.Time
		t.LastUsedAt = lastUsedAt.Time
		t.CreatedAt = createdAt.Time
		t.UpdatedAt = updatedAt.Time

		tokens = append(tokens, t)
	}
	return tokens, nil
}

// ToAuthConfig 转换为 AuthConfig（兼容现有代码）
func (t *Token) ToAuthConfig() AuthConfig {
	return AuthConfig{
		AuthType:     t.AuthType,
		RefreshToken: t.RefreshToken,
		ClientID:     t.ClientID,
		ClientSecret: t.ClientSecret,
		Disabled:     t.Disabled,
		Group:        t.GroupName,
		Name:         t.Name,
		Status:       TokenStatus(t.Status),
		TokenID:      t.ID,
	}
}

// FromAuthConfig 从 AuthConfig 创建 Token
func FromAuthConfig(cfg AuthConfig) *Token {
	return &Token{
		AuthType:     cfg.AuthType,
		RefreshToken: cfg.RefreshToken,
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Disabled:     cfg.Disabled,
		GroupName:    cfg.Group,
		Name:         cfg.Name,
		Status:       string(cfg.Status),
	}
}

// FixOrphanedExhaustedTokens 修复孤立的耗尽 Token
// 找出 available_usage <= 0 但 status/group 未正确设置的 Token 并修复
func (r *TokenRepository) FixOrphanedExhaustedTokens() (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 找出需要修复的 Token：
	// 1. available_usage <= 0（已耗尽）
	// 2. 状态不是 exhausted 或 banned
	// 3. 分组不是 exhausted 或 banned
	// 4. 未禁用
	// 5. 已经被验证过（last_verified_at 不为空，说明有真实数据）
	query := `
		UPDATE tokens
		SET status = 'exhausted', group_name = 'exhausted', updated_at = CURRENT_TIMESTAMP
		WHERE available_usage <= 0
		  AND available_usage IS NOT NULL
		  AND last_verified_at IS NOT NULL
		  AND status NOT IN ('exhausted', 'banned')
		  AND group_name NOT IN ('exhausted', 'banned')
		  AND disabled = 0
	`
	result, err := r.db.Exec(query)
	if err != nil {
		return 0, err
	}

	affected, _ := result.RowsAffected()
	return int(affected), nil
}
