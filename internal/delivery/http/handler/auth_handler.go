package handler

import (
	"errors"
	"strings"

	"skill-sync/internal/delivery/http/middleware"
	"skill-sync/internal/pkg/response"
	"skill-sync/internal/usecase"
	ucauth "skill-sync/internal/usecase/auth"

	"github.com/gofiber/fiber/v3"
)

type AuthHandler struct {
	uc usecase.AuthUsecase
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func NewAuthHandler(uc usecase.AuthUsecase) *AuthHandler {
	return &AuthHandler{uc: uc}
}

func (h *AuthHandler) RegisterRoutes(r fiber.Router) {
	if r == nil {
		return
	}

	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	r.Post("/refresh", h.Refresh)
}

func (h *AuthHandler) Register(c fiber.Ctx) error {
	var req registerRequest
	if err := c.Bind().Body(&req); err != nil {
		return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, err)
	}

	usr, access, refresh, err := h.uc.Register(c.Context(), ucauth.RegisterInput{Email: req.Email, Password: req.Password})
	if err != nil {
		return mapAuthUsecaseError(err)
	}

	data := map[string]any{
		"user":          usr,
		"access_token":  access,
		"refresh_token": refresh,
	}
	return response.Success(c, fiber.StatusOK, response.MessageOK, data)
}

func (h *AuthHandler) Login(c fiber.Ctx) error {
	var req loginRequest
	if err := c.Bind().Body(&req); err != nil {
		return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, err)
	}

	usr, access, refresh, err := h.uc.Login(c.Context(), ucauth.LoginInput{Email: req.Email, Password: req.Password})
	if err != nil {
		return mapAuthUsecaseError(err)
	}

	data := map[string]any{
		"user":          usr,
		"access_token":  access,
		"refresh_token": refresh,
	}
	return response.Success(c, fiber.StatusOK, response.MessageOK, data)
}

func (h *AuthHandler) Refresh(c fiber.Ctx) error {
	tok, ok := bearerFromAuthorizationHeader(c.Get("Authorization"))
	if !ok {
		return middleware.NewAppError(fiber.StatusUnauthorized, "Unauthorized", nil, nil)
	}

	access, refresh, err := h.uc.Refresh(c.Context(), tok)
	if err != nil {
		if errors.Is(err, usecase.ErrRefreshTokenExpired) {
			return middleware.NewAppError(fiber.StatusUnauthorized, "Refresh token expired", nil, err)
		}
		if errors.Is(err, usecase.ErrInvalidRefreshToken) {
			return middleware.NewAppError(fiber.StatusUnauthorized, "Invalid refresh token", nil, err)
		}
		if errors.Is(err, usecase.ErrUnauthorized) {
			return middleware.NewAppError(fiber.StatusUnauthorized, "Unauthorized", nil, err)
		}
		return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, err)
	}

	data := map[string]any{
		"access_token":  access,
		"refresh_token": refresh,
	}
	return response.Success(c, fiber.StatusOK, response.MessageOK, data)
}

func bearerFromAuthorizationHeader(authHeader string) (string, bool) {
	authHeader = strings.TrimSpace(authHeader)
	if authHeader == "" {
		return "", false
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		return "", false
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	tok := strings.TrimSpace(parts[1])
	if tok == "" {
		return "", false
	}
	return tok, true
}

func mapAuthUsecaseError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, ucauth.ErrEmailAlreadyRegistered):
		return middleware.NewAppError(fiber.StatusConflict, "Email already registered", nil, err)
	case errors.Is(err, ucauth.ErrInvalidCredentials):
		return middleware.NewAppError(fiber.StatusUnauthorized, "Unauthorized", nil, err)
	case errors.Is(err, ucauth.ErrInvalidInput):
		return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, err)
	case errors.Is(err, ucauth.ErrInternal):
		return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, err)
	default:
		return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, err)
	}
}
