package handler

import (
	"skill-sync/internal/pkg/response"

	"github.com/gofiber/fiber/v3"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) RegisterRoutes(r fiber.Router) {
	if r == nil {
		return
	}

	r.Get("/health", h.Get)
}

func (h *HealthHandler) Get(c fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, response.MessageOK, nil)
}
