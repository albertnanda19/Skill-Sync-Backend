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

type CompanyScraper struct {
	db database.DB
}

func NewCompanyScraper(db database.DB) *CompanyScraper {
	return &CompanyScraper{db: db}
}

type CompanyCareersTarget struct {
	SourceName         string
	BaseURL            string
	ListURL            string
	LinkSelector       string
	TitleSelector      string
	LocationSelector   string
	DetailBodySelector string
}

type companyListItem struct {
	Link     string
	Title    string
	Location string
}

type companyDetail struct {
	Title       string
	Location    string
	Description string
	URL         string
}

func (s *CompanyScraper) Scrape(ctx context.Context, targets []CompanyCareersTarget, pages int, workers int) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("nil scraper/db")
	}
	if len(targets) == 0 {
		return nil
	}
	if workers <= 0 {
		workers = 4
	}

	for _, t := range targets {
		t := t
		if strings.TrimSpace(t.SourceName) == "" || strings.TrimSpace(t.ListURL) == "" {
			continue
		}
		if strings.TrimSpace(t.BaseURL) == "" {
			t.BaseURL = t.ListURL
		}
		if strings.TrimSpace(t.LinkSelector) == "" {
			t.LinkSelector = "a"
		}
		if strings.TrimSpace(t.TitleSelector) == "" {
			t.TitleSelector = "title"
		}
		if strings.TrimSpace(t.DetailBodySelector) == "" {
			t.DetailBodySelector = "body"
		}

		sourceID, err := ensureJobSource(ctx, s.db, t.SourceName, t.BaseURL)
		if err != nil {
			continue
		}

		runID, _ := createScrapeRun(ctx, s.db, sourceID)
		if runID != uuid.Nil {
			defer func(r uuid.UUID) {
				_ = finishScrapeRun(context.Background(), s.db, r, "finished")
			}(runID)
		}

		_ = deactivateJobsForSource(ctx, s.db, sourceID)

		pool := NewWorkerPool(workers, workers*2)
		pool.SetRateLimit(3)
		results := pool.Run(ctx)

		for page := 1; page <= maxInt(1, pages); page++ {
			listURL := t.ListURL
			if strings.Contains(listURL, "%d") {
				listURL = fmt.Sprintf(listURL, page)
			}
			items, err := s.scrapeListingPage(ctx, t, listURL)
			if err != nil {
				_ = logScrape(ctx, s.db, runID, "error", fmt.Sprintf("company list page %d: %v", page, err))
				continue
			}
			for _, it := range items {
				it := it
				if strings.TrimSpace(it.Link) == "" {
					continue
				}
				link := it.Link
				pool.Submit(func(ctx context.Context) error {
					d, err := s.scrapeDetailPage(ctx, t, link)
					if err != nil {
						return err
					}
					return insertRawJob(ctx, s.db, sourceID, runID, rawJobInput{
						ExternalJobID:  stableExternalIDFromURL(d.URL),
						Title:          pickNonEmpty(d.Title, it.Title),
						Company:        t.SourceName,
						Location:       pickNonEmpty(d.Location, it.Location),
						Description:    d.Description,
						RawDescription: d.Description,
						PostedAt:       nil,
						URL:            normalizeURL(d.URL),
						IsActive:       true,
					})
				})
			}
		}

		pool.Close()
		for res := range results {
			if res.Err != nil {
				_ = logScrape(ctx, s.db, runID, "error", fmt.Sprintf("company item: %v", res.Err))
			}
		}
	}

	return nil
}

func (s *CompanyScraper) scrapeListingPage(ctx context.Context, target CompanyCareersTarget, listURL string) ([]companyListItem, error) {
	allowed := hostFromURL(listURL)
	var c *colly.Collector
	if strings.TrimSpace(allowed) == "" {
		c = colly.NewCollector()
	} else {
		c = colly.NewCollector(colly.AllowedDomains(allowed))
	}
	_ = c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 2, RandomDelay: 850 * time.Millisecond, Delay: 450 * time.Millisecond})

	items := make([]companyListItem, 0)
	dedup := map[string]struct{}{}

	c.OnRequest(func(r *colly.Request) {
		for k, v := range httpHeaders() {
			r.Headers.Set(k, v)
		}
	})

	c.OnHTML(target.LinkSelector, func(e *colly.HTMLElement) {
		href := strings.TrimSpace(e.Attr("href"))
		if href == "" {
			return
		}
		abs := e.Request.AbsoluteURL(href)
		abs = normalizeURL(abs)
		if abs == "" {
			return
		}
		if _, ok := dedup[abs]; ok {
			return
		}
		dedup[abs] = struct{}{}

		title := ""
		if strings.TrimSpace(target.TitleSelector) != "" {
			title = strings.TrimSpace(e.DOM.Find(target.TitleSelector).Text())
		}
		location := ""
		if strings.TrimSpace(target.LocationSelector) != "" {
			location = strings.TrimSpace(e.DOM.Find(target.LocationSelector).Text())
		}

		items = append(items, companyListItem{Link: abs, Title: title, Location: location})
	})

	var reqErr error
	c.OnError(func(r *colly.Response, err error) {
		reqErr = err
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
	return items, nil
}

func (s *CompanyScraper) scrapeDetailPage(ctx context.Context, target CompanyCareersTarget, jobURL string) (companyDetail, error) {
	allowed := hostFromURL(jobURL)
	var c *colly.Collector
	if strings.TrimSpace(allowed) == "" {
		c = colly.NewCollector()
	} else {
		c = colly.NewCollector(colly.AllowedDomains(allowed))
	}
	_ = c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 2, RandomDelay: 900 * time.Millisecond, Delay: 500 * time.Millisecond})

	var out companyDetail
	out.URL = jobURL
	var reqErr error

	c.OnRequest(func(r *colly.Request) {
		for k, v := range httpHeaders() {
			r.Headers.Set(k, v)
		}
	})

	c.OnHTML(target.TitleSelector, func(e *colly.HTMLElement) {
		if strings.TrimSpace(out.Title) == "" {
			out.Title = strings.TrimSpace(e.Text)
		}
	})

	if strings.TrimSpace(target.LocationSelector) != "" {
		c.OnHTML(target.LocationSelector, func(e *colly.HTMLElement) {
			if strings.TrimSpace(out.Location) == "" {
				out.Location = strings.TrimSpace(e.Text)
			}
		})
	}

	c.OnHTML(target.DetailBodySelector, func(e *colly.HTMLElement) {
		out.Description = strings.TrimSpace(e.Text)
	})

	c.OnError(func(r *colly.Response, err error) {
		reqErr = err
	})

	if ctx.Err() != nil {
		return companyDetail{}, ctx.Err()
	}
	if err := c.Visit(jobURL); err != nil {
		return companyDetail{}, err
	}
	c.Wait()
	if reqErr != nil {
		return companyDetail{}, reqErr
	}
	return out, nil
}

func hostFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := u.Host
	if host == "" {
		return ""
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
