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

type MatchHandler struct {
	uc usecase.MatchingUsecase
}

func NewMatchHandler(uc usecase.MatchingUsecase) *MatchHandler {
	return &MatchHandler{uc: uc}
}

func (h *MatchHandler) RegisterRoutes(r fiber.Router) {
	if r == nil {
		return
	}
	grp := r.Group("/jobs")
	grp.Get("/:job_id/match", h.GetMatch)
}

func (h *MatchHandler) GetMatch(c fiber.Ctx) error {
	userID, ok := c.Locals(middleware.CtxUserIDKey).(uuid.UUID)
	if !ok {
		return middleware.NewAppError(fiber.StatusUnauthorized, "Unauthorized", nil, nil)
	}

	jobID, err := uuid.Parse(c.Params("job_id"))
	if err != nil {
		return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, err)
	}

	res, err := h.uc.CalculateMatch(c.Context(), userID, jobID)
	if err != nil {
		return mapMatchingUsecaseError(err)
	}

	out := dto.MatchingResultResponse{
		MatchScore:       res.MatchScore,
		MandatoryMissing: res.MandatoryMissing,
		MatchedSkills:    make([]dto.MatchSkillResponse, 0, len(res.MatchedSkills)),
		MissingSkills:    make([]dto.MissingSkillResponse, 0, len(res.MissingSkills)),
	}
	for _, ms := range res.MatchedSkills {
		out.MatchedSkills = append(out.MatchedSkills, dto.MatchSkillResponse{
			SkillID:           ms.SkillID,
			SkillName:         ms.SkillName,
			ScoreContribution: ms.ScoreContribution,
		})
	}
	for _, ms := range res.MissingSkills {
		out.MissingSkills = append(out.MissingSkills, dto.MissingSkillResponse{
			SkillID:     ms.SkillID,
			SkillName:   ms.SkillName,
			IsMandatory: ms.IsMandatory,
		})
	}

	return response.Success(c, fiber.StatusOK, response.MessageOK, out)
}

func mapMatchingUsecaseError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, usecase.ErrJobNotFound):
		return middleware.NewAppError(fiber.StatusNotFound, "Job not found", nil, err)
	case errors.Is(err, usecase.ErrUserSkillProfileEmpty):
		return middleware.NewAppError(fiber.StatusBadRequest, "User skill profile empty", nil, err)
	case errors.Is(err, usecase.ErrUnauthorized):
		return middleware.NewAppError(fiber.StatusUnauthorized, "Unauthorized", nil, err)
	case errors.Is(err, usecase.ErrInternal):
		return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, err)
	default:
		return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, err)
	}
}
