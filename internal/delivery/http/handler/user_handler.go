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
	uc     usecase.UserUsecase
	skills usecase.UserSkillUsecase
}

func (h *UserHandler) Onboarding(c fiber.Ctx) error {
	userID, ok := c.Locals(middleware.CtxUserIDKey).(uuid.UUID)
	if !ok {
		return middleware.NewAppError(fiber.StatusUnauthorized, "Unauthorized", nil, nil)
	}

	var req onboardingRequest
	if err := c.Bind().Body(&req); err != nil {
		return middleware.NewAppError(fiber.StatusBadRequest, "Invalid request payload", nil, err)
	}
	if req.ExperienceLevel == nil && len(req.PreferredRoles) == 0 && len(req.Skills) == 0 {
		return middleware.NewAppError(fiber.StatusBadRequest, "Invalid request payload", nil, nil)
	}

	if req.ExperienceLevel != nil || len(req.PreferredRoles) > 0 {
		_, err := h.uc.UpdateProfile(c.Context(), userID, useruc.UpdateProfileInput{
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
	}

	if len(req.Skills) > 0 {
		if h.skills == nil {
			return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, nil)
		}
		for _, s := range req.Skills {
			created, err := h.skills.AddUserSkill(c.Context(), userID, usecase.AddUserSkillInput{
				SkillID:          s.SkillID,
				ProficiencyLevel: s.ProficiencyLevel,
				YearsExperience:  s.YearsExperience,
			})
			_ = created
			if err != nil {
				if errors.Is(err, usecase.ErrSkillAlreadyExists) {
					if rerr := h.skills.RemoveUserSkill(c.Context(), userID, s.SkillID); rerr != nil {
						return mapUserSkillUsecaseError(rerr)
					}
					_, aerr := h.skills.AddUserSkill(c.Context(), userID, usecase.AddUserSkillInput{
						SkillID:          s.SkillID,
						ProficiencyLevel: s.ProficiencyLevel,
						YearsExperience:  s.YearsExperience,
					})
					if aerr != nil {
						return mapUserSkillUsecaseError(aerr)
					}
					continue
				}
				return mapUserSkillUsecaseError(err)
			}
		}
	}

	prof, err := h.uc.GetProfile(c.Context(), userID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return middleware.NewAppError(fiber.StatusNotFound, "User not found", nil, err)
		}
		return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, err)
	}

	items := make([]dto.UserMeSkillItem, 0)
	if h.skills != nil {
		skillItems, serr := h.skills.ListUserSkills(c.Context(), userID)
		if serr != nil {
			return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, serr)
		}
		items = make([]dto.UserMeSkillItem, 0, len(skillItems))
		for _, it := range skillItems {
			items = append(items, dto.UserMeSkillItem{
				SkillName:        it.SkillName,
				ProficiencyLevel: it.ProficiencyLevel,
				YearsExperience:  it.YearsExperience,
			})
		}
	}

	core := make([]dto.UserMeSkillItem, 0)
	developing := make([]dto.UserMeSkillItem, 0)
	for _, it := range items {
		if it.ProficiencyLevel >= 4 || it.YearsExperience >= 3 {
			core = append(core, it)
			continue
		}
		developing = append(developing, it)
	}

	res := dto.UserMeResponse{
		ID:        prof.ID,
		Email:     prof.Email,
		FullName:  prof.FullName,
		CreatedAt: prof.CreatedAt,
		Skills: dto.UserMeSkillsSection{
			Core:       core,
			Developing: developing,
		},
		Preferences: dto.UserMePreferencesSection{
			PreferredRoles:     prof.PreferredRoles,
			PreferenceLocation: prof.PreferenceLocation,
			ExperienceLevel:    prof.ExperienceLevel,
		},
	}
	return response.Success(c, fiber.StatusOK, response.MessageOK, res)
}

type updateProfileRequest struct {
	FullName           *string  `json:"full_name"`
	ExperienceLevel    *string  `json:"experience_level"`
	PreferenceLocation *string  `json:"preference_location"`
	PreferredRoles     []string `json:"preferred_roles"`
}

type onboardingSkillRequest struct {
	SkillID          uuid.UUID `json:"skill_id"`
	ProficiencyLevel int       `json:"proficiency_level"`
	YearsExperience  int       `json:"years_experience"`
}

type onboardingRequest struct {
	ExperienceLevel *string                  `json:"experience_level"`
	PreferredRoles  []string                 `json:"preferred_roles"`
	Skills          []onboardingSkillRequest `json:"skills"`
}

func NewUserHandler(uc usecase.UserUsecase, skills usecase.UserSkillUsecase) *UserHandler {
	return &UserHandler{uc: uc, skills: skills}
}

func (h *UserHandler) RegisterRoutes(r fiber.Router) {
	if r == nil {
		return
	}

	r.Get("/me", h.GetMe)
	r.Post("/me/onboarding", h.Onboarding)
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

	items := make([]dto.UserMeSkillItem, 0)
	if h.skills != nil {
		skillItems, serr := h.skills.ListUserSkills(c.Context(), userID)
		if serr != nil {
			return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, serr)
		}
		items = make([]dto.UserMeSkillItem, 0, len(skillItems))
		for _, it := range skillItems {
			items = append(items, dto.UserMeSkillItem{
				SkillName:        it.SkillName,
				ProficiencyLevel: it.ProficiencyLevel,
				YearsExperience:  it.YearsExperience,
			})
		}
	}

	core := make([]dto.UserMeSkillItem, 0)
	developing := make([]dto.UserMeSkillItem, 0)
	for _, it := range items {
		if it.ProficiencyLevel >= 4 || it.YearsExperience >= 3 {
			core = append(core, it)
			continue
		}
		developing = append(developing, it)
	}

	res := dto.UserMeResponse{
		ID:        prof.ID,
		Email:     prof.Email,
		FullName:  prof.FullName,
		CreatedAt: prof.CreatedAt,
		Skills: dto.UserMeSkillsSection{
			Core:       core,
			Developing: developing,
		},
		Preferences: dto.UserMePreferencesSection{
			PreferredRoles:     prof.PreferredRoles,
			PreferenceLocation: prof.PreferenceLocation,
			ExperienceLevel:    prof.ExperienceLevel,
		},
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
	if req.FullName == nil && req.ExperienceLevel == nil && req.PreferenceLocation == nil && len(req.PreferredRoles) == 0 {
		return middleware.NewAppError(fiber.StatusBadRequest, "Invalid request payload", nil, nil)
	}

	prof, err := h.uc.UpdateProfile(c.Context(), userID, useruc.UpdateProfileInput{
		FullName:           req.FullName,
		ExperienceLevel:    req.ExperienceLevel,
		PreferenceLocation: req.PreferenceLocation,
		PreferredRoles:     req.PreferredRoles,
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
		ID:                 prof.ID,
		Email:              prof.Email,
		FullName:           prof.FullName,
		ExperienceLevel:    prof.ExperienceLevel,
		PreferenceLocation: prof.PreferenceLocation,
		PreferredRoles:     prof.PreferredRoles,
		CreatedAt:          prof.CreatedAt,
	}
	return response.Success(c, fiber.StatusOK, response.MessageOK, res)
}
