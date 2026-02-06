package usecase

import (
	"context"
	"errors"

	"skill-sync/internal/domain/user"
	"skill-sync/internal/pkg/jwt"
	ucauth "skill-sync/internal/usecase/auth"
)

var (
	ErrUnauthorized        = errors.New("unauthorized")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrRefreshTokenExpired = errors.New("refresh token expired")
	ErrInternal            = errors.New("internal error")
)

type AuthUsecase interface {
	Register(ctx context.Context, in ucauth.RegisterInput) (user.User, string, string, error)
	Login(ctx context.Context, in ucauth.LoginInput) (user.User, string, string, error)
	Refresh(ctx context.Context, refreshToken string) (string, string, error)
}

type Auth struct {
	authSvc *ucauth.Service
	users   user.Repository
	jwt     jwt.Service
}

func NewAuthUsecase(users user.Repository, jwtSvc jwt.Service) *Auth {
	return &Auth{authSvc: ucauth.NewService(users), users: users, jwt: jwtSvc}
}

func (u *Auth) Register(ctx context.Context, in ucauth.RegisterInput) (user.User, string, string, error) {
	usr, err := u.authSvc.Register(ctx, in)
	if err != nil {
		return user.User{}, "", "", err
	}

	access, err := u.jwt.GenerateAccessToken(usr.ID, usr.Email)
	if err != nil {
		return user.User{}, "", "", ErrInternal
	}
	refresh, err := u.jwt.GenerateRefreshToken(usr.ID)
	if err != nil {
		return user.User{}, "", "", ErrInternal
	}

	return usr, access, refresh, nil
}

func (u *Auth) Login(ctx context.Context, in ucauth.LoginInput) (user.User, string, string, error) {
	usr, err := u.authSvc.Login(ctx, in)
	if err != nil {
		return user.User{}, "", "", err
	}

	access, err := u.jwt.GenerateAccessToken(usr.ID, usr.Email)
	if err != nil {
		return user.User{}, "", "", ErrInternal
	}
	refresh, err := u.jwt.GenerateRefreshToken(usr.ID)
	if err != nil {
		return user.User{}, "", "", ErrInternal
	}

	return usr, access, refresh, nil
}

func (u *Auth) Refresh(ctx context.Context, refreshToken string) (string, string, error) {
	if refreshToken == "" {
		return "", "", ErrUnauthorized
	}

	claims, err := u.jwt.ValidateToken(refreshToken)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", "", ErrRefreshTokenExpired
		}
		return "", "", ErrInvalidRefreshToken
	}

	if !u.jwt.IsRefreshToken(claims) || claims.TokenType != jwt.TokenTypeRefresh {
		return "", "", ErrInvalidRefreshToken
	}

	usr, err := u.users.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return "", "", ErrInternal
	}

	access, err := u.jwt.GenerateAccessToken(usr.ID, usr.Email)
	if err != nil {
		return "", "", ErrInternal
	}
	newRefresh, err := u.jwt.GenerateRefreshToken(usr.ID)
	if err != nil {
		return "", "", ErrInternal
	}

	return access, newRefresh, nil
}
