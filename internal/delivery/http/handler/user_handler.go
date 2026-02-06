package handler

import (
	"errors"

	"skill-sync/internal/delivery/http/dto"
	"skill-sync/internal/delivery/http/middleware"
	"skill-sync/internal/domain/user"
	"skill-sync/internal/pkg/response"
	"skill-sync/internal/usecase"
	useruc "skill-sync/internal/usecase/user"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type UserHandler struct {
	uc usecase.UserUsecase
}

type updateProfileRequest struct {
	FullName        *string  `json:"full_name"`
	ExperienceLevel *string  `json:"experience_level"`
	PreferredRoles  []string `json:"preferred_roles"`
}

func NewUserHandler(uc usecase.UserUsecase) *UserHandler {
	return &UserHandler{uc: uc}
}

func (h *UserHandler) RegisterRoutes(r fiber.Router) {
	if r == nil {
		return
	}

	r.Get("/me", h.GetMe)
	r.Put("/me", h.UpdateMe)
}

func (h *UserHandler) GetMe(c fiber.Ctx) error {
	userID, ok := c.Locals(middleware.CtxUserIDKey).(uuid.UUID)
	if !ok {
		return middleware.NewAppError(fiber.StatusUnauthorized, "Unauthorized", nil, nil)
	}

	prof, err := h.uc.GetProfile(c.Context(), userID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return middleware.NewAppError(fiber.StatusNotFound, "User not found", nil, err)
		}
		return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, err)
	}

	res := dto.UserProfileResponse{
		ID:              prof.ID,
		Email:           prof.Email,
		FullName:        prof.FullName,
		ExperienceLevel: prof.ExperienceLevel,
		PreferredRoles:  prof.PreferredRoles,
		CreatedAt:       prof.CreatedAt,
	}
	return response.Success(c, fiber.StatusOK, response.MessageOK, res)
}

func (h *UserHandler) UpdateMe(c fiber.Ctx) error {
	userID, ok := c.Locals(middleware.CtxUserIDKey).(uuid.UUID)
	if !ok {
		return middleware.NewAppError(fiber.StatusUnauthorized, "Unauthorized", nil, nil)
	}

	var req updateProfileRequest
	if err := c.Bind().Body(&req); err != nil {
		return middleware.NewAppError(fiber.StatusBadRequest, "Invalid request payload", nil, err)
	}
	if req.FullName == nil && req.ExperienceLevel == nil && len(req.PreferredRoles) == 0 {
		return middleware.NewAppError(fiber.StatusBadRequest, "Invalid request payload", nil, nil)
	}

	prof, err := h.uc.UpdateProfile(c.Context(), userID, useruc.UpdateProfileInput{
		FullName:        req.FullName,
		ExperienceLevel: req.ExperienceLevel,
		PreferredRoles:  req.PreferredRoles,
	})
	if err != nil {
		if errors.Is(err, useruc.ErrInvalidInput) {
			return middleware.NewAppError(fiber.StatusBadRequest, "Invalid request payload", nil, err)
		}
		if errors.Is(err, user.ErrNotFound) {
			return middleware.NewAppError(fiber.StatusNotFound, "User not found", nil, err)
		}
		return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, err)
	}

	res := dto.UserProfileResponse{
		ID:              prof.ID,
		Email:           prof.Email,
		FullName:        prof.FullName,
		ExperienceLevel: prof.ExperienceLevel,
		PreferredRoles:  prof.PreferredRoles,
		CreatedAt:       prof.CreatedAt,
	}
	return response.Success(c, fiber.StatusOK, response.MessageOK, res)
}
