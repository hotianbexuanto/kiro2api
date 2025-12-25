package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

// Token 缓存 Key 前缀
const (
	KeyPrefixToken    = "kiro:token:"    // Token 信息 Hash
	KeyPrefixInFlight = "kiro:inflight:" // 并发计数
	KeyPrefixCooldown = "kiro:cooldown:" // 冷却时间
	KeyTokenList      = "kiro:tokens"    // Token ID 列表
)

// TokenCache Token 缓存数据
type TokenCache struct {
	AccessToken string    `redis:"access_token"`
	ExpiresAt   time.Time `redis:"expires_at"`
	Available   int64     `redis:"available"`
	TotalLimit  float64   `redis:"total_limit"`
	UserEmail   string    `redis:"user_email"`
	GroupName   string    `redis:"group_name"`
	Disabled    bool      `redis:"disabled"`
	Status      string    `redis:"status"`
}

// SetToken 缓存 Token 信息
func SetToken(ctx context.Context, tokenID int64, data *TokenCache, ttl time.Duration) error {
	cache := GetDefault()
	if !cache.Enabled() {
		return nil
	}
	key := fmt.Sprintf("%s%d", KeyPrefixToken, tokenID)
	return cache.Client().HSet(ctx, key, map[string]interface{}{
		"access_token": data.AccessToken,
		"expires_at":   data.ExpiresAt.Unix(),
		"available":    data.Available,
		"total_limit":  data.TotalLimit,
		"user_email":   data.UserEmail,
		"group_name":   data.GroupName,
		"disabled":     data.Disabled,
		"status":       data.Status,
	}).Err()
}

// GetToken 获取 Token 缓存
func GetToken(ctx context.Context, tokenID int64) (*TokenCache, error) {
	cache := GetDefault()
	if !cache.Enabled() {
		return nil, nil
	}
	key := fmt.Sprintf("%s%d", KeyPrefixToken, tokenID)
	result, err := cache.Client().HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, nil
	}

	expiresAt, _ := strconv.ParseInt(result["expires_at"], 10, 64)
	available, _ := strconv.ParseInt(result["available"], 10, 64)
	totalLimit, _ := strconv.ParseFloat(result["total_limit"], 64)
	disabled := result["disabled"] == "1" || result["disabled"] == "true"

	return &TokenCache{
		AccessToken: result["access_token"],
		ExpiresAt:   time.Unix(expiresAt, 0),
		Available:   available,
		TotalLimit:  totalLimit,
		UserEmail:   result["user_email"],
		GroupName:   result["group_name"],
		Disabled:    disabled,
		Status:      result["status"],
	}, nil
}
