package handler

import (
	"errors"
	"strconv"

	"skill-sync/internal/delivery/http/dto"
	"skill-sync/internal/delivery/http/middleware"
	"skill-sync/internal/pkg/response"
	"skill-sync/internal/usecase"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type JobRecommendationHandler struct {
	uc usecase.JobRecommendationUsecase
}

func NewJobRecommendationHandler(uc usecase.JobRecommendationUsecase) *JobRecommendationHandler {
	return &JobRecommendationHandler{uc: uc}
}

func (h *JobRecommendationHandler) RegisterRoutes(r fiber.Router) {
	if r == nil {
		return
	}
	grp := r.Group("/jobs")
	grp.Get("/recommendations", h.GetRecommendations)
}

func (h *JobRecommendationHandler) GetRecommendations(c fiber.Ctx) error {
	userID, ok := c.Locals(middleware.CtxUserIDKey).(uuid.UUID)
	if !ok {
		return middleware.NewAppError(fiber.StatusUnauthorized, "Unauthorized", nil, nil)
	}

	limit := parseQueryInt(c, "limit", 20)
	offset := parseQueryInt(c, "offset", 0)
	minScore := parseQueryInt(c, "min_score", 0)
	if limit > 50 {
		limit = 50
	}
	if limit < 1 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	if minScore < 0 {
		minScore = 0
	}

	items, err := h.uc.GetRecommendations(c.Context(), userID, usecase.JobRecommendationParams{
		Limit:    limit,
		Offset:   offset,
		MinScore: minScore,
	})
	if err != nil {
		return mapJobRecommendationUsecaseError(err)
	}

	out := make([]dto.JobRecommendationResponse, 0, len(items))
	for _, it := range items {
		missing := make([]dto.JobRecommendationMissingSkillItem, 0, len(it.MissingSkills))
		for _, ms := range it.MissingSkills {
			missing = append(missing, dto.JobRecommendationMissingSkillItem{
				SkillID:     ms.SkillID,
				SkillName:   ms.SkillName,
				IsMandatory: ms.IsMandatory,
			})
		}
		out = append(out, dto.JobRecommendationResponse{
			JobID:            it.JobID,
			Title:            it.Title,
			CompanyName:      it.CompanyName,
			Location:         it.Location,
			MatchScore:       it.MatchScore,
			MandatoryMissing: it.MandatoryMissing,
			MissingSkills:    missing,
		})
	}

	return response.Success(c, fiber.StatusOK, response.MessageOK, out)
}

func parseQueryInt(c fiber.Ctx, key string, defaultVal int) int {
	s := c.Query(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

func mapJobRecommendationUsecaseError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, usecase.ErrUnauthorized):
		return middleware.NewAppError(fiber.StatusUnauthorized, "Unauthorized", nil, err)
	case errors.Is(err, usecase.ErrUserSkillProfileEmpty):
		return middleware.NewAppError(fiber.StatusBadRequest, "User skill profile empty", nil, err)
	case errors.Is(err, usecase.ErrNoJobsFound):
		return middleware.NewAppError(fiber.StatusNotFound, "No jobs found", nil, err)
	case errors.Is(err, usecase.ErrInternal):
		return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, err)
	default:
		return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, err)
	}
}
