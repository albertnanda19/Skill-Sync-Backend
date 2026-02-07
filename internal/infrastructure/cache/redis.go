package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client *redis.Client
	logger *log.Logger

	warnedUnavailable atomic.Bool
}

func NewRedis(logger *log.Logger) *Redis {
	host := strings.TrimSpace(os.Getenv("REDIS_HOST"))
	if host == "" {
		host = "localhost"
	}
	port := strings.TrimSpace(os.Getenv("REDIS_PORT"))
	if port == "" {
		port = "6379"
	}
	pass := strings.TrimSpace(os.Getenv("REDIS_PASSWORD"))

	addr := fmt.Sprintf("%s:%s", host, port)
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: pass,
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		if logger != nil {
			logger.Printf("[Cache] Redis unavailable, bypassing cache: %v", err)
		}
		_ = client.Close()
		return &Redis{client: nil, logger: logger}
	}

	return &Redis{client: client, logger: logger}
}

func (r *Redis) isUnavailable() bool {
	return r == nil || r.client == nil
}

func (r *Redis) warnUnavailableOnce(err error) {
	if r == nil {
		return
	}
	if r.logger == nil {
		return
	}
	if r.warnedUnavailable.CompareAndSwap(false, true) {
		if err != nil {
			r.logger.Printf("[Cache] Redis unavailable, bypassing cache: %v", err)
			return
		}
		r.logger.Printf("[Cache] Redis unavailable, bypassing cache")
	}
}

func (r *Redis) GetJSON(ctx context.Context, key string, out any) (bool, error) {
	if r.isUnavailable() {
		return false, nil
	}
	b, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		r.warnUnavailableOnce(err)
		return false, err
	}
	if len(b) == 0 {
		return false, nil
	}
	if err := json.Unmarshal(b, out); err != nil {
		return false, err
	}
	return true, nil
}

func (r *Redis) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	if r.isUnavailable() {
		return nil
	}
	if ttl <= 0 {
		ttl = DefaultTTLFromEnv()
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := r.client.Set(ctx, key, b, ttl).Err(); err != nil {
		r.warnUnavailableOnce(err)
		return err
	}
	return nil
}

func (r *Redis) Delete(ctx context.Context, key string) error {
	if r.isUnavailable() {
		return nil
	}
	if err := r.client.Del(ctx, key).Err(); err != nil {
		r.warnUnavailableOnce(err)
		return err
	}
	return nil
}

func (r *Redis) SetIfNotExists(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	if r.isUnavailable() {
		return false, nil
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	ok, err := r.client.SetNX(ctx, key, value, ttl).Result()
	if err != nil {
		r.warnUnavailableOnce(err)
		return false, err
	}
	return ok, nil
}

func DefaultTTLFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv("REDIS_TTL"))
	if raw == "" {
		return 600 * time.Second
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return 600 * time.Second
	}
	return time.Duration(v) * time.Second
}
