package seeder

import (
	"context"
	"fmt"

	"skill-sync/internal/database"
)

type SkillsSeeder struct{}

func (SkillsSeeder) Name() string { return "skills" }

func (SkillsSeeder) Run(ctx context.Context, db database.DB) error {
	if err := EnsureTableColumns(ctx, db, "skills", "id", "name", "category", "created_at"); err != nil {
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
		Name     string
		Category string
	}{
		{Name: "Go", Category: "Programming Language"},
		{Name: "JavaScript", Category: "Programming Language"},
		{Name: "TypeScript", Category: "Programming Language"},
		{Name: "PostgreSQL", Category: "Database"},
		{Name: "Redis", Category: "Database"},
		{Name: "Docker", Category: "DevOps"},
		{Name: "Kubernetes", Category: "DevOps"},
		{Name: "AWS", Category: "Cloud"},
		{Name: "GCP", Category: "Cloud"},
	}

	for _, it := range items {
		affected, err := tx.Exec(
			ctx,
			`INSERT INTO skills (id, name, category) VALUES (gen_random_uuid(), $1, $2) ON CONFLICT (name) DO NOTHING`,
			it.Name,
			it.Category,
		)
		if err != nil {
			return err
		}
		_ = affected
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
