package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

// IncrInFlight 增加并发计数
func IncrInFlight(ctx context.Context, tokenID int64) (int64, error) {
	cache := GetDefault()
	if !cache.Enabled() {
		return 0, nil
	}
	key := fmt.Sprintf("%s%d", KeyPrefixInFlight, tokenID)
	return cache.Client().Incr(ctx, key).Result()
}

// DecrInFlight 减少并发计数
func DecrInFlight(ctx context.Context, tokenID int64) (int64, error) {
	cache := GetDefault()
	if !cache.Enabled() {
		return 0, nil
	}
	key := fmt.Sprintf("%s%d", KeyPrefixInFlight, tokenID)
	return cache.Client().Decr(ctx, key).Result()
}

// GetInFlight 获取并发计数
func GetInFlight(ctx context.Context, tokenID int64) (int64, error) {
	cache := GetDefault()
	if !cache.Enabled() {
		return 0, nil
	}
	key := fmt.Sprintf("%s%d", KeyPrefixInFlight, tokenID)
	val, err := cache.Client().Get(ctx, key).Result()
	if err != nil {
		return 0, nil // key 不存在返回 0
	}
	return strconv.ParseInt(val, 10, 64)
}

// GetAllInFlight 获取所有 Token 的并发计数
func GetAllInFlight(ctx context.Context) (map[int64]int64, error) {
	cache := GetDefault()
	if !cache.Enabled() {
		return nil, nil
	}

	// 使用 SCAN 遍历所有 inflight key
	result := make(map[int64]int64)
	pattern := KeyPrefixInFlight + "*"
	iter := cache.Client().Scan(ctx, 0, pattern, 100).Iterator()

	for iter.Next(ctx) {
		key := iter.Val()
		// 解析 token ID
		var tokenID int64
		fmt.Sscanf(key, KeyPrefixInFlight+"%d", &tokenID)

		val, err := cache.Client().Get(ctx, key).Int64()
		if err == nil && val > 0 {
			result[tokenID] = val
		}
	}

	return result, iter.Err()
}

// SetCooldown 设置冷却时间
func SetCooldown(ctx context.Context, tokenID int64, duration time.Duration) error {
	cache := GetDefault()
	if !cache.Enabled() {
		return nil
	}
	key := fmt.Sprintf("%s%d", KeyPrefixCooldown, tokenID)
	return cache.Client().Set(ctx, key, "1", duration).Err()
}

// IsCoolingDown 检查是否在冷却中
func IsCoolingDown(ctx context.Context, tokenID int64) bool {
	cache := GetDefault()
	if !cache.Enabled() {
		return false
	}
	key := fmt.Sprintf("%s%d", KeyPrefixCooldown, tokenID)
	exists, _ := cache.Client().Exists(ctx, key).Result()
	return exists > 0
}
