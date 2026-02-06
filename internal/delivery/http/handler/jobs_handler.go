package handler

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"skill-sync/internal/delivery/http/dto"
	"skill-sync/internal/delivery/http/middleware"
	"skill-sync/internal/pkg/response"
	"skill-sync/internal/usecase"

	"github.com/gofiber/fiber/v3"
)

type JobsHandler struct {
	uc usecase.JobListUsecase
}

func NewJobsHandler(uc usecase.JobListUsecase) *JobsHandler {
	return &JobsHandler{uc: uc}
}

func (h *JobsHandler) HandleListJobs(c fiber.Ctx) error {
	title := c.Query("title")
	companyName := c.Query("company_name")
	location := c.Query("location")
	skills := parseSkillsQuery(c.Query("skills"))

	limit, err := parseQueryIntStrict(c, "limit", 20)
	if err != nil {
		return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, err)
	}
	offset, err := parseQueryIntStrict(c, "offset", 0)
	if err != nil {
		return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, err)
	}

	items, err := h.uc.ListJobs(c.Context(), usecase.JobListParams{
		Title:       title,
		CompanyName: companyName,
		Location:    location,
		Skills:      skills,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		return mapJobListUsecaseError(err)
	}

	out := make([]dto.JobListResponse, 0, len(items))
	for _, it := range items {
		posted := ""
		if it.PostedAt != nil {
			t := time.Time(*it.PostedAt)
			if !t.IsZero() {
				posted = t.UTC().Format(time.RFC3339)
			}
		}

		out = append(out, dto.JobListResponse{
			JobID:       it.JobID,
			Title:       it.Title,
			CompanyName: it.CompanyName,
			Location:    it.Location,
			Description: it.Description,
			Skills:      it.Skills,
			PostedDate:  posted,
		})
	}

	return response.Success(c, fiber.StatusOK, "success", out)
}

func parseQueryIntStrict(c fiber.Ctx, key string, defaultVal int) (int, error) {
	s := c.Query(key)
	if s == "" {
		return defaultVal, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func parseSkillsQuery(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func mapJobListUsecaseError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, usecase.ErrInvalidInput):
		return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, err)
	case errors.Is(err, usecase.ErrInternal):
		return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, err)
	default:
		return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, err)
	}
}
