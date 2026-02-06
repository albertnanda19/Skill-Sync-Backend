package user

import (
	"context"
	"errors"
	"strings"

	"skill-sync/internal/domain/user"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrInternal     = errors.New("internal error")
)

type UpdateMeInput struct {
	Email    *string
	Password *string
}

type Service struct {
	users user.Repository
}

func NewService(users user.Repository) *Service {
	return &Service{users: users}
}

func (s *Service) GetMe(ctx context.Context, userID uuid.UUID) (user.User, error) {
	usr, err := s.users.GetUserByID(ctx, userID)
	if err != nil {
		return user.User{}, ErrInternal
	}
	return sanitizeUser(usr), nil
}

func (s *Service) UpdateMe(ctx context.Context, userID uuid.UUID, in UpdateMeInput) (user.User, error) {
	usr, err := s.users.GetUserByID(ctx, userID)
	if err != nil {
		return user.User{}, ErrInternal
	}

	if in.Email != nil {
		email := normalizeEmail(*in.Email)
		if email == "" {
			return user.User{}, ErrInvalidInput
		}
		usr.Email = email
	}

	if in.Password != nil {
		pw := strings.TrimSpace(*in.Password)
		if !isValidPassword(pw) {
			return user.User{}, ErrInvalidInput
		}
		hash, err := hashPassword(pw)
		if err != nil {
			return user.User{}, ErrInternal
		}
		usr.PasswordHash = hash
	}

	if err := s.users.UpdateUser(ctx, usr); err != nil {
		return user.User{}, ErrInternal
	}

	updated, err := s.users.GetUserByID(ctx, userID)
	if err != nil {
		return user.User{}, ErrInternal
	}
	return sanitizeUser(updated), nil
}

func normalizeEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return ""
	}
	return strings.ToLower(email)
}

func isValidPassword(pw string) bool {
	pw = strings.TrimSpace(pw)
	if len(pw) < 8 {
		return false
	}
	return true
}

func sanitizeUser(u user.User) user.User {
	u.PasswordHash = ""
	return u
}

func hashPassword(pw string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return "", ErrInternal
	}
	return string(hash), nil
}
