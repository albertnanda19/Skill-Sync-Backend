package cache

import (
	"context"
	"time"

	infracache "skill-sync/internal/infrastructure/cache"
)

type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

type LockingCache interface {
	Cache
	SetIfNotExists(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	DeleteByPattern(ctx context.Context, pattern string) error
}

type RedisClient struct {
	r *infracache.Redis
}

func NewRedisClient(r *infracache.Redis) *RedisClient {
	return &RedisClient{r: r}
}

func (c *RedisClient) Get(ctx context.Context, key string) (string, error) {
	if c == nil || c.r == nil {
		return "", nil
	}
	v, _, err := c.r.GetString(ctx, key)
	return v, err
}

func (c *RedisClient) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	if c == nil || c.r == nil {
		return nil
	}
	return c.r.SetString(ctx, key, value, ttl)
}

func (c *RedisClient) Delete(ctx context.Context, key string) error {
	if c == nil || c.r == nil {
		return nil
	}
	return c.r.Delete(ctx, key)
}

func (c *RedisClient) SetIfNotExists(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	if c == nil || c.r == nil {
		return false, nil
	}
	return c.r.SetIfNotExists(ctx, key, value, ttl)
}

func (c *RedisClient) DeleteByPattern(ctx context.Context, pattern string) error {
	if c == nil || c.r == nil {
		return nil
	}
	return c.r.DeleteByPattern(ctx, pattern)
}
