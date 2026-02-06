package user

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("user not found")

type Repository interface {
	Create(ctx context.Context, u User) error
	GetByID(ctx context.Context, id uuid.UUID) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
}
