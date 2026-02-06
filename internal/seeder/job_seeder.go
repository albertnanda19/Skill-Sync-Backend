package seeder

import (
	"context"
	"fmt"
	"strings"
	"time"

	"skill-sync/internal/database"

	"github.com/google/uuid"
)

type JobSeeder struct{}

func (JobSeeder) Name() string { return "jobs" }

func (JobSeeder) Run(ctx context.Context, db database.DB) error {
	if err := ensureTableColumns(ctx, db, "jobs",
		"id",
		"source_id",
		"external_job_id",
		"title",
		"company",
		"location",
		"employment_type",
		"description",
		"raw_description",
		"posted_at",
		"scraped_at",
		"created_at",
	); err != nil {
		return err
	}

	if err := ensureTableColumns(ctx, db, "job_sources", "id", "name", "base_url", "created_at"); err != nil {
		return err
	}

	sourceID, err := findJobSourceID(ctx, db, "LinkedIn")
	if err != nil {
		return err
	}

	now := time.Now().UTC()

	items := []struct {
		Title          string
		Company        string
		Location       string
		EmploymentType string
		Description    string
	}{
		{
			Title:          "Backend Engineer (Go)",
			Company:        "SkillSync Labs",
			Location:       "Jakarta, ID",
			EmploymentType: "Full-time",
			Description:    "Build and maintain Go services, REST APIs, and PostgreSQL-backed systems.",
		},
		{
			Title:          "Fullstack Engineer (React + Go)",
			Company:        "SkillSync Labs",
			Location:       "Bandung, ID",
			EmploymentType: "Full-time",
			Description:    "Develop web apps with React/TypeScript and backend services in Go.",
		},
		{
			Title:          "DevOps Engineer",
			Company:        "CloudKita",
			Location:       "Remote",
			EmploymentType: "Full-time",
			Description:    "Operate CI/CD, Docker, Kubernetes, and cloud infrastructure for production workloads.",
		},
		{
			Title:          "Data Engineer",
			Company:        "InsightWorks",
			Location:       "Surabaya, ID",
			EmploymentType: "Full-time",
			Description:    "Build data pipelines, manage warehouses, and optimize PostgreSQL for analytics.",
		},
		{
			Title:          "Mobile Engineer (React Native)",
			Company:        "AppForge",
			Location:       "Yogyakarta, ID",
			EmploymentType: "Contract",
			Description:    "Build cross-platform mobile apps, integrate APIs, and maintain release pipelines.",
		},
		{
			Title:          "QA Automation Engineer",
			Company:        "QualityHub",
			Location:       "Remote",
			EmploymentType: "Full-time",
			Description:    "Write automated tests for APIs and web apps, integrate tests into CI pipelines.",
		},
		{
			Title:          "Site Reliability Engineer (SRE)",
			Company:        "ScaleUp",
			Location:       "Jakarta, ID",
			EmploymentType: "Full-time",
			Description:    "Improve reliability, observability, and performance across distributed services.",
		},
		{
			Title:          "Product Engineer (TypeScript)",
			Company:        "BuildFast",
			Location:       "Remote",
			EmploymentType: "Full-time",
			Description:    "Ship product features in TypeScript with strong focus on UI quality and API integration.",
		},
	}

	for _, it := range items {
		jobID, exists, err := findJobIDByTitle(ctx, db, it.Title)
		if err != nil {
			continue
		}
		if exists {
			_ = jobID
			continue
		}

		id := uuid.New()
		externalID := buildExternalJobID(it.Title)

		_, err = db.Exec(ctx,
			`INSERT INTO jobs (
				id, source_id, external_job_id, title, company, location, employment_type,
				description, raw_description, posted_at, scraped_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT (source_id, external_job_id) DO NOTHING`,
			id,
			sourceID,
			externalID,
			it.Title,
			it.Company,
			it.Location,
			it.EmploymentType,
			it.Description,
			it.Description,
			now,
			now,
		)
		if err != nil {
			continue
		}
	}

	return nil
}

func findJobSourceID(ctx context.Context, db database.DB, sourceName string) (uuid.UUID, error) {
	row := db.QueryRow(ctx, `SELECT id FROM job_sources WHERE name = $1 LIMIT 1`, sourceName)
	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		return uuid.Nil, fmt.Errorf("find job source %s: %w", sourceName, err)
	}
	return id, nil
}

func findJobIDByTitle(ctx context.Context, db database.DB, title string) (uuid.UUID, bool, error) {
	row := db.QueryRow(ctx, `SELECT id FROM jobs WHERE title = $1 LIMIT 1`, title)
	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		return uuid.Nil, false, nil
	}
	if id == uuid.Nil {
		return uuid.Nil, false, nil
	}
	return id, true, nil
}

func buildExternalJobID(title string) string {
	s := strings.ToLower(title)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "(", "")
	s = strings.ReplaceAll(s, ")", "")
	if len(s) > 60 {
		s = s[:60]
	}
	return "seed-" + s
}

func ensureTableColumns(ctx context.Context, db database.DB, table string, columns ...string) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}
	if table == "" {
		return fmt.Errorf("empty table")
	}
	for _, col := range columns {
		if col == "" {
			return fmt.Errorf("empty column")
		}
	}

	rows, err := db.Query(
		ctx,
		`SELECT column_name FROM information_schema.columns WHERE table_schema='public' AND table_name=$1`,
		table,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	existing := map[string]struct{}{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return err
		}
		existing[c] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, col := range columns {
		if _, ok := existing[col]; !ok {
			return fmt.Errorf("schema mismatch: missing column %s.%s", table, col)
		}
	}
	return nil
}
