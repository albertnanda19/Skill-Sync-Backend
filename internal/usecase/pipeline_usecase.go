package usecase

import (
	"context"
	"time"

	"skill-sync/internal/domain"
	"skill-sync/internal/infrastructure/cache"
	"skill-sync/internal/repository"
)

type PipelineUsecase interface {
	GetStatus(ctx context.Context) (*domain.PipelineStatus, error)
}

type Pipeline struct {
	repo  repository.PipelineRepository
	db    interface{ Ping(ctx context.Context) error }
	redis interface{ Ping(ctx context.Context) error }
	now   func() time.Time
}

func NewPipelineUsecase(repo repository.PipelineRepository, db interface{ Ping(ctx context.Context) error }, redis *cache.Redis) *Pipeline {
	var redisPing interface{ Ping(ctx context.Context) error }
	if redis != nil {
		redisPing = redis
	}
	return &Pipeline{repo: repo, db: db, redis: redisPing, now: time.Now}
}

func (u *Pipeline) GetStatus(ctx context.Context) (*domain.PipelineStatus, error) {
	total, err := u.repo.GetTotalJobs(ctx)
	if err != nil {
		return nil, err
	}
	today, err := u.repo.GetJobsToday(ctx)
	if err != nil {
		return nil, err
	}
	sources, err := u.repo.GetSourceStats(ctx)
	if err != nil {
		return nil, err
	}

	databaseHealthy := false
	if u.db != nil {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := u.db.Ping(pingCtx)
		cancel()
		databaseHealthy = err == nil
	}

	redisHealthy := false
	if u.redis != nil {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := u.redis.Ping(pingCtx)
		cancel()
		redisHealthy = err == nil
	}

	return &domain.PipelineStatus{
		TotalJobs:       total,
		JobsToday:       today,
		Sources:         sources,
		DatabaseHealthy: databaseHealthy,
		RedisHealthy:    redisHealthy,
		ServerTime:      u.now().UTC(),
	}, nil
}
