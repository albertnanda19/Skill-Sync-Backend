package handler

import (
	"skill-sync/internal/pkg/response"
	"skill-sync/internal/usecase"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type SkillHandler struct {
	uc usecase.SkillUsecase
}

type skillResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func NewSkillHandler(uc usecase.SkillUsecase) *SkillHandler {
	return &SkillHandler{uc: uc}
}

func (h *SkillHandler) RegisterRoutes(r fiber.Router) {
	if r == nil {
		return
	}

	grp := r.Group("/skills")
	grp.Get("/", h.List)
}

func (h *SkillHandler) List(c fiber.Ctx) error {
	items, err := h.uc.ListSkills(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, response.MessageInternalServerError, nil)
	}

	res := make([]skillResponse, 0, len(items))
	for _, it := range items {
		res = append(res, skillResponse{ID: it.ID, Name: it.Name})
	}
	return response.Success(c, fiber.StatusOK, response.MessageOK, res)
}
