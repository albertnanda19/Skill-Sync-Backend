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

type MatchV2Handler struct {
	uc usecase.MatchingUsecaseV2
}

func NewMatchV2Handler(uc usecase.MatchingUsecaseV2) *MatchV2Handler {
	return &MatchV2Handler{uc: uc}
}

func (h *MatchV2Handler) RegisterRoutes(r fiber.Router) {
	if r == nil {
		return
	}
	grp := r.Group("/jobs")
	grp.Get("/:job_id/match", h.GetMatchV2)
}

func (h *MatchV2Handler) GetMatchV2(c fiber.Ctx) error {
	userID, ok := c.Locals(middleware.CtxUserIDKey).(uuid.UUID)
	if !ok {
		return middleware.NewAppError(fiber.StatusUnauthorized, "Unauthorized", nil, nil)
	}

	jobID, err := uuid.Parse(c.Params("job_id"))
	if err != nil {
		return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, err)
	}

	res, err := h.uc.CalculateMatchV2(c.Context(), userID, jobID)
	if err != nil {
		return mapMatchingV2UsecaseError(err)
	}

	out := dto.MatchingResultResponseV2{
		MatchScore:       res.MatchScore,
		MandatoryMissing: res.MandatoryMissing,
		MatchedSkills:    make([]dto.MatchSkillResponseV2, 0, len(res.MatchedSkills)),
		MissingSkills:    make([]dto.MissingSkillResponseV2, 0, len(res.MissingSkills)),
	}
	for _, ms := range res.MatchedSkills {
		out.MatchedSkills = append(out.MatchedSkills, dto.MatchSkillResponseV2{
			SkillID:           ms.SkillID,
			SkillName:         ms.SkillName,
			ScoreContribution: ms.ScoreContribution,
		})
	}
	for _, ms := range res.MissingSkills {
		out.MissingSkills = append(out.MissingSkills, dto.MissingSkillResponseV2{
			SkillID:     ms.SkillID,
			SkillName:   ms.SkillName,
			IsMandatory: ms.IsMandatory,
		})
	}

	return response.Success(c, fiber.StatusOK, response.MessageOK, out)
}

func mapMatchingV2UsecaseError(err error) error {
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
