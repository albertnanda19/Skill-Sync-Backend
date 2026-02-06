package jwt

import (
	"errors"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

var (
	ErrTokenExpired = errors.New("token expired")
	ErrTokenInvalid = errors.New("token invalid")
)

type Claims struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email,omitempty"`
	TokenType string    `json:"token_type"`

	IssuedAt  time.Time `json:"issued_at"`
	ExpiredAt time.Time `json:"expired_at"`

	jwtlib.RegisteredClaims
}

type Service interface {
	GenerateAccessToken(userID uuid.UUID, email string) (string, error)
	GenerateRefreshToken(userID uuid.UUID) (string, error)
	ValidateToken(tokenString string) (Claims, error)
	IsRefreshToken(claims Claims) bool
}

type HMACService struct {
	accessSecret  []byte
	refreshSecret []byte

	accessExpiresIn  time.Duration
	refreshExpiresIn time.Duration

	now func() time.Time
}

func NewHMACService(accessSecret, refreshSecret string, accessExpiresIn, refreshExpiresIn time.Duration) *HMACService {
	return &HMACService{
		accessSecret:      []byte(accessSecret),
		refreshSecret:     []byte(refreshSecret),
		accessExpiresIn:   accessExpiresIn,
		refreshExpiresIn:  refreshExpiresIn,
		now:               time.Now,
	}
}

func (s *HMACService) GenerateAccessToken(userID uuid.UUID, email string) (string, error) {
	return s.generate(TokenTypeAccess, userID, email)
}

func (s *HMACService) GenerateRefreshToken(userID uuid.UUID) (string, error) {
	return s.generate(TokenTypeRefresh, userID, "")
}

func (s *HMACService) ValidateToken(tokenString string) (Claims, error) {
	var lastErr error

	claims, err := s.validateWithSecret(tokenString, s.accessSecret)
	if err == nil {
		return claims, nil
	}
	lastErr = err

	claims, err = s.validateWithSecret(tokenString, s.refreshSecret)
	if err == nil {
		return claims, nil
	}

	if errors.Is(lastErr, ErrTokenExpired) || errors.Is(err, ErrTokenExpired) {
		return Claims{}, ErrTokenExpired
	}
	return Claims{}, ErrTokenInvalid
}

func (s *HMACService) IsRefreshToken(claims Claims) bool {
	return claims.TokenType == TokenTypeRefresh
}

func (s *HMACService) generate(tokenType string, userID uuid.UUID, email string) (string, error) {
	now := s.now()
	secret, expIn, err := s.secretAndExpiry(tokenType)
	if err != nil {
		return "", err
	}

	exp := now.Add(expIn)

	c := Claims{
		UserID:    userID,
		Email:     email,
		TokenType: tokenType,
		IssuedAt:  now.UTC(),
		ExpiredAt: exp.UTC(),
		RegisteredClaims: jwtlib.RegisteredClaims{
			IssuedAt:  jwtlib.NewNumericDate(now.UTC()),
			ExpiresAt: jwtlib.NewNumericDate(exp.UTC()),
			Subject:   userID.String(),
		},
	}

	t := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, c)
	return t.SignedString(secret)
}

func (s *HMACService) validateWithSecret(tokenString string, secret []byte) (Claims, error) {
	p := jwtlib.NewParser(jwtlib.WithValidMethods([]string{jwtlib.SigningMethodHS256.Alg()}))

	var c Claims
	tok, err := p.ParseWithClaims(tokenString, &c, func(token *jwtlib.Token) (any, error) {
		return secret, nil
	})
	if err != nil {
		if errors.Is(err, jwtlib.ErrTokenExpired) {
			return Claims{}, ErrTokenExpired
		}
		return Claims{}, ErrTokenInvalid
	}
	if tok == nil || !tok.Valid {
		return Claims{}, ErrTokenInvalid
	}

	now := s.now().UTC()
	if !c.ExpiredAt.IsZero() && now.After(c.ExpiredAt.UTC()) {
		return Claims{}, ErrTokenExpired
	}

	if c.TokenType != TokenTypeAccess && c.TokenType != TokenTypeRefresh {
		return Claims{}, ErrTokenInvalid
	}

	return c, nil
}

func (s *HMACService) secretAndExpiry(tokenType string) ([]byte, time.Duration, error) {
	switch tokenType {
	case TokenTypeAccess:
		if len(s.accessSecret) == 0 || s.accessExpiresIn <= 0 {
			return nil, 0, ErrTokenInvalid
		}
		return s.accessSecret, s.accessExpiresIn, nil
	case TokenTypeRefresh:
		if len(s.refreshSecret) == 0 || s.refreshExpiresIn <= 0 {
			return nil, 0, ErrTokenInvalid
		}
		return s.refreshSecret, s.refreshExpiresIn, nil
	default:
		return nil, 0, ErrTokenInvalid
	}
}
