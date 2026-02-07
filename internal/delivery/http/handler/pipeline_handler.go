package handler

import (
	"skill-sync/internal/pkg/response"
	"skill-sync/internal/usecase"

	"github.com/gofiber/fiber/v3"
)

type PipelineHandler struct {
	uc usecase.PipelineUsecase
}

func NewPipelineHandler(uc usecase.PipelineUsecase) *PipelineHandler {
	return &PipelineHandler{uc: uc}
}

func (h *PipelineHandler) RegisterRoutes(r fiber.Router) {
	if r == nil {
		return
	}
	r.Get("/pipeline/status", h.GetStatus)
}

func (h *PipelineHandler) GetStatus(c fiber.Ctx) error {
	status, err := h.uc.GetStatus(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "failed to get pipeline status", nil)
	}
	return response.Success(c, fiber.StatusOK, response.MessageOK, status)
}
