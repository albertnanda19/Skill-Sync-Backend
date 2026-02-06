package handler

import "net/http"

type UserHandler struct{}

func NewUserHandler() *UserHandler {
	return &UserHandler{}
}

func (h *UserHandler) RegisterRoutes(_ interface{}) {}

func (h *UserHandler) Handler() http.Handler {
	return nil
}
