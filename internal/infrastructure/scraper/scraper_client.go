package scraper

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

type ScraperClient interface {
	TriggerScrape(ctx context.Context, query string, location string) (taskID string, err error)
}

type httpScraperClient struct {
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

func NewScraperClient(baseURL string, logger *log.Logger) ScraperClient {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil
	}
	return &httpScraperClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 5 * time.Second},
		logger:  logger,
	}
}

func (c *httpScraperClient) TriggerScrape(ctx context.Context, query string, location string) (string, error) {
	if c == nil {
		return "", errors.New("nil scraper client")
	}
	if c.client == nil {
		return "", errors.New("nil http client")
	}
	endpoint := c.baseURL + "/scrape"

	body := triggerScrapeRequest{Query: strings.TrimSpace(query), Location: strings.TrimSpace(location)}
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
		err := fmt.Errorf("scraper trigger failed: status=%d body=%s", resp.StatusCode, bodyStr)
		if c.logger != nil {
			c.logger.Printf("[Scraper] TriggerScrape error endpoint=%s status=%d body=%q", endpoint, resp.StatusCode, bodyStr)
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

var _ ScraperClient = (*httpScraperClient)(nil)
