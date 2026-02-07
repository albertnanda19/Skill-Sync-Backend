package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"skill-sync/internal/repository"
)

type GlintsScraper struct {
	client   *http.Client
	siteBase string
}

func NewGlintsScraper() *GlintsScraper {
	return &GlintsScraper{
		client:   &http.Client{Timeout: 10 * time.Second},
		siteBase: "https://glints.com",
	}
}

func (s *GlintsScraper) Name() string {
	return "Glints"
}

type glintsJobItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Location string `json:"location"`
	Company  string `json:"company"`
	URL      string `json:"url"`
	Slug     string `json:"slug"`
}

func (s *GlintsScraper) Search(ctx context.Context, params SearchParams) ([]repository.JobUpsert, error) {
	if s == nil {
		return nil, nil
	}

	url := strings.TrimRight(s.siteBase, "/") + "/id/opportunities/jobs/explore?country=ID&locationName=All+Cities/Provinces&page=1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SkillSyncScraper/0.1")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	bodyBytes, err := readAllLimit(resp.Body, 5<<20)
	if err != nil {
		return nil, err
	}

	items, err := extractGlintsJobsFromNextData(bodyBytes, 50)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	out := make([]repository.JobUpsert, 0, len(items))
	for _, it := range items {
		jobURL := strings.TrimSpace(it.URL)
		if jobURL != "" && strings.HasPrefix(jobURL, "/") {
			jobURL = strings.TrimRight(s.siteBase, "/") + jobURL
		}
		if jobURL == "" {
			continue
		}
		out = append(out, repository.JobUpsert{
			SourceName:    s.Name(),
			SourceBaseURL: s.siteBase,
			SourceURL:     jobURL,
			ExternalJobID: strings.TrimSpace(it.ID),
			Title:         strings.TrimSpace(it.Title),
			Company:       strings.TrimSpace(it.Company),
			Location:      strings.TrimSpace(it.Location),
			ScrapedAt:     &now,
			IsActive:      true,
		})
	}

	out = filterJobsByParams(out, params)
	return out, nil
}

func extractGlintsJobsFromNextData(html []byte, limit int) ([]glintsJobItem, error) {
	s := string(html)
	re := regexp.MustCompile(`(?s)<script[^>]+id="__NEXT_DATA__"[^>]*>(.*?)</script>`)
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return nil, fmt.Errorf("__NEXT_DATA__ not found")
	}
	jsonStr := strings.TrimSpace(m[1])
	if jsonStr == "" {
		return nil, fmt.Errorf("empty __NEXT_DATA__")
	}

	var root any
	dec := json.NewDecoder(strings.NewReader(jsonStr))
	dec.UseNumber()
	if err := dec.Decode(&root); err != nil {
		return nil, err
	}

	arr := findFirstJobsArray(root)
	if arr == nil {
		return nil, fmt.Errorf("job data not found in __NEXT_DATA__")
	}
	if limit <= 0 {
		limit = 50
	}

	out := make([]glintsJobItem, 0, limit)
	seen := map[string]struct{}{}
	for _, it := range arr {
		mm, ok := it.(map[string]any)
		if !ok {
			continue
		}
		id, _ := mm["id"].(string)
		title, _ := mm["title"].(string)
		if strings.TrimSpace(id) == "" || strings.TrimSpace(title) == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		urlStr, _ := mm["url"].(string)
		slug, _ := mm["slug"].(string)
		location, _ := mm["location"].(string)
		company, _ := mm["company"].(string)
		out = append(out, glintsJobItem{ID: id, Title: title, URL: urlStr, Slug: slug, Location: location, Company: company})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func walkAny(v any, visitMap func(m map[string]any)) {
	switch x := v.(type) {
	case map[string]any:
		visitMap(x)
		for _, vv := range x {
			walkAny(vv, visitMap)
		}
	case []any:
		for _, vv := range x {
			walkAny(vv, visitMap)
		}
	}
}

func findFirstJobsArray(root any) []any {
	var found []any
	walkAny(root, func(m map[string]any) {
		if found != nil {
			return
		}
		v, ok := m["jobs"]
		if !ok {
			return
		}
		arr, ok := v.([]any)
		if !ok {
			return
		}
		for _, it := range arr {
			mm, ok := it.(map[string]any)
			if !ok {
				continue
			}
			id, _ := mm["id"].(string)
			title, _ := mm["title"].(string)
			if id != "" && title != "" {
				found = arr
				return
			}
		}
	})
	return found
}

func readAllLimit(r io.Reader, max int64) ([]byte, error) {
	lr := &io.LimitedReader{R: r, N: max}
	b, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if lr.N <= 0 {
		return nil, fmt.Errorf("response too large")
	}
	return b, nil
}
