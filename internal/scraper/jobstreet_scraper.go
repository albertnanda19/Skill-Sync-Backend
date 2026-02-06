package scraper

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"skill-sync/internal/database"

	"github.com/gocolly/colly/v2"
	"github.com/google/uuid"
)

type JobStreetScraper struct {
	db          database.DB
	baseURL     string
	allowedHost string
}

func NewJobStreetScraper(db database.DB) *JobStreetScraper {
	s := &JobStreetScraper{db: db, baseURL: "https://www.jobstreet.co.id"}
	s.allowedHost = hostFromBaseURL(s.baseURL)
	return s
}

func NewJobStreetScraperWithBaseURL(db database.DB, baseURL string) *JobStreetScraper {
	s := &JobStreetScraper{db: db, baseURL: strings.TrimSpace(baseURL)}
	if s.baseURL == "" {
		s.baseURL = "https://www.jobstreet.co.id"
	}
	s.allowedHost = hostFromBaseURL(s.baseURL)
	return s
}

type jobstreetListItem struct {
	Title    string
	Company  string
	Location string
	Link     string
}

func (s *JobStreetScraper) Scrape(ctx context.Context, startURLTemplate string, pages int, workers int) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("nil scraper/db")
	}
	if pages <= 0 {
		pages = 1
	}
	if strings.TrimSpace(startURLTemplate) == "" {
		startURLTemplate = strings.TrimRight(s.baseURL, "/") + "/id/job-search/jobs?sort=createdAt&page=%d"
	}

	sourceID, err := ensureJobSource(ctx, s.db, "JobStreet", s.baseURL)
	if err != nil {
		return err
	}

	runID, _ := createScrapeRun(ctx, s.db, sourceID)
	if runID != uuid.Nil {
		defer func() {
			_ = finishScrapeRun(context.Background(), s.db, runID, "finished")
		}()
	}

	pool := NewWorkerPool(workers, workers*2)
	results := pool.Run(ctx)

	for page := 1; page <= pages; page++ {
		listURL := fmt.Sprintf(startURLTemplate, page)
		items, err := s.scrapeListingPage(ctx, listURL)
		if err != nil {
			_ = logScrape(ctx, s.db, runID, "error", fmt.Sprintf("jobstreet list page %d: %v", page, err))
			continue
		}
		for _, it := range items {
			it := it
			if strings.TrimSpace(it.Link) == "" {
				continue
			}
			link := it.Link
			pool.Submit(func(ctx context.Context) error {
				detail, err := s.scrapeDetailPage(ctx, link)
				if err != nil {
					return err
				}
				return insertRawJob(ctx, s.db, sourceID, runID, rawJobInput{
					ExternalJobID:  detail.externalID,
					Title:          pickNonEmpty(detail.title, it.Title),
					Company:        pickNonEmpty(detail.company, it.Company),
					Location:       pickNonEmpty(detail.location, it.Location),
					EmploymentType: detail.employmentType,
					Description:    detail.description,
					RawDescription: detail.rawDescription,
					PostedAt:       detail.postedAt,
					URL:            normalizeURL(link),
					IsActive:       true,
				})
			})
		}
	}

	pool.Close()

	for res := range results {
		if res.Err != nil {
			_ = logScrape(ctx, s.db, runID, "error", fmt.Sprintf("jobstreet item: %v", res.Err))
		}
	}

	return nil
}

