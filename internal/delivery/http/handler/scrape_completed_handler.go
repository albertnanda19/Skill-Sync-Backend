package handler

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"skill-sync/internal/config"
	"skill-sync/internal/delivery/http/middleware"
	"skill-sync/internal/repository"
	"skill-sync/internal/ws"

	"github.com/gofiber/fiber/v3"
)

type ScrapeCompletedRequest struct {
	TaskID      string `json:"task_id"`
	Keyword     string `json:"keyword"`
	Source      string `json:"source"`
	CompletedAt string `json:"completed_at"`
}

type scrapeCacheInvalidator interface {
	InvalidateCacheByKeyword(ctx context.Context, keyword string) error
	GetString(ctx context.Context, key string) (string, bool, error)
	SetString(ctx context.Context, key string, value string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Publish(ctx context.Context, channel string, payload string) error
}

type ScrapeCompletedHandler struct {
	cfg    config.Config
	cache  scrapeCacheInvalidator
	jobs   repository.JobRepository
	logger *log.Logger
}

func NewScrapeCompletedHandler(cfg config.Config, cache scrapeCacheInvalidator, jobs repository.JobRepository, logger *log.Logger) *ScrapeCompletedHandler {
	return &ScrapeCompletedHandler{cfg: cfg, cache: cache, jobs: jobs, logger: logger}
}

func (h *ScrapeCompletedHandler) HandleScrapeCompleted(c fiber.Ctx) error {
	tok := strings.TrimSpace(c.Get("X-Internal-Token"))
	if tok == "" || tok != h.cfg.InternalToken {
		return middleware.NewAppError(fiber.StatusUnauthorized, "Unauthorized", nil, nil)
	}

	var req ScrapeCompletedRequest
	if err := c.Bind().Body(&req); err != nil {
		if h.logger != nil {
			h.logger.Printf("Webhook error | error=%v", err)
		}
		return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, err)
	}

	req.TaskID = strings.TrimSpace(req.TaskID)
	req.Keyword = strings.TrimSpace(req.Keyword)
	req.Source = strings.TrimSpace(req.Source)
	req.CompletedAt = strings.TrimSpace(req.CompletedAt)

	if req.TaskID == "" || req.Keyword == "" {
		return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, nil)
	}

	if req.CompletedAt != "" {
		if _, err := time.Parse(time.RFC3339, req.CompletedAt); err != nil {
			return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, err)
		}
	}

	if h.logger != nil {
		h.logger.Printf("Scrape completed | task=%s keyword=%s source=%s", req.TaskID, req.Keyword, req.Source)
	}

	kwNorm := strings.ToLower(strings.Join(strings.Fields(req.Keyword), " "))
	if kwNorm != "" {
		now := strconv.FormatInt(time.Now().Unix(), 10)
		_ = h.cache.SetString(context.Background(), "scrape:last:"+kwNorm, now, 1*time.Hour)
		_ = h.cache.Delete(context.Background(), "scrape:lock:"+kwNorm)
		_ = h.cache.Delete(context.Background(), "scrape:task:"+kwNorm)
		_ = h.cache.Publish(context.Background(), "jobs:invalidate:"+kwNorm, "1")
	}

	if h.cache != nil {
		if err := h.cache.InvalidateCacheByKeyword(c.Context(), req.Keyword); err != nil {
			if h.logger != nil {
				h.logger.Printf("Webhook error | error=%v", err)
			}
		}
	}

	if h.logger != nil {
		h.logger.Printf("Cache invalidated | keyword=%s", req.Keyword)
	}

	maxCreatedAt := time.Time{}
	if h.jobs != nil {
		if t, err := h.jobs.GetMaxJobCreatedAt(c.Context()); err == nil {
			maxCreatedAt = t
		}
	}

	kwKey := strings.ToLower(strings.Join(strings.Fields(req.Keyword), " "))
	srcKey := strings.ToLower(strings.Join(strings.Fields(req.Source), " "))
	markerKey := "ws:jobs_updated:last_max_created_at:" + kwKey + ":" + srcKey
	marker := ""
	if !maxCreatedAt.IsZero() {
		marker = maxCreatedAt.UTC().Format(time.RFC3339Nano)
	}

	shouldNotify := true
	if h.cache != nil && marker != "" {
		prev, ok, err := h.cache.GetString(context.Background(), markerKey)
		if err == nil && ok && strings.TrimSpace(prev) == marker {
			shouldNotify = false
		}
	}

	if shouldNotify {
		if h.cache != nil && marker != "" {
			_ = h.cache.SetString(context.Background(), markerKey, marker, 24*time.Hour)
		}
		v := true
		ws.NotifyJobsUpdatedWithState(req.Keyword, req.Source, &v, maxCreatedAt)
		if h.logger != nil {
			h.logger.Printf("WS notify | type=jobs_updated keyword=%s source=%s has_new_data=true", req.Keyword, req.Source)
		}
	} else if h.logger != nil {
		h.logger.Printf("WS notify skipped | type=jobs_updated keyword=%s source=%s has_new_data=false", req.Keyword, req.Source)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "cache_invalidated",
		"keyword": req.Keyword,
	})
}
