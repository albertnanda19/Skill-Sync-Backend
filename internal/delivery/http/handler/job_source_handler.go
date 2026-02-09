package handler

import (
	"errors"

	"skill-sync/internal/delivery/http/dto"
	"skill-sync/internal/delivery/http/middleware"
	"skill-sync/internal/pkg/response"
	"skill-sync/internal/usecase"

	"github.com/gofiber/fiber/v3"
)

type JobSourceHandler struct {
	uc usecase.JobSourceUsecase
}

func NewJobSourceHandler(uc usecase.JobSourceUsecase) *JobSourceHandler {
	return &JobSourceHandler{uc: uc}
}

func (h *JobSourceHandler) RegisterRoutes(r fiber.Router) {
	if r == nil {
		return
	}
	if h == nil {
		return
	}

	r.Get("/job-sources", h.List)
}

func (h *JobSourceHandler) List(c fiber.Ctx) error {
	items, err := h.uc.ListJobSources(c.Context())
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrInvalidInput):
			return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, err)
		case errors.Is(err, usecase.ErrInternal):
			return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, err)
		default:
			return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, err)
		}
	}

	out := make([]dto.JobSourceResponse, 0, len(items))
	for _, it := range items {
		out = append(out, dto.JobSourceResponse{ID: it.ID, Name: it.Name, BaseURL: it.BaseURL})
	}
	return response.Success(c, fiber.StatusOK, response.MessageOK, out)
}
