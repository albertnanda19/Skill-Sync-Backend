package scraper

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"skill-sync/internal/database"

	"github.com/google/uuid"
)

type rawJobInput struct {
	ExternalJobID  string
	Title          string
	Company        string
	Location       string
	EmploymentType string
	Description    string
	RawDescription string
	PostedAt       *time.Time
	ScrapedAt      *time.Time
	URL            string
	IsActive       bool
}

func ensureJobSource(ctx context.Context, db database.DB, name string, baseURL string) (uuid.UUID, error) {
	if db == nil {
		return uuid.Nil, fmt.Errorf("nil db")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return uuid.Nil, fmt.Errorf("empty source name")
	}
	baseURL = strings.TrimSpace(baseURL)

	_, _ = db.Exec(ctx,
		`INSERT INTO job_sources (id, name, base_url) VALUES (gen_random_uuid(), $1, $2) ON CONFLICT (name) DO NOTHING`,
		name,
		nullableText(baseURL),
	)

	row := db.QueryRow(ctx, `SELECT id FROM job_sources WHERE name = $1 LIMIT 1`, name)
	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func createScrapeRun(ctx context.Context, db database.DB, sourceID uuid.UUID) (uuid.UUID, error) {
	if db == nil {
		return uuid.Nil, fmt.Errorf("nil db")
	}
	id := uuid.New()
	now := time.Now().UTC()
	_, err := db.Exec(ctx,
		`INSERT INTO scrape_runs (id, source_id, started_at, status) VALUES ($1,$2,$3,$4)`,
		id, sourceID, now, "running",
	)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func finishScrapeRun(ctx context.Context, db database.DB, runID uuid.UUID, status string) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}
	if runID == uuid.Nil {
		return nil
	}
	_, err := db.Exec(ctx,
		`UPDATE scrape_runs SET finished_at = $2, status = $3 WHERE id = $1`,
		runID, time.Now().UTC(), strings.TrimSpace(status),
	)
	return err
}

func logScrape(ctx context.Context, db database.DB, runID uuid.UUID, level string, message string) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}
	if runID == uuid.Nil {
		return nil
	}
	level = strings.TrimSpace(level)
	if level == "" {
		level = "info"
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return nil
	}
	_, err := db.Exec(ctx,
		`INSERT INTO scrape_logs (id, scrape_run_id, level, message) VALUES ($1,$2,$3,$4)`,
		uuid.New(), runID, level, message,
	)
	return err
}

func insertRawJob(ctx context.Context, db database.DB, sourceID uuid.UUID, runID uuid.UUID, in rawJobInput) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}
	if sourceID == uuid.Nil {
		return fmt.Errorf("nil source_id")
	}

	now := time.Now().UTC()
	scrapedAt := in.ScrapedAt
	if scrapedAt == nil {
		scrapedAt = &now
	}

	externalID := strings.TrimSpace(in.ExternalJobID)
	if externalID == "" {
		externalID = stableExternalIDFromURL(in.URL)
	}
	url := strings.TrimSpace(in.URL)
	if url != "" {
		var exists bool
		row := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM jobs WHERE source_id = $1 AND url = $2)`, sourceID, url)
		if err := row.Scan(&exists); err == nil {
			if exists {
				return nil
			}
		}
	}

	_, err := db.Exec(ctx,
		`INSERT INTO jobs (
			id, source_id, external_job_id, title, company, location, employment_type,
			description, raw_description, posted_at, scraped_at, url, is_active
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (source_id, external_job_id) DO NOTHING`,
		uuid.New(),
		sourceID,
		nullableText(externalID),
		nullableText(in.Title),
		nullableText(in.Company),
		nullableText(in.Location),
		nullableText(in.EmploymentType),
		nullableText(in.Description),
		nullableText(in.RawDescription),
		in.PostedAt,
		scrapedAt,
		nullableText(url),
		in.IsActive,
	)
	if err != nil {
		_ = logScrape(ctx, db, runID, "error", fmt.Sprintf("insert job external_id=%s url=%s: %v", externalID, in.URL, err))
		return err
	}

	return nil
}

func stableExternalIDFromURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	h := sha1.Sum([]byte(u))
	return "urlsha1-" + hex.EncodeToString(h[:])
}

func nullableText(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
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
