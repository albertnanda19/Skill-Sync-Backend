package usecase

import (
	"context"

	"skill-sync/internal/domain/user"
	ucuser "skill-sync/internal/usecase/user"

	"github.com/google/uuid"
)

type UserUsecase interface {
	GetProfile(ctx context.Context, userID uuid.UUID) (ucuser.Profile, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, in ucuser.UpdateProfileInput) (ucuser.Profile, error)
}

type User struct {
	svc *ucuser.Service
}

func NewUserUsecase(users user.Repository) *User {
	return &User{svc: ucuser.NewService(users)}
}

func (u *User) GetProfile(ctx context.Context, userID uuid.UUID) (ucuser.Profile, error) {
	return u.svc.GetProfile(ctx, userID)
}

func (u *User) UpdateProfile(ctx context.Context, userID uuid.UUID, in ucuser.UpdateProfileInput) (ucuser.Profile, error) {
	return u.svc.UpdateProfile(ctx, userID, in)
}
