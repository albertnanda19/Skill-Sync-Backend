package user

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("user not found")

type Repository interface {
	CreateUser(ctx context.Context, u User) error
	GetUserByID(ctx context.Context, id uuid.UUID) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	UpdateUser(ctx context.Context, u User) error
	DeleteUser(ctx context.Context, id uuid.UUID) error
	ExistsByEmail(ctx context.Context, email string) (bool, error)

	GetProfileByUserID(ctx context.Context, userID uuid.UUID) (Profile, error)
	UpdateProfile(ctx context.Context, p Profile) error
}
