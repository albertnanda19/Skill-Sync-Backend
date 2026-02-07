package controller

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"skill-sync/internal/config"
)

type TriggerController interface {
	HandleSearchTrigger(ctx context.Context, keyword string)
}

type redisDeps interface {
	GetString(ctx context.Context, key string) (string, bool, error)
	SetIfNotExists(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	SetString(ctx context.Context, key string, value string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Publish(ctx context.Context, channel string, payload string) error
}

type Controller struct {
	cache         redisDeps
	python        PythonClient
	logger        *log.Logger
	cooldown      time.Duration
	lockTTL       time.Duration
	lastScrapeTTL time.Duration
}

func NewTriggerController(cache redisDeps, python PythonClient, cfg config.Config, logger *log.Logger) *Controller {
	cooldown := time.Duration(cfg.ScrapeCooldownSeconds) * time.Second
	if cooldown <= 0 {
		cooldown = 300 * time.Second
	}
	lockTTL := time.Duration(cfg.ScrapeLockTTLSeconds) * time.Second
	if lockTTL <= 0 {
		lockTTL = 300 * time.Second
	}
	return &Controller{
		cache:         cache,
		python:        python,
		logger:        logger,
		cooldown:      cooldown,
		lockTTL:       lockTTL,
		lastScrapeTTL: 1 * time.Hour,
	}
}

func (c *Controller) HandleSearchTrigger(ctx context.Context, keyword string) {
	if c == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			if c.logger != nil {
				c.logger.Printf("[SearchTrigger] keyword=%s event=panic err=%v", keyword, r)
			}
		}
	}()

	c.handle(ctx, keyword)
}

func (c *Controller) handle(ctx context.Context, keyword string) {
	if c.cache == nil || c.python == nil {
		return
	}

	kw := normalizeKeyword(keyword)
	if kw == "" {
		return
	}

	// STEP 2 — Check running lock
	if _, exists, _ := c.cache.GetString(ctx, lockKey(kw)); exists {
		if c.logger != nil {
			c.logger.Printf("[SearchTrigger] keyword=%s event=scrape_skipped reason=lock", kw)
		}
		return
	}

	// STEP 3 — Check cooldown
	if raw, ok, _ := c.cache.GetString(ctx, lastKey(kw)); ok {
		if lastUnix, err := strconv.ParseInt(raw, 10, 64); err == nil {
			last := time.Unix(lastUnix, 0)
			if time.Since(last) < c.cooldown {
				if c.logger != nil {
					c.logger.Printf("[SearchTrigger] keyword=%s event=scrape_skipped reason=cooldown", kw)
				}
				return
			}
		}
	}

	// STEP 4 — Acquire lock (atomic)
	nowUnix := strconv.FormatInt(time.Now().Unix(), 10)
	ok, err := c.cache.SetIfNotExists(ctx, lockKey(kw), nowUnix, c.lockTTL)
	if err != nil {
		if c.logger != nil {
			c.logger.Printf("[SearchTrigger] keyword=%s event=scrape_skipped reason=lock_error err=%v", kw, err)
		}
		return
	}
	if !ok {
		if c.logger != nil {
			c.logger.Printf("[SearchTrigger] keyword=%s event=scrape_skipped reason=lock_race", kw)
		}
		return
	}
	if c.logger != nil {
		c.logger.Printf("[SearchTrigger] keyword=%s event=lock_acquired", kw)
	}

	// STEP 5 — Trigger Python scraper
	ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	taskID, err := c.python.TriggerScrape(ctx2, kw)
	cancel()
	if err != nil {
		_ = c.cache.Delete(context.Background(), lockKey(kw))
		if c.logger != nil {
			c.logger.Printf("[SearchTrigger] keyword=%s event=scrape_failed err=%v", kw, err)
		}
		return
	}
	if taskID == "" {
		_ = c.cache.Delete(context.Background(), lockKey(kw))
		if c.logger != nil {
			c.logger.Printf("[SearchTrigger] keyword=%s event=scrape_failed err=%v", kw, fmt.Errorf("empty task id"))
		}
		return
	}
	if c.logger != nil {
		c.logger.Printf("[SearchTrigger] keyword=%s event=scrape_triggered task_id=%s", kw, taskID)
	}

	// STEP 6 — Store metadata
	_ = c.cache.SetString(context.Background(), taskKey(kw), taskID, c.lockTTL)

	// STEP 7 — Background monitor
	_ = monitorScrape(context.Background(), kw, taskID, c.python, c.cache, c.logger, c.lockTTL, c.lastScrapeTTL)
}

var _ TriggerController = (*Controller)(nil)
