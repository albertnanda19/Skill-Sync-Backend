package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"skill-sync/internal/domain/user"
)

var (
	ErrEmailAlreadyRegistered = errors.New("email already registered")
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrInvalidInput           = errors.New("invalid input")
	ErrInternal               = errors.New("internal error")
)

type RegisterInput struct {
	Email    string
	Password string
}

type LoginInput struct {
	Email    string
	Password string
}

type AuthUsecase interface {
	Register(ctx context.Context, in RegisterInput) (user.User, error)
	Login(ctx context.Context, in LoginInput) (user.User, error)
}

type Service struct {
	users user.Repository
}

func NewService(users user.Repository) *Service {
	return &Service{users: users}
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (user.User, error) {
	email := normalizeEmail(in.Email)
	if email == "" {
		return user.User{}, ErrInvalidInput
	}
	if !isValidPassword(in.Password) {
		return user.User{}, ErrInvalidInput
	}

	exists, err := s.users.ExistsByEmail(ctx, email)
	if err != nil {
		return user.User{}, ErrInternal
	}
	if exists {
		return user.User{}, ErrEmailAlreadyRegistered
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return user.User{}, ErrInternal
	}

	u := user.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: string(hash),
	}

	if err := s.users.CreateUser(ctx, u); err != nil {
		exists, exErr := s.users.ExistsByEmail(ctx, email)
		if exErr == nil && exists {
			return user.User{}, ErrEmailAlreadyRegistered
		}
		return user.User{}, ErrInternal
	}

	created, err := s.users.GetUserByID(ctx, u.ID)
	if err != nil {
		return user.User{}, ErrInternal
	}
	return sanitizeUser(created), nil
}

func (s *Service) Login(ctx context.Context, in LoginInput) (user.User, error) {
	email := normalizeEmail(in.Email)
	if email == "" {
		return user.User{}, ErrInvalidCredentials
	}
	if in.Password == "" {
		return user.User{}, ErrInvalidCredentials
	}

	u, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return user.User{}, ErrInvalidCredentials
		}
		return user.User{}, ErrInternal
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)); err != nil {
		return user.User{}, ErrInvalidCredentials
	}

	return sanitizeUser(u), nil
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
