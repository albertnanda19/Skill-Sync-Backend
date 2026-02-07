package service

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"skill-sync/internal/repository"

	"github.com/gocolly/colly/v2"
)

type JobStreetScraper struct {
	baseURL        string
	allowedHost    string
	allowedHostAlt string
}

func NewJobStreetScraper() *JobStreetScraper {
	base := "https://www.jobstreet.co.id"
	s := &JobStreetScraper{baseURL: base}
	s.allowedHost = hostFromBaseURL(base)
	s.allowedHostAlt = "id.jobstreet.com"
	return s
}

func (s *JobStreetScraper) Name() string {
	return "JobStreet"
}

func (s *JobStreetScraper) Search(ctx context.Context, params SearchParams) ([]repository.JobUpsert, error) {
	q := strings.TrimSpace(params.Title)
	if q == "" {
		q = strings.TrimSpace(params.Location)
	}
	if q == "" {
		q = "jobs"
	}

	startURL := strings.TrimRight(s.baseURL, "/") + "/id/job-search/" + urlPathEscapeLoose(q) + "-jobs/in-" + urlPathEscapeLoose(strings.TrimSpace(params.Location)) + "?sort=createdAt&page=%d"
	if strings.TrimSpace(params.Location) == "" {
		startURL = strings.TrimRight(s.baseURL, "/") + "/id/job-search/" + urlPathEscapeLoose(q) + "-jobs?sort=createdAt&page=%d"
	}

	items, err := s.scrapeListingOnly(ctx, startURL, 50)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	out := make([]repository.JobUpsert, 0, len(items))
	for _, it := range items {
		if strings.TrimSpace(it.Link) == "" {
			continue
		}
		out = append(out, repository.JobUpsert{
			SourceName:    s.Name(),
			SourceBaseURL: s.baseURL,
			SourceURL:     strings.TrimSpace(it.Link),
			Title:         strings.TrimSpace(it.Title),
			Company:       strings.TrimSpace(it.Company),
			Location:      strings.TrimSpace(it.Location),
			ScrapedAt:     &now,
			IsActive:      true,
		})
	}
	return out, nil
}

func urlPathEscapeLoose(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	return s
}

type jobstreetListItem struct {
	Title    string
	Company  string
	Location string
	Link     string
}

func (s *JobStreetScraper) scrapeListingOnly(ctx context.Context, startURLTemplate string, max int) ([]jobstreetListItem, error) {
	if max <= 0 {
		max = 50
	}
	listURL := startURLTemplate
	if strings.Contains(listURL, "%d") {
		listURL = fmt.Sprintf(listURL, 1)
	}

	c := colly.NewCollector(
		colly.AllowedDomains(s.allowedHost, s.allowedHostAlt),
	)
	_ = c.Limit(&colly.LimitRule{DomainGlob: "*jobstreet*", Parallelism: 2, RandomDelay: 750 * time.Millisecond, Delay: 400 * time.Millisecond})

	items := make([]jobstreetListItem, 0)
	seen := map[string]struct{}{}

	c.OnHTML("a", func(e *colly.HTMLElement) {
		if len(items) >= max {
			return
		}
		href := strings.TrimSpace(e.Attr("href"))
		if href == "" {
			return
		}
		if !strings.Contains(href, "/job/") && !strings.Contains(href, "/en/job/") && !strings.Contains(href, "/id/job/") {
			return
		}
		abs := e.Request.AbsoluteURL(href)
		abs = strings.TrimSpace(abs)
		if abs == "" {
			return
		}
		if _, ok := seen[abs]; ok {
			return
		}
		seen[abs] = struct{}{}
		items = append(items, jobstreetListItem{Link: abs})
	})

	var reqErr error
	c.OnError(func(r *colly.Response, err error) {
		reqErr = err
	})

	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("User-Agent", "SkillSyncScraper/0.1")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")
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
