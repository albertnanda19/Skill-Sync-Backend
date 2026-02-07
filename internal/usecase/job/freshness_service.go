package job

import (
	"context"
	"log"
	"strings"
	"time"

	"skill-sync/internal/infrastructure/scraper"
	"skill-sync/internal/repository"
)

type freshnessCache interface {
	SetIfNotExists(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
}

type FreshnessService struct {
	repo               repository.JobRepository
	scraper            scraper.ScraperClient
	cache              freshnessCache
	logger             *log.Logger
	freshnessThreshold time.Duration
}

func NewFreshnessService(repo repository.JobRepository, scraperClient scraper.ScraperClient, cache freshnessCache, logger *log.Logger, freshnessMinutes int) *FreshnessService {
	threshold := time.Duration(freshnessMinutes) * time.Minute
	if threshold <= 0 {
		threshold = 30 * time.Minute
	}
	return &FreshnessService{
		repo:               repo,
		scraper:            scraperClient,
		cache:              cache,
		logger:             logger,
		freshnessThreshold: threshold,
	}
}

func (s *FreshnessService) EnsureFresh(ctx context.Context, query, location string) {
	if s == nil {
		return
	}
	if s.repo == nil {
		return
	}
	if s.scraper == nil {
		return
	}

	query = strings.TrimSpace(query)
	location = strings.TrimSpace(location)
	if query == "" && location == "" {
		return
	}

	latest, err := s.repo.GetLatestScrapedAt(ctx, query, location)
	if err != nil {
		return
	}

	stale := latest.IsZero() || time.Since(latest) > s.freshnessThreshold
	if !stale {
		return
	}
	if s.logger != nil {
		s.logger.Printf("[Jobs] Freshness stale detected query=%q location=%q latest=%v threshold=%s", query, location, latest, s.freshnessThreshold)
	}

	// Production-safe: use Redis lock if available to avoid repeated triggers.
	lockKey := "jobs:freshness:lock:"
	if query != "" {
		lockKey += "q=" + strings.ToLower(query)
	}
	if location != "" {
		lockKey += ":l=" + strings.ToLower(location)
	}
	lockKey = strings.Join(strings.Fields(lockKey), " ")
	lockAcquired := true
	if s.cache != nil {
		ok, err := s.cache.SetIfNotExists(ctx, lockKey, "1", 2*time.Minute)
		if err == nil {
			lockAcquired = ok
		}
	}
	if !lockAcquired {
		return
	}

	go func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		taskID, err := s.scraper.TriggerScrape(ctx2, query, location)
		if err != nil {
			if s.logger != nil {
				s.logger.Printf("[Jobs] Scrape trigger error query=%q location=%q err=%v", query, location, err)
			}
			return
		}
		if s.logger != nil {
			s.logger.Printf("[Jobs] Scrape triggered query=%q location=%q task_id=%s", query, location, taskID)
		}
	}()
}
