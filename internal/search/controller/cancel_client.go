package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type TaskCanceler interface {
	DeleteScrapeTask(ctx context.Context, taskID string, force bool) (deleted bool, err error)
}

type httpTaskCanceler struct {
	baseURL        string
	internalToken  string
	client         *http.Client
	logger         *log.Logger
}

type deleteTaskResponse struct {
	TaskID  string `json:"task_id"`
	Deleted bool   `json:"deleted"`
}

func NewTaskCanceler(baseURL string, internalToken string, logger *log.Logger) TaskCanceler {
	baseURL = strings.TrimSpace(baseURL)
	internalToken = strings.TrimSpace(internalToken)
	if baseURL == "" || internalToken == "" {
		return nil
	}
	return &httpTaskCanceler{
		baseURL:       strings.TrimRight(baseURL, "/"),
		internalToken: internalToken,
		client:        &http.Client{Timeout: 5 * time.Second},
		logger:        logger,
	}
}

func (c *httpTaskCanceler) DeleteScrapeTask(ctx context.Context, taskID string, force bool) (bool, error) {
	if c == nil {
		return false, errors.New("nil task canceler")
	}
	if c.client == nil {
		return false, errors.New("nil http client")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return false, errors.New("empty task id")
	}

	u, err := url.Parse(c.baseURL + "/internal/scrape/" + url.PathEscape(taskID))
	if err != nil {
		return false, err
	}
	q := u.Query()
	if force {
		q.Set("force", "true")
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u.String(), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("X-Internal-Token", c.internalToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		rb, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		bodyStr := strings.TrimSpace(string(rb))
		err := fmt.Errorf("python delete task failed: status=%d body=%s", resp.StatusCode, bodyStr)
		if c.logger != nil {
			c.logger.Printf("[SearchTrigger] python_delete_task_error endpoint=%s status=%d body=%q", u.String(), resp.StatusCode, bodyStr)
		}
		return false, err
	}

	var out deleteTaskResponse
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&out); err != nil {
		return false, err
	}
	return out.Deleted, nil
}

var _ TaskCanceler = (*httpTaskCanceler)(nil)
