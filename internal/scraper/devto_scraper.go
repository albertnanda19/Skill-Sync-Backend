package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"skill-sync/internal/database"

	"github.com/google/uuid"
)

type DevtoScraper struct {
	db       database.DB
	client   *http.Client
	apiBase  string
	siteBase string
}

func NewDevtoScraper(db database.DB) *DevtoScraper {
	return &DevtoScraper{
		db: db,
		client: &http.Client{
			Timeout: 25 * time.Second,
		},
		apiBase:  "https://dev.to",
		siteBase: "https://dev.to",
	}
}

type devtoListing struct {
	ID           int     `json:"id"`
	Title        string  `json:"title"`
	Category     string  `json:"category"`
	Slug         string  `json:"slug"`
	Organization string  `json:"organization_name"`
	Company      string  `json:"company_name"`
	Location     string  `json:"location"`
	PublishedAt  *string `json:"published_at"`
	URL          string  `json:"url"`
}

type devtoListingDetail struct {
	ID              int     `json:"id"`
	Title           string  `json:"title"`
	Organization    string  `json:"organization_name"`
	Company         string  `json:"company_name"`
	Location        string  `json:"location"`
	PublishedAt     *string `json:"published_at"`
	BodyMarkdown    string  `json:"body_markdown"`
	URL             string  `json:"url"`
	ContactViaEmail bool    `json:"contact_via_email"`
}

func (s *DevtoScraper) Scrape(ctx context.Context, pages int, workers int) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("nil scraper/db")
	}
	if pages <= 0 {
		pages = 1
	}

	sourceID, err := ensureJobSource(ctx, s.db, "Dev.to Jobs", s.siteBase)
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
	pool.SetRateLimit(4)
	results := pool.Run(ctx)

	for page := 1; page <= pages; page++ {
		listings, err := s.fetchListings(ctx, page)
		if err != nil {
			_ = logScrape(ctx, s.db, runID, "error", fmt.Sprintf("devto listings page %d: %v", page, err))
			continue
		}
		for _, it := range listings {
			it := it
			if it.ID == 0 {
				continue
			}
			pool.Submit(func(ctx context.Context) error {
				detail, err := s.fetchListingDetail(ctx, it.ID)
				if err != nil {
					return err
				}
				return insertRawJob(ctx, s.db, sourceID, runID, rawJobInput{
					ExternalJobID:  strconv.Itoa(detail.ID),
					Title:          detail.Title,
					Company:        pickNonEmpty(detail.Company, detail.Organization),
					Location:       detail.Location,
					EmploymentType: it.Category,
					Description:    detail.BodyMarkdown,
					RawDescription: detail.BodyMarkdown,
					PostedAt:       parseRFC3339OrNil(detail.PublishedAt),
					URL:            normalizeURL(pickNonEmpty(detail.URL, it.URL)),
					IsActive:       true,
				})
			})
		}
	}

	pool.Close()

	for res := range results {
		if res.Err != nil {
			_ = logScrape(ctx, s.db, runID, "error", fmt.Sprintf("devto item: %v", res.Err))
		}
	}

	return nil
}

func (s *DevtoScraper) fetchListings(ctx context.Context, page int) ([]devtoListing, error) {
	url := fmt.Sprintf("%s/api/listings?category=jobs&per_page=30&page=%d", strings.TrimRight(s.apiBase, "/"), page)
	body, err := httpGetWithRetry(ctx, s.client, url, 3)
	if err != nil {
		return nil, err
	}
	var out []devtoListing
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *DevtoScraper) fetchListingDetail(ctx context.Context, id int) (devtoListingDetail, error) {
	url := fmt.Sprintf("%s/api/listings/%d", strings.TrimRight(s.apiBase, "/"), id)
	body, err := httpGetWithRetry(ctx, s.client, url, 3)
	if err != nil {
		return devtoListingDetail{}, err
	}
	var out devtoListingDetail
	if err := json.Unmarshal(body, &out); err != nil {
		return devtoListingDetail{}, err
	}
	return out, nil
}

func httpGetWithRetry(ctx context.Context, client *http.Client, url string, attempts int) ([]byte, error) {
	if attempts <= 0 {
		attempts = 1
	}
	var body []byte
	var lastErr error
	for i := 0; i < attempts; i++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "SkillSyncScraper/0.1")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(300*(i+1)) * time.Millisecond)
			continue
		}
		func() {
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				lastErr = fmt.Errorf("status %d", resp.StatusCode)
				return
			}
			b, err := readAllLimit(resp.Body, 5<<20)
			if err != nil {
				lastErr = err
				return
			}
			lastErr = nil
			body = b
		}()
		if lastErr == nil {
			return body, nil
		}
		time.Sleep(time.Duration(300*(i+1)) * time.Millisecond)
	}
	return nil, lastErr
}

func normalizeURL(u string) string {
	u = strings.TrimSpace(u)
	return u
}

func pickNonEmpty(a, b string) string {
	a = strings.TrimSpace(a)
	if a != "" {
		return a
	}
	return strings.TrimSpace(b)
}

func parseRFC3339OrNil(s *string) *time.Time {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	tm, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil
	}
	tm = tm.UTC()
	return &tm
}
