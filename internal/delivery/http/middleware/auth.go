package middleware

import "net/http"

type Auth struct{}

func NewAuth() *Auth {
	return &Auth{}
}

func (a *Auth) Middleware(next http.Handler) http.Handler {
	return next
}
