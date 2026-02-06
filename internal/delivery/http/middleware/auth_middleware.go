package middleware

import (
	"errors"
	"strings"

	"skill-sync/internal/pkg/jwt"

	"github.com/gofiber/fiber/v3"
)

const (
	CtxUserIDKey = "user_id"
	CtxEmailKey  = "email"
)

type AuthMiddleware struct {
	jwt jwt.Service
}

func NewAuthMiddleware(jwtSvc jwt.Service) *AuthMiddleware {
	return &AuthMiddleware{jwt: jwtSvc}
}

func (m *AuthMiddleware) Middleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		token, ok := bearerTokenFromHeader(c.Get("Authorization"))
		if !ok {
			return NewAppError(fiber.StatusUnauthorized, "Unauthorized", nil, nil)
		}

		claims, err := m.jwt.ValidateToken(token)
		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				return NewAppError(fiber.StatusUnauthorized, "Token expired", nil, err)
			}
			return NewAppError(fiber.StatusUnauthorized, "Invalid token", nil, err)
		}

		if claims.TokenType != jwt.TokenTypeAccess || m.jwt.IsRefreshToken(claims) {
			return NewAppError(fiber.StatusUnauthorized, "Invalid token", nil, nil)
		}

		c.Locals(CtxUserIDKey, claims.UserID)
		c.Locals(CtxEmailKey, claims.Email)

		return c.Next()
	}
}

func bearerTokenFromHeader(authHeader string) (string, bool) {
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

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", false
	}

	return token, true
}
