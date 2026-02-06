package scraper

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func (s *GlintsScraper) fetchExplorePageHeadless(ctx context.Context, page int, limit int) ([]glintsJobItem, error) {
	if s == nil {
		return nil, fmt.Errorf("nil scraper")
	}
	if limit <= 0 {
		limit = 30
	}

	base := strings.TrimRight(s.siteBase, "/")
	url := fmt.Sprintf("%s/id/opportunities/jobs/explore?country=ID&locationName=All+Cities/Provinces&page=%d", base, page)

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx,
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-dev-shm-usage", true),
			chromedp.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"),
		)...,
	)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	reqCtx, reqCancel := context.WithTimeout(browserCtx, 25*time.Second)
	defer reqCancel()

	var hrefs []string
	err := chromedp.Run(reqCtx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(1500*time.Millisecond),
		chromedp.EvaluateAsDevTools(`Array.from(document.querySelectorAll('a[href]'))
			.map(a => a.getAttribute('href'))
			.filter(h => h && h.includes('/id/opportunities/jobs/'))`, &hrefs),
	)
	if err != nil {
		return nil, err
	}

	uuidRe := regexp.MustCompile(`(?i)([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})`)
	seen := map[string]struct{}{}
	out := make([]glintsJobItem, 0, limit)

	for _, h := range hrefs {
		if len(out) >= limit {
			break
		}
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		m := uuidRe.FindStringSubmatch(h)
		if len(m) < 2 {
			continue
		}
		id := strings.TrimSpace(m[1])
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if strings.HasPrefix(h, "/") {
			h = base + h
		} else if strings.HasPrefix(h, "http://") || strings.HasPrefix(h, "https://") {
			// keep
		} else {
			h = base + "/" + h
		}
		out = append(out, glintsJobItem{ID: id, URL: h})
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("no job urls found (headless)")
	}
	return out, nil
}
