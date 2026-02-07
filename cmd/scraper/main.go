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
	py "skill-sync/internal/infrastructure/scraper"
)

func main() {
	_ = flag.String("source", "all", "legacy (ignored)")
	_ = flag.Int("pages", 2, "legacy (ignored)")
	_ = flag.Int("workers", 6, "legacy (ignored)")
	_ = flag.String("jobstreet_url_template", "", "legacy (ignored)")
	_ = flag.String("company_targets", "", "legacy (ignored)")
	_ = flag.Bool("glints_headless", false, "legacy (ignored)")

	query := flag.String("query", "", "job search query")
	location := flag.String("location", "", "job location")
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := py.NewScraperClient(cfg.ScraperBaseURL, log.Default())
	if client == nil {
		log.Fatalf("SCRAPER_BASE_URL is not configured")
	}

	q := strings.TrimSpace(*query)
	loc := strings.TrimSpace(*location)
	if q == "" && loc == "" {
		log.Fatalf("provide -query and/or -location")
	}

	taskID, err := client.TriggerScrape(ctx, q, loc)
	if err != nil {
		log.Fatalf("trigger scrape failed: %v", err)
	}
	log.Printf("scrape triggered task_id=%s", taskID)
}