func (s *JobStreetScraper) scrapeListingPage(ctx context.Context, listURL string) ([]jobstreetListItem, error) {
	c := colly.NewCollector(
		colly.AllowedDomains(s.allowedHost),
	)

	_ = c.Limit(&colly.LimitRule{DomainGlob: "*jobstreet.co.id*", Parallelism: 2, RandomDelay: 750 * time.Millisecond, Delay: 400 * time.Millisecond})

	items := make([]jobstreetListItem, 0)

	c.OnHTML("a", func(e *colly.HTMLElement) {
		href := strings.TrimSpace(e.Attr("href"))
		if href == "" {
			return
		}
		if !strings.Contains(href, "/job/") && !strings.Contains(href, "/en/job/") && !strings.Contains(href, "/id/job/") {
			return
		}
		abs := e.Request.AbsoluteURL(href)
		if abs == "" {
			return
		}
		items = append(items, jobstreetListItem{Link: abs})
	})

	var reqErr error
	c.OnError(func(r *colly.Response, err error) {
		reqErr = err
	})

	c.OnRequest(func(r *colly.Request) {
		for k, v := range httpHeaders() {
			r.Headers.Set(k, v)
		}
	})

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if err := c.Visit(listURL); err != nil {
		return nil, err
	}

	c.Wait()
	if reqErr != nil {
		return nil, reqErr
	}

	dedup := map[string]struct{}{}
	out := make([]jobstreetListItem, 0, len(items))
	for _, it := range items {
		u := normalizeURL(it.Link)
		if u == "" {
			continue
		}
		if _, ok := dedup[u]; ok {
			continue
		}
		dedup[u] = struct{}{}
		out = append(out, jobstreetListItem{Link: u})
	}
	return out, nil
}

type jobstreetDetail struct {
	externalID     string
	title          string
	company        string
	location       string
	employmentType string
	description    string
	rawDescription string
	postedAt       *time.Time
}

func (s *JobStreetScraper) scrapeDetailPage(ctx context.Context, jobURL string) (jobstreetDetail, error) {
	c := colly.NewCollector(
		colly.AllowedDomains(s.allowedHost),
	)
	_ = c.Limit(&colly.LimitRule{DomainGlob: "*jobstreet.co.id*", Parallelism: 2, RandomDelay: 850 * time.Millisecond, Delay: 450 * time.Millisecond})

	var out jobstreetDetail
	var reqErr error

	c.OnRequest(func(r *colly.Request) {
		for k, v := range httpHeaders() {
			r.Headers.Set(k, v)
		}
	})

	c.OnResponse(func(r *colly.Response) {
		out.externalID = extractJobStreetExternalID(jobURL)
	})

	c.OnHTML("title", func(e *colly.HTMLElement) {
		out.title = strings.TrimSpace(e.Text)
	})

	c.OnHTML("h1", func(e *colly.HTMLElement) {
		if strings.TrimSpace(out.title) == "" {
			out.title = strings.TrimSpace(e.Text)
		}
	})

	c.OnHTML("body", func(e *colly.HTMLElement) {
		out.rawDescription = strings.TrimSpace(e.DOM.Find("body").Text())
		out.description = out.rawDescription
	})

	c.OnError(func(r *colly.Response, err error) {
		reqErr = err
	})

	if ctx.Err() != nil {
		return jobstreetDetail{}, ctx.Err()
	}
	if err := c.Visit(jobURL); err != nil {
		return jobstreetDetail{}, err
	}

	c.Wait()
	if reqErr != nil {
		return jobstreetDetail{}, reqErr
	}
	if strings.TrimSpace(out.externalID) == "" {
		out.externalID = extractJobStreetExternalID(jobURL)
	}
	return out, nil
}

func extractJobStreetExternalID(jobURL string) string {
	jobURL = strings.TrimSpace(jobURL)
	u, err := url.Parse(jobURL)
	if err == nil {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) > 0 {
			last := parts[len(parts)-1]
			last = strings.TrimSpace(last)
			if last != "" {
				return last
			}
		}
	}
	return jobURL
}

func httpHeaders() map[string]string {
	return map[string]string{
		"User-Agent":      "SkillSyncScraper/0.1",
		"Accept-Language": "en-US,en;q=0.9,id;q=0.8",
	}
}

func hostFromBaseURL(base string) string {
	base = strings.TrimSpace(base)
	u, err := url.Parse(base)
	if err != nil {
		return "www.jobstreet.co.id"
	}
	host := u.Host
	if host == "" {
		return "www.jobstreet.co.id"
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}
