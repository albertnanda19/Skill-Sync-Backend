package user

import (
	"context"
	"errors"
	"strings"
	"time"

	"skill-sync/internal/domain/user"

	"github.com/google/uuid"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrInternal     = errors.New("internal error")
)

type UpdateProfileInput struct {
	FullName        *string
	ExperienceLevel *string
	PreferredRoles  []string
}

type Profile struct {
	ID              uuid.UUID
	Email           string
	FullName        *string
	ExperienceLevel *string
	PreferredRoles  []string
	CreatedAt       time.Time
}

type Service struct {
	users user.Repository
}

func NewService(users user.Repository) *Service {
	return &Service{users: users}
}

func (s *Service) GetProfile(ctx context.Context, userID uuid.UUID) (Profile, error) {
	usr, err := s.users.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return Profile{}, user.ErrNotFound
		}
		return Profile{}, ErrInternal
	}

	p, err := s.users.GetProfileByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return Profile{ID: usr.ID, Email: usr.Email, PreferredRoles: []string{}, CreatedAt: usr.CreatedAt}, nil
		}
		return Profile{}, ErrInternal
	}

	return Profile{
		ID:              usr.ID,
		Email:           usr.Email,
		FullName:        p.FullName,
		ExperienceLevel: p.ExperienceLevel,
		PreferredRoles:  p.PreferredRoles,
		CreatedAt:       usr.CreatedAt,
	}, nil
}

func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, in UpdateProfileInput) (Profile, error) {
	_, err := s.users.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return Profile{}, user.ErrNotFound
		}
		return Profile{}, ErrInternal
	}

	existing, err := s.users.GetProfileByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			existing = user.Profile{ID: uuid.New(), UserID: &userID, PreferredRoles: []string{}}
		} else {
			return Profile{}, ErrInternal
		}
	}

	if in.FullName != nil {
		v := strings.TrimSpace(*in.FullName)
		if v != "" {
			existing.FullName = &v
		}
	}
	if in.ExperienceLevel != nil {
		v := strings.TrimSpace(*in.ExperienceLevel)
		if v != "" {
			existing.ExperienceLevel = &v
		}
	}
	if len(in.PreferredRoles) > 0 {
		existing.PreferredRoles = in.PreferredRoles
	}

	if existing.UserID == nil {
		existing.UserID = &userID
	}
	if existing.ID == uuid.Nil {
		existing.ID = uuid.New()
	}

	if err := s.users.UpdateProfile(ctx, existing); err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return Profile{}, user.ErrNotFound
		}
		return Profile{}, ErrInternal
	}

	return s.GetProfile(ctx, userID)
}
