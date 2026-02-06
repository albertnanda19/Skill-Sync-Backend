package seeder

import (
	"context"

	"skill-sync/internal/database"
)

type Seeder interface {
	Name() string
	Run(ctx context.Context, db database.DB) error
}
