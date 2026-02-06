package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"skill-sync/internal/database"

	"github.com/google/uuid"
)

type GlintsScraper struct {
	db       database.DB
	client   *http.Client
	apiBase  string
	siteBase string
}

func NewGlintsScraper(db database.DB) *GlintsScraper {
	return &GlintsScraper{
		db: db,
		client: &http.Client{
			Timeout: 25 * time.Second,
		},
		apiBase:  "https://glints.com",
		siteBase: "https://glints.com",
	}
}

type glintsJobItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Location  string `json:"location"`
	Company   string `json:"company"`
	CreatedAt string `json:"createdAt"`
	URL       string `json:"url"`
	Slug      string `json:"slug"`
}

type glintsSearchResponse struct {
	Jobs []glintsJobItem `json:"jobs"`
}

type glintsJobDetail struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Location    string `json:"location"`
	CompanyName string `json:"companyName"`
	Description string `json:"description"`
	CreatedAt   string `json:"createdAt"`
	URL         string `json:"url"`
}

func (s *GlintsScraper) Scrape(ctx context.Context, pages int, workers int) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("nil scraper/db")
	}
	if pages <= 0 {
		pages = 1
	}

	sourceID, err := ensureJobSource(ctx, s.db, "Glints", s.siteBase)
	if err != nil {
		return err
	}

	runID, _ := createScrapeRun(ctx, s.db, sourceID)
	if runID != uuid.Nil {
		defer func() {
			_ = finishScrapeRun(context.Background(), s.db, runID, "finished")
		}()
	}

	_ = deactivateJobsForSource(ctx, s.db, sourceID)

	pool := NewWorkerPool(workers, workers*2)
	pool.SetRateLimit(3)
	results := pool.Run(ctx)

	for page := 1; page <= pages; page++ {
		items, err := s.fetchSearchPage(ctx, page)
		if err != nil {
			_ = logScrape(ctx, s.db, runID, "error", fmt.Sprintf("glints search page %d: %v", page, err))
			continue
		}
		for _, it := range items {
			it := it
			jobID := strings.TrimSpace(it.ID)
			if jobID == "" {
				continue
			}
			pool.Submit(func(ctx context.Context) error {
				detail, err := s.fetchJobDetail(ctx, jobID)
				if err != nil {
					return err
				}

				url := normalizeURL(pickNonEmpty(detail.URL, it.URL))
				if strings.TrimSpace(url) == "" {
					if strings.TrimSpace(it.Slug) != "" {
						url = strings.TrimRight(s.siteBase, "/") + "/id/opportunities/jobs/" + strings.TrimLeft(it.Slug, "/")
					}
				}

				postedAt := parseRFC3339OrNil(stringPtr(detail.CreatedAt))
				if postedAt == nil {
					postedAt = parseRFC3339OrNil(stringPtr(it.CreatedAt))
				}

				return insertRawJob(ctx, s.db, sourceID, runID, rawJobInput{
					ExternalJobID:  detail.ID,
					Title:          pickNonEmpty(detail.Title, it.Title),
					Company:        pickNonEmpty(detail.CompanyName, it.Company),
					Location:       pickNonEmpty(detail.Location, it.Location),
					Description:    detail.Description,
					RawDescription: detail.Description,
					PostedAt:       postedAt,
					URL:            url,
					IsActive:       true,
				})
			})
		}
	}

	pool.Close()
	for res := range results {
		if res.Err != nil {
			_ = logScrape(ctx, s.db, runID, "error", fmt.Sprintf("glints item: %v", res.Err))
		}
	}
	return nil
}

func (s *GlintsScraper) fetchSearchPage(ctx context.Context, page int) ([]glintsJobItem, error) {
	url := fmt.Sprintf("%s/api/v1/jobs?page=%d", strings.TrimRight(s.apiBase, "/"), page)
	body, err := httpGetWithRetry(ctx, s.client, url, 3)
	if err != nil {
		return nil, err
	}
	var out glintsSearchResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out.Jobs, nil
}

func (s *GlintsScraper) fetchJobDetail(ctx context.Context, jobID string) (glintsJobDetail, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return glintsJobDetail{}, fmt.Errorf("empty job id")
	}
	url := fmt.Sprintf("%s/api/v1/jobs/%s", strings.TrimRight(s.apiBase, "/"), jobID)
	body, err := httpGetWithRetry(ctx, s.client, url, 3)
	if err != nil {
		return glintsJobDetail{}, err
	}
	var out glintsJobDetail
	if err := json.Unmarshal(body, &out); err != nil {
		return glintsJobDetail{}, err
	}
	if strings.TrimSpace(out.ID) == "" {
		out.ID = jobID
	}
	return out, nil
}

func stringPtr(s string) *string {
	v := strings.TrimSpace(s)
	if v == "" {
		return nil
	}
	return &v
}
