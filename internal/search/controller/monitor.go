package controller

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"
)

type monitorDeps interface {
	SetString(ctx context.Context, key string, value string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Publish(ctx context.Context, channel string, payload string) error
}

func monitorScrape(ctx context.Context, keyword string, taskID string, py PythonClient, cache monitorDeps, logger *log.Logger, lockTTL time.Duration, lastTTL time.Duration) error {
	if py == nil {
		return fmt.Errorf("nil python client")
	}
	if cache == nil {
		return fmt.Errorf("nil cache")
	}

	deadline := time.Now().Add(lockTTL)
	pollInterval := 2 * time.Second

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("monitor timeout keyword=%s task_id=%s", keyword, taskID)
		}

		ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
		status, err := py.GetScrapeStatus(ctx2, taskID)
		cancel()
		if err != nil {
			if logger != nil {
				logger.Printf("[SearchTrigger] keyword=%s event=scrape_monitor_error task_id=%s err=%v", keyword, taskID, err)
			}
			time.Sleep(pollInterval)
			continue
		}

		switch status {
		case "completed", "complete", "done", "success":
			now := strconv.FormatInt(time.Now().Unix(), 10)
			_ = cache.SetString(context.Background(), lastKey(keyword), now, lastTTL)
			_ = cache.Delete(context.Background(), lockKey(keyword))
			_ = cache.Publish(context.Background(), invalidateChannel(keyword), "1")
			if logger != nil {
				logger.Printf("[SearchTrigger] keyword=%s event=scrape_completed task_id=%s", keyword, taskID)
			}
			return nil
		case "failed", "error":
			_ = cache.Delete(context.Background(), lockKey(keyword))
			if logger != nil {
				logger.Printf("[SearchTrigger] keyword=%s event=scrape_failed task_id=%s", keyword, taskID)
			}
			return fmt.Errorf("scrape failed keyword=%s task_id=%s", keyword, taskID)
		default:
			if logger != nil {
				logger.Printf("[SearchTrigger] keyword=%s event=scrape_running task_id=%s status=%s", keyword, taskID, status)
			}
			time.Sleep(pollInterval)
		}
	}
}
