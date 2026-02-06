package seeder

import (
	"context"
	"fmt"

	"skill-sync/internal/database"
)

type JobSourcesSeeder struct{}

func (JobSourcesSeeder) Name() string { return "job_sources" }

func (JobSourcesSeeder) Run(ctx context.Context, db database.DB) error {
	if err := EnsureTableColumns(ctx, db, "job_sources", "id", "name", "base_url", "created_at"); err != nil {
		return err
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	items := []struct {
		Name    string
		BaseURL string
	}{
		{Name: "LinkedIn", BaseURL: "https://www.linkedin.com/jobs"},
		{Name: "Glints", BaseURL: "https://glints.com"},
		{Name: "Kalibrr", BaseURL: "https://www.kalibrr.com"},
		{Name: "Indeed", BaseURL: "https://www.indeed.com"},
	}

	for _, it := range items {
		_, err := tx.Exec(
			ctx,
			`INSERT INTO job_sources (id, name, base_url) VALUES (gen_random_uuid(), $1, $2) ON CONFLICT (name) DO NOTHING`,
			it.Name,
			it.BaseURL,
		)
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
