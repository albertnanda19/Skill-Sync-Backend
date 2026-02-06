package main

import (
	"context"
	"flag"
	"log"
	"strings"
	"time"

	"skill-sync/internal/app"
	"skill-sync/internal/config"
	"skill-sync/internal/database/migration"
	"skill-sync/internal/scraper"
)

func main() {
	source := flag.String("source", "all", "all|jobstreet|devto|glints|company")
	pages := flag.Int("pages", 2, "number of pages to scrape")
	workers := flag.Int("workers", 6, "number of concurrent workers")
	jobstreetTemplate := flag.String("jobstreet_url_template", "https://www.jobstreet.co.id/id/job-search/jobs?sort=createdAt&page=%d", "JobStreet listing URL template with %d page placeholder")
	companyTargets := flag.String("company_targets", "", "comma-separated company targets as name|listURL (example: Acme|https://acme.com/careers)")
	glintsHeadless := flag.Bool("glints_headless", false, "enable headless browser fallback for Glints (requires Chrome/Chromium)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	c, err := app.NewContainer(cfg)
	if err != nil {
		log.Fatalf("failed to init container: %v", err)
	}
	defer func() {
		_ = c.Close()
	}()

	migCtx, migCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer migCancel()
	r := migration.Runner{Dir: "migrations"}
	if err := r.Run(migCtx, c.DB.SQLDB()); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	switch *source {
	case "jobstreet":
		log.Printf("scrape start source=jobstreet pages=%d workers=%d", *pages, *workers)
		started := time.Now()
		if err := scraper.NewJobStreetScraper(c.DB).Scrape(ctx, *jobstreetTemplate, *pages, *workers); err != nil {
			log.Fatalf("jobstreet scrape failed: %v", err)
		}
		log.Printf("scrape finished source=jobstreet duration=%s", time.Since(started))
	case "devto":
		log.Printf("scrape start source=devto pages=%d workers=%d", *pages, *workers)
		started := time.Now()
		if err := scraper.NewDevtoScraper(c.DB).Scrape(ctx, *pages, *workers); err != nil {
			log.Fatalf("devto scrape failed: %v", err)
		}
		log.Printf("scrape finished source=devto duration=%s", time.Since(started))
	case "glints":
		log.Printf("scrape start source=glints pages=%d workers=%d", *pages, *workers)
		started := time.Now()
		gs := scraper.NewGlintsScraper(c.DB)
		gs.EnableHeadlessFallback(*glintsHeadless)
		if err := gs.Scrape(ctx, *pages, *workers); err != nil {
			log.Fatalf("glints scrape failed: %v", err)
		}
		log.Printf("scrape finished source=glints duration=%s", time.Since(started))
	case "company":
		log.Printf("scrape start source=company pages=%d workers=%d", *pages, *workers)
		started := time.Now()
		if err := scraper.NewCompanyScraper(c.DB).Scrape(ctx, parseCompanyTargets(*companyTargets), *pages, *workers); err != nil {
			log.Fatalf("company scrape failed: %v", err)
		}
		log.Printf("scrape finished source=company duration=%s", time.Since(started))
	case "all":
		log.Printf("scrape start source=all pages=%d workers=%d", *pages, *workers)
		if err := scraper.NewDevtoScraper(c.DB).Scrape(ctx, *pages, *workers); err != nil {
			log.Printf("devto scrape failed: %v", err)
		} else {
			log.Printf("scrape finished source=devto")
		}
		if err := scraper.NewJobStreetScraper(c.DB).Scrape(ctx, *jobstreetTemplate, *pages, *workers); err != nil {
			log.Printf("jobstreet scrape failed: %v", err)
		} else {
			log.Printf("scrape finished source=jobstreet")
		}
		gs := scraper.NewGlintsScraper(c.DB)
		gs.EnableHeadlessFallback(*glintsHeadless)
		if err := gs.Scrape(ctx, *pages, *workers); err != nil {
			log.Printf("glints scrape failed: %v", err)
		} else {
			log.Printf("scrape finished source=glints")
		}
		if err := scraper.NewCompanyScraper(c.DB).Scrape(ctx, parseCompanyTargets(*companyTargets), *pages, *workers); err != nil {
			log.Printf("company scrape failed: %v", err)
		} else {
			log.Printf("scrape finished source=company")
		}
		log.Printf("scrape finished source=all")
	default:
		log.Fatalf("invalid -source: %s", *source)
	}
}

func parseCompanyTargets(raw string) []scraper.CompanyCareersTarget {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]scraper.CompanyCareersTarget, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		name, listURL, ok := strings.Cut(p, "|")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		listURL = strings.TrimSpace(listURL)
		if name == "" || listURL == "" {
			continue
		}
		out = append(out, scraper.CompanyCareersTarget{SourceName: name, BaseURL: listURL, ListURL: listURL})
	}
	return out
}
