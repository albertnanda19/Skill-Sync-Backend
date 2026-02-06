package repository

import (
	"context"
	"database/sql"
	"errors"

	"skill-sync/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrJobNotFound = errors.New("job not found")
)

type JobRepository interface {
	ExistsByID(ctx context.Context, jobID uuid.UUID) (bool, error)
}

type PostgresJobRepository struct {
	db database.DB
}

func NewPostgresJobRepository(db database.DB) *PostgresJobRepository {
	return &PostgresJobRepository{db: db}
}

func (r *PostgresJobRepository) ExistsByID(ctx context.Context, jobID uuid.UUID) (bool, error) {
	var exists bool
	row := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM jobs WHERE id = $1)`, jobID)
	if err := row.Scan(&exists); err != nil {
		if err == sql.ErrNoRows || errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return exists, nil
}
