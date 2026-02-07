package handler

import (
	"context"
	"log"
	"strings"
	"time"

	"skill-sync/internal/config"
	"skill-sync/internal/delivery/http/middleware"
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
}

type ScrapeCompletedHandler struct {
	cfg    config.Config
	cache  scrapeCacheInvalidator
	logger *log.Logger
}

func NewScrapeCompletedHandler(cfg config.Config, cache scrapeCacheInvalidator, logger *log.Logger) *ScrapeCompletedHandler {
	return &ScrapeCompletedHandler{cfg: cfg, cache: cache, logger: logger}
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

	ws.NotifyJobsUpdated(req.Keyword, req.Source)
	if h.logger != nil {
		h.logger.Printf("WS notify | type=jobs_updated keyword=%s source=%s", req.Keyword, req.Source)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "cache_invalidated",
		"keyword": req.Keyword,
	})
}
