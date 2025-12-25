package cache

import (
	"context"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"kiro2api/internal/logger"
)

// defaultCache 默认 Redis 缓存实例（用于向后兼容）
var defaultCache *RedisCache

// ========== 新版：依赖注入 ==========

// RedisCache Redis 缓存（依赖注入版本）
type RedisCache struct {
	client  *redis.Client
	enabled bool
}

// NewRedis 创建 Redis 缓存
func NewRedis(url string) (*RedisCache, error) {
	if url == "" {
		return &RedisCache{enabled: false}, nil
	}

	opt, err := redis.ParseURL(url)
	if err != nil {
		opt = &redis.Options{
			Addr:         url,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
			PoolSize:     20,
			MinIdleConns: 5,
		}
	}

	c := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	logger.Info("Redis 连接成功", logger.String("addr", opt.Addr))
	return &RedisCache{client: c, enabled: true}, nil
}

// Enabled 是否启用
func (r *RedisCache) Enabled() bool {
	return r.enabled
}

// Client 获取客户端
func (r *RedisCache) Client() *redis.Client {
	return r.client
}

// Close 关闭连接
func (r *RedisCache) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// InitDefault 初始化默认 Redis 缓存（向后兼容）
func InitDefault() {
	url := os.Getenv("REDIS_URL")
	cache, _ := NewRedis(url)
	defaultCache = cache
}

// GetDefault 获取默认 Redis 缓存
func GetDefault() *RedisCache {
	if defaultCache == nil {
		InitDefault()
	}
	return defaultCache
}
