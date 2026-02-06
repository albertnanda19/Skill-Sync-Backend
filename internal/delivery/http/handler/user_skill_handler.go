package handler

import (
	"errors"

	"skill-sync/internal/delivery/http/dto"
	"skill-sync/internal/delivery/http/middleware"
	"skill-sync/internal/pkg/response"
	"skill-sync/internal/usecase"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type UserSkillHandler struct {
	uc usecase.UserSkillUsecase
}

type addUserSkillRequest struct {
	SkillID          uuid.UUID `json:"skill_id"`
	ProficiencyLevel int       `json:"proficiency_level"`
	YearsExperience  int       `json:"years_experience"`
}

type updateUserSkillRequest struct {
	ProficiencyLevel int `json:"proficiency_level"`
	YearsExperience  int `json:"years_experience"`
}

func NewUserSkillHandler(uc usecase.UserSkillUsecase) *UserSkillHandler {
	return &UserSkillHandler{uc: uc}
}

func (h *UserSkillHandler) RegisterRoutes(r fiber.Router) {
	if r == nil {
		return
	}

	grp := r.Group("/me/skills")
	grp.Get("/", h.List)
	grp.Post("/", h.Add)
	grp.Put("/:id", h.Update)
	grp.Delete("/:id", h.Delete)
}

func (h *UserSkillHandler) List(c fiber.Ctx) error {
	userID, ok := c.Locals(middleware.CtxUserIDKey).(uuid.UUID)
	if !ok {
		return middleware.NewAppError(fiber.StatusUnauthorized, "Unauthorized", nil, nil)
	}

	items, err := h.uc.ListUserSkills(c.Context(), userID)
	if err != nil {
		return mapUserSkillUsecaseError(err)
	}

	res := make([]dto.UserSkillResponse, 0, len(items))
	for _, it := range items {
		res = append(res, dto.UserSkillResponse{
			ID:               it.ID,
			SkillID:          it.SkillID,
			SkillName:        it.SkillName,
			ProficiencyLevel: it.ProficiencyLevel,
			YearsExperience:  it.YearsExperience,
		})
	}
	return response.Success(c, fiber.StatusOK, response.MessageOK, res)
}

func (h *UserSkillHandler) Add(c fiber.Ctx) error {
	userID, ok := c.Locals(middleware.CtxUserIDKey).(uuid.UUID)
	if !ok {
		return middleware.NewAppError(fiber.StatusUnauthorized, "Unauthorized", nil, nil)
	}

	var req addUserSkillRequest
	if err := c.Bind().Body(&req); err != nil {
		return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, err)
	}

	created, err := h.uc.AddUserSkill(c.Context(), userID, usecase.AddUserSkillInput{
		SkillID:          req.SkillID,
		ProficiencyLevel: req.ProficiencyLevel,
		YearsExperience:  req.YearsExperience,
	})
	if err != nil {
		return mapUserSkillUsecaseError(err)
	}

	res := dto.UserSkillResponse{
		ID:               created.ID,
		SkillID:          created.SkillID,
		SkillName:        created.SkillName,
		ProficiencyLevel: created.ProficiencyLevel,
		YearsExperience:  created.YearsExperience,
	}
	return response.Success(c, fiber.StatusOK, response.MessageOK, res)
}

func (h *UserSkillHandler) Update(c fiber.Ctx) error {
	userID, ok := c.Locals(middleware.CtxUserIDKey).(uuid.UUID)
	if !ok {
		return middleware.NewAppError(fiber.StatusUnauthorized, "Unauthorized", nil, nil)
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, err)
	}

	var req updateUserSkillRequest
	if err := c.Bind().Body(&req); err != nil {
		return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, err)
	}

	updated, err := h.uc.UpdateUserSkill(c.Context(), userID, id, usecase.UpdateUserSkillInput{
		ProficiencyLevel: req.ProficiencyLevel,
		YearsExperience:  req.YearsExperience,
	})
	if err != nil {
		return mapUserSkillUsecaseError(err)
	}

	res := dto.UserSkillResponse{
		ID:               updated.ID,
		SkillID:          updated.SkillID,
		SkillName:        updated.SkillName,
		ProficiencyLevel: updated.ProficiencyLevel,
		YearsExperience:  updated.YearsExperience,
	}
	return response.Success(c, fiber.StatusOK, response.MessageOK, res)
}

func (h *UserSkillHandler) Delete(c fiber.Ctx) error {
	userID, ok := c.Locals(middleware.CtxUserIDKey).(uuid.UUID)
	if !ok {
		return middleware.NewAppError(fiber.StatusUnauthorized, "Unauthorized", nil, nil)
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, err)
	}

	if err := h.uc.DeleteUserSkill(c.Context(), userID, id); err != nil {
		return mapUserSkillUsecaseError(err)
	}
	return response.Success(c, fiber.StatusOK, response.MessageOK, nil)
}

func mapUserSkillUsecaseError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, usecase.ErrInvalidProficiencyLevel):
		return middleware.NewAppError(fiber.StatusBadRequest, "Invalid proficiency level", nil, err)
	case errors.Is(err, usecase.ErrSkillAlreadyExists):
		return middleware.NewAppError(fiber.StatusConflict, "Skill already exists", nil, err)
	case errors.Is(err, usecase.ErrSkillNotFound):
		return middleware.NewAppError(fiber.StatusNotFound, "Skill not found", nil, err)
	case errors.Is(err, usecase.ErrInvalidInput):
		return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, err)
	case errors.Is(err, usecase.ErrInternal):
		return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, err)
	default:
		return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, err)
	}
}
