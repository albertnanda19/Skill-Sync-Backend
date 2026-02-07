package usecase

import (
	"context"
	"time"
)

type SearchCache interface {
	GetJSON(ctx context.Context, key string, out any) (bool, error)
	SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	SetIfNotExists(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
}
