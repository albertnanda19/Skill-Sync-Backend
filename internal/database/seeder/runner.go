package seeder

import (
	"context"
	"fmt"

	"skill-sync/internal/database"
)

type Runner struct {
	Seeders []Seeder
}

func (r Runner) Run(ctx context.Context, db database.DB) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}
	for _, s := range r.Seeders {
		if s == nil {
			continue
		}
		if err := s.Run(ctx, db); err != nil {
			return fmt.Errorf("seed %s: %w", s.Name(), err)
		}
	}
	return nil
}
