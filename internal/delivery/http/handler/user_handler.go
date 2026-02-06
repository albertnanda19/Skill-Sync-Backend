package handler

import (
	"errors"

	"skill-sync/internal/delivery/http/middleware"
	"skill-sync/internal/pkg/response"
	useruc "skill-sync/internal/usecase/user"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type UserHandler struct {
	uc *useruc.Service
}

type updateMeRequest struct {
	Email    *string `json:"email"`
	Password *string `json:"password"`
}

func NewUserHandler(uc *useruc.Service) *UserHandler {
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

	usr, err := h.uc.GetMe(c.Context(), userID)
	if err != nil {
		return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, err)
	}
	return response.Success(c, fiber.StatusOK, response.MessageOK, usr)
}

func (h *UserHandler) UpdateMe(c fiber.Ctx) error {
	userID, ok := c.Locals(middleware.CtxUserIDKey).(uuid.UUID)
	if !ok {
		return middleware.NewAppError(fiber.StatusUnauthorized, "Unauthorized", nil, nil)
	}

	var req updateMeRequest
	if err := c.Bind().Body(&req); err != nil {
		return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, err)
	}

	usr, err := h.uc.UpdateMe(c.Context(), userID, useruc.UpdateMeInput{Email: req.Email, Password: req.Password})
	if err != nil {
		if errors.Is(err, useruc.ErrInvalidInput) {
			return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, err)
		}
		return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, err)
	}

	return response.Success(c, fiber.StatusOK, response.MessageOK, usr)
}
