package main

import (
	"context"
	"flag"
	"log"
	"time"

	"skill-sync/internal/app"
	"skill-sync/internal/config"
	"skill-sync/internal/database/migration"
	"skill-sync/internal/scraper"
)

func main() {
	source := flag.String("source", "all", "all|jobstreet|devto")
	pages := flag.Int("pages", 2, "number of pages to scrape")
	workers := flag.Int("workers", 6, "number of concurrent workers")
	jobstreetTemplate := flag.String("jobstreet_url_template", "https://www.jobstreet.co.id/id/job-search/jobs?sort=createdAt&page=%d", "JobStreet listing URL template with %d page placeholder")
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
		if err := scraper.NewJobStreetScraper(c.DB).Scrape(ctx, *jobstreetTemplate, *pages, *workers); err != nil {
			log.Fatalf("jobstreet scrape failed: %v", err)
		}
	case "devto":
		if err := scraper.NewDevtoScraper(c.DB).Scrape(ctx, *pages, *workers); err != nil {
			log.Fatalf("devto scrape failed: %v", err)
		}
	case "all":
		if err := scraper.NewDevtoScraper(c.DB).Scrape(ctx, *pages, *workers); err != nil {
			log.Printf("devto scrape failed: %v", err)
		}
		if err := scraper.NewJobStreetScraper(c.DB).Scrape(ctx, *jobstreetTemplate, *pages, *workers); err != nil {
			log.Printf("jobstreet scrape failed: %v", err)
		}
	default:
		log.Fatalf("invalid -source: %s", *source)
	}
}
