package handler

import (
	"log"
	"time"

	"skill-sync/internal/pkg/response"
	"skill-sync/internal/usecase"

	"github.com/gofiber/fiber/v3"
)

type PipelineStatusHandler struct {
	uc  usecase.PipelineStatusUsecase
	log *log.Logger
}

func NewPipelineStatusHandler(uc usecase.PipelineStatusUsecase, logger *log.Logger) *PipelineStatusHandler {
	if logger == nil {
		logger = log.Default()
	}
	return &PipelineStatusHandler{uc: uc, log: logger}
}

func (h *PipelineStatusHandler) RegisterRoutes(r fiber.Router) {
	if r == nil {
		return
	}
	r.Get("/pipeline/metrics", h.GetStatus)
}

func (h *PipelineStatusHandler) GetStatus(c fiber.Ctx) error {
	start := time.Now()
	if h != nil && h.log != nil {
		h.log.Printf("http_request method=%s path=%s status=started", c.Method(), c.Path())
	}

	data, err := h.uc.GetStatus(c.Context())
	if err != nil {
		if h != nil && h.log != nil {
			h.log.Printf("http_request method=%s path=%s status=error duration=%s err=%v", c.Method(), c.Path(), time.Since(start), err)
		}
		return response.Success(c, fiber.StatusOK, response.MessageOK, data)
	}

	if h != nil && h.log != nil {
		h.log.Printf("http_request method=%s path=%s status=ok duration=%s", c.Method(), c.Path(), time.Since(start))
	}
	return response.Success(c, fiber.StatusOK, response.MessageOK, data)
}
