package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"skill-sync/internal/database"

	"github.com/google/uuid"
)

type GlintsScraper struct {
	db               database.DB
	client           *http.Client
	siteBase         string
	headlessFallback bool
}

func NewGlintsScraper(db database.DB) *GlintsScraper {
	return &GlintsScraper{
		db: db,
		client: &http.Client{
			Timeout: 25 * time.Second,
		},
		siteBase: "https://glints.com",
	}
}

func (s *GlintsScraper) EnableHeadlessFallback(enable bool) {
	if s == nil {
		return
	}
	s.headlessFallback = enable
}

type glintsJobItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Location string `json:"location"`
	Company  string `json:"company"`
	URL      string `json:"url"`
	Slug     string `json:"slug"`
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
		items, err := s.fetchExplorePage(ctx, page)
		if err != nil {
			_ = logScrape(ctx, s.db, runID, "error", fmt.Sprintf("glints search page %d: %v", page, err))
			continue
		}
		_ = logScrape(ctx, s.db, runID, "info", fmt.Sprintf("glints page %d candidates=%d", page, len(items)))
		for _, it := range items {
			it := it
			jobID := strings.TrimSpace(it.ID)
			pool.Submit(func(ctx context.Context) error {
				jobURL := normalizeURL(it.URL)
				if strings.TrimSpace(jobURL) == "" {
					jobURL = s.buildJobURL(it)
				}
				if strings.TrimSpace(jobURL) == "" {
					return fmt.Errorf("empty job url")
				}

				return insertRawJob(ctx, s.db, sourceID, runID, rawJobInput{
					ExternalJobID:  jobID,
					Title:          "",
					Company:        "",
					Location:       "",
					EmploymentType: "",
					Description:    "",
					RawDescription: "",
					PostedAt:       nil,
					URL:            jobURL,
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

func (s *GlintsScraper) fetchExplorePage(ctx context.Context, page int) ([]glintsJobItem, error) {
	base := strings.TrimRight(s.siteBase, "/")
	explore := fmt.Sprintf("%s/id/opportunities/jobs/explore?country=ID&locationName=All+Cities/Provinces&page=%d", base, page)
	reqCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	body, err := httpGetWithRetry(reqCtx, s.client, explore, 2)
	if err != nil {
		return nil, err
	}
	items, err := extractGlintsJobsFromNextData(body, 30)
	if err != nil {
		items2, err2 := extractGlintsJobsFromHTML(body, 30, s.siteBase)
		if err2 == nil && len(items2) > 0 {
			return items2, nil
		}
		if s != nil && s.headlessFallback {
			items3, err3 := s.fetchExplorePageHeadless(ctx, page, 30)
			if err3 == nil && len(items3) > 0 {
				return items3, nil
			}
		}
		if err2 != nil {
			hasNext := strings.Contains(string(body), "__NEXT_DATA__")
			snippet := strings.TrimSpace(string(body))
			if len(snippet) > 240 {
				snippet = snippet[:240]
			}
			snippet = strings.Join(strings.Fields(snippet), " ")
			return nil, fmt.Errorf("%v; html fallback: %v; has_next_data=%t; snippet=%q", err, err2, hasNext, snippet)
		}
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no jobs parsed")
	}
	return items, nil
}

func (s *GlintsScraper) buildJobURL(it glintsJobItem) string {
	base := strings.TrimRight(s.siteBase, "/")
	slug := strings.TrimSpace(it.Slug)
	if slug != "" {
		id := strings.TrimSpace(it.ID)
		if id != "" {
			return base + "/id/opportunities/jobs/" + strings.TrimLeft(slug, "/") + "/" + id
		}
		return base + "/id/opportunities/jobs/" + strings.TrimLeft(slug, "/")
	}
	id := strings.TrimSpace(it.ID)
	if id != "" {
		return base + "/id/opportunities/jobs/" + id
	}
	return ""
}

func (s *GlintsScraper) fetchJobDetailHTML(ctx context.Context, jobURL string) (title string, desc string, location string, err error) {
	jobURL = strings.TrimSpace(jobURL)
	if jobURL == "" {
		return "", "", "", fmt.Errorf("empty job url")
	}
	reqCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	body, err := httpGetWithRetry(reqCtx, s.client, jobURL, 1)
	if err != nil {
		return "", "", "", err
	}
	html := string(body)
	title = extractFirstTagText(html, "<title>", "</title>")
	if strings.TrimSpace(title) == "" {
		title = extractFirstTagText(html, "<h1", "</h1>")
		title = stripHTMLTags(title)
	}
	desc = stripHTMLTags(html)
	desc = strings.TrimSpace(desc)
	if len(desc) > 50000 {
		desc = desc[:50000]
	}
	return strings.TrimSpace(title), desc, strings.TrimSpace(location), nil
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
	if err := jsonUnmarshalLoose([]byte(jsonStr), &root); err != nil {
		return nil, err
	}

	jobsArr := findFirstJobsArray(root)
	if jobsArr == nil {
		items := extractGlintsJobsFromNextDataJobPaths(jsonStr, limit, "https://glints.com")
		if len(items) > 0 {
			return items, nil
		}
		return nil, fmt.Errorf("job data not found in __NEXT_DATA__")
	}
	if limit <= 0 {
		limit = 30
	}
	out := make([]glintsJobItem, 0)
	seen := map[string]struct{}{}
	for _, item := range jobsArr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		title, _ := m["title"].(string)
		if id == "" || title == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		urlStr, _ := m["url"].(string)
		slug, _ := m["slug"].(string)
		location, _ := m["location"].(string)
		company, _ := m["company"].(string)
		out = append(out, glintsJobItem{ID: id, Title: title, URL: urlStr, Slug: slug, Location: location, Company: company})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func extractGlintsJobsFromNextDataJobPaths(jsonStr string, limit int, baseURL string) []glintsJobItem {
	if limit <= 0 {
		limit = 30
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://glints.com"
	}
	pathRe := regexp.MustCompile(`(?i)(/id/opportunities/jobs/[^\"\s]*?([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})[^\"\s]*)`)
	matches := pathRe.FindAllStringSubmatch(jsonStr, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]glintsJobItem, 0, limit)
	for _, m := range matches {
		if len(out) >= limit {
			break
		}
		if len(m) < 3 {
			continue
		}
		path := strings.TrimSpace(m[1])
		id := strings.TrimSpace(m[2])
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		out = append(out, glintsJobItem{ID: id, URL: baseURL + path})
	}
	return out
}

func extractGlintsJobsFromHTML(html []byte, limit int, baseURL string) ([]glintsJobItem, error) {
	s := string(html)
	if limit <= 0 {
		limit = 30
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://glints.com"
	}
	pathRe := regexp.MustCompile(`(?i)(?:href\s*=\s*['\"])?(https?:\/\/[^\s'\">]+)?(\/id\/opportunities\/jobs\/[^\s'\"\?#>]+)`)
	uuidRe := regexp.MustCompile(`(?i)([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})`)

	out := make([]glintsJobItem, 0)
	seen := map[string]struct{}{}
	for _, m := range pathRe.FindAllStringSubmatch(s, -1) {
		if len(out) >= limit {
			break
		}
		if len(m) < 3 {
			continue
		}
		absPrefix := strings.TrimSpace(m[1])
		path := strings.TrimSpace(m[2])
		idMatch := uuidRe.FindStringSubmatch(path)
		if len(idMatch) < 2 {
			continue
		}
		id := strings.TrimSpace(idMatch[1])
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		jobURL := ""
		if absPrefix != "" {
			jobURL = absPrefix
			if strings.HasSuffix(jobURL, "/") {
				jobURL = strings.TrimRight(jobURL, "/")
			}
			jobURL = jobURL + path
		} else {
			jobURL = baseURL + path
		}
		out = append(out, glintsJobItem{ID: id, URL: jobURL})
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("no job urls found")
	}
	return out, nil
}

func jsonUnmarshalLoose(b []byte, out *any) error {
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	return dec.Decode(out)
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

func extractFirstTagText(html string, start string, end string) string {
	idx := strings.Index(strings.ToLower(html), strings.ToLower(start))
	if idx < 0 {
		return ""
	}
	h := html[idx:]
	endIdx := strings.Index(strings.ToLower(h), strings.ToLower(end))
	if endIdx < 0 {
		return ""
	}
	chunk := h[:endIdx]
	if strings.HasPrefix(strings.ToLower(start), "<h1") {
		gt := strings.Index(chunk, ">")
		if gt >= 0 {
			chunk = chunk[gt+1:]
		}
		return chunk
	}
	chunk = strings.TrimPrefix(chunk, start)
	chunk = strings.TrimSpace(chunk)
	return chunk
}

func stripHTMLTags(s string) string {
	re := regexp.MustCompile(`(?s)<[^>]*>`)
	out := re.ReplaceAllString(s, " ")
	out = strings.ReplaceAll(out, "\u00a0", " ")
	out = strings.Join(strings.Fields(out), " ")
	return out
}
