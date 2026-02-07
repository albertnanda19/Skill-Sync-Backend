package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type PythonClient interface {
	TriggerScrape(ctx context.Context, keyword string) (taskID string, err error)
	GetScrapeStatus(ctx context.Context, taskID string) (status string, err error)
}

type httpPythonClient struct {
	baseURL string
	client  *http.Client
	logger  *log.Logger
}

type triggerScrapeRequest struct {
	Query    string `json:"query"`
	Location string `json:"location"`
}

type triggerScrapeResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

type scrapeStatusResponse struct {
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

func NewPythonClient(baseURL string, logger *log.Logger) PythonClient {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil
	}
	return &httpPythonClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 5 * time.Second},
		logger:  logger,
	}
}

func (c *httpPythonClient) TriggerScrape(ctx context.Context, keyword string) (string, error) {
	if c == nil {
		return "", errors.New("nil python client")
	}
	if c.client == nil {
		return "", errors.New("nil http client")
	}
	endpoint := c.baseURL + "/scrape"

	body := triggerScrapeRequest{Query: strings.TrimSpace(keyword), Location: "Indonesia"}
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		rb, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		bodyStr := strings.TrimSpace(string(rb))
		err := fmt.Errorf("python scrape trigger failed: status=%d body=%s", resp.StatusCode, bodyStr)
		if c.logger != nil {
			c.logger.Printf("[SearchTrigger] python_trigger_error endpoint=%s status=%d body=%q", endpoint, resp.StatusCode, bodyStr)
		}
		return "", err
	}

	var out triggerScrapeResponse
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&out); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.TaskID), nil
}

func (c *httpPythonClient) GetScrapeStatus(ctx context.Context, taskID string) (string, error) {
	if c == nil {
		return "", errors.New("nil python client")
	}
	if c.client == nil {
		return "", errors.New("nil http client")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "", errors.New("empty task id")
	}
	endpoint := c.baseURL + "/scrape/" + taskID

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		rb, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		bodyStr := strings.TrimSpace(string(rb))
		err := fmt.Errorf("python scrape status failed: status=%d body=%s", resp.StatusCode, bodyStr)
		if c.logger != nil {
			c.logger.Printf("[SearchTrigger] python_status_error endpoint=%s status=%d body=%q", endpoint, resp.StatusCode, bodyStr)
		}
		return "", err
	}

	var out scrapeStatusResponse
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&out); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.Status), nil
}

var _ PythonClient = (*httpPythonClient)(nil)
