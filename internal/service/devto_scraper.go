package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"skill-sync/internal/repository"
)

type DevtoScraper struct {
	client   *http.Client
	apiBase  string
	siteBase string
}

func NewDevtoScraper() *DevtoScraper {
	return &DevtoScraper{
		client: &http.Client{Timeout: 10 * time.Second},
		apiBase:  "https://dev.to",
		siteBase: "https://dev.to",
	}
}

func (s *DevtoScraper) Name() string {
	return "Dev.to Jobs"
}

type devtoListing struct {
	ID           int     `json:"id"`
	Title        string  `json:"title"`
	Category     string  `json:"category"`
	Organization string  `json:"organization_name"`
	Company      string  `json:"company_name"`
	Location     string  `json:"location"`
	PublishedAt  *string `json:"published_at"`
	URL          string  `json:"url"`
}

func (s *DevtoScraper) Search(ctx context.Context, params SearchParams) ([]repository.JobUpsert, error) {
	if s == nil {
		return nil, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(s.apiBase, "/")+"/api/listings?category=jobs&per_page=50&page=1", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SkillSyncScraper/0.1")
	req.Header.Set("Accept", "*/*")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	dec := json.NewDecoder(resp.Body)
	var items []devtoListing
	if err := dec.Decode(&items); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	out := make([]repository.JobUpsert, 0, len(items))
	for _, it := range items {
		if it.ID == 0 {
			continue
		}
		u := strings.TrimSpace(it.URL)
		if u == "" {
			continue
		}
		out = append(out, repository.JobUpsert{
			SourceName:     s.Name(),
			SourceBaseURL:  s.siteBase,
			SourceURL:      u,
			ExternalJobID:  fmt.Sprintf("%d", it.ID),
			Title:          strings.TrimSpace(it.Title),
			Company:        strings.TrimSpace(pickNonEmpty(it.Company, it.Organization)),
			Location:       strings.TrimSpace(it.Location),
			EmploymentType: strings.TrimSpace(it.Category),
			ScrapedAt:      &now,
			IsActive:       true,
		})
	}

	out = filterJobsByParams(out, params)
	return out, nil
}

func pickNonEmpty(a, b string) string {
	a = strings.TrimSpace(a)
	if a != "" {
		return a
	}
	return strings.TrimSpace(b)
}

func filterJobsByParams(in []repository.JobUpsert, params SearchParams) []repository.JobUpsert {
	titleQ := strings.ToLower(strings.TrimSpace(params.Title))
	companyQ := strings.ToLower(strings.TrimSpace(params.CompanyName))
	locQ := strings.ToLower(strings.TrimSpace(params.Location))
	skills := make([]string, 0, len(params.Skills))
	for _, s := range params.Skills {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		skills = append(skills, s)
	}

	out := make([]repository.JobUpsert, 0, len(in))
	for _, j := range in {
		if titleQ != "" && !strings.Contains(strings.ToLower(j.Title), titleQ) {
			continue
		}
		if companyQ != "" && !strings.Contains(strings.ToLower(j.Company), companyQ) {
			continue
		}
		if locQ != "" && !strings.Contains(strings.ToLower(j.Location), locQ) {
			continue
		}
		if len(skills) > 0 {
			combined := strings.ToLower(j.Title + " " + j.Description + " " + j.RawDescription)
			ok := false
			for _, s := range skills {
				if strings.Contains(combined, s) {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		out = append(out, j)
		if len(out) >= 50 {
			break
		}
	}
	return out
}
