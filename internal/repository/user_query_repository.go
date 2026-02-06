package repository

import (
	"context"

	"skill-sync/internal/database"

	"github.com/google/uuid"
)

type UserQueryRepository interface {
	ListUserIDs(ctx context.Context, limit, offset int) ([]uuid.UUID, error)
}

type PostgresUserQueryRepository struct {
	db database.DB
}

func NewPostgresUserQueryRepository(db database.DB) *PostgresUserQueryRepository {
	return &PostgresUserQueryRepository{db: db}
}

func (r *PostgresUserQueryRepository) ListUserIDs(ctx context.Context, limit, offset int) ([]uuid.UUID, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.Query(ctx,
		`SELECT id
		 FROM users
		 ORDER BY created_at ASC
		 LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
