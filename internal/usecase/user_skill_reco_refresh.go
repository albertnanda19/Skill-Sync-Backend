package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type UserSkillWithRecommendationRefresh struct {
	base  UserSkillUsecase
	reco  AIRecommendationUsecase
	cache *AIRecommendationCache
}

func NewUserSkillWithRecommendationRefresh(base UserSkillUsecase, reco AIRecommendationUsecase, cache *AIRecommendationCache) *UserSkillWithRecommendationRefresh {
	return &UserSkillWithRecommendationRefresh{base: base, reco: reco, cache: cache}
}

func (u *UserSkillWithRecommendationRefresh) ListUserSkills(ctx context.Context, userID uuid.UUID) ([]UserSkillItem, error) {
	return u.base.ListUserSkills(ctx, userID)
}

func (u *UserSkillWithRecommendationRefresh) AddUserSkill(ctx context.Context, userID uuid.UUID, in AddUserSkillInput) (UserSkillItem, error) {
	out, err := u.base.AddUserSkill(ctx, userID, in)
	if err == nil {
		u.refreshAsync(userID)
	}
	return out, err
}

func (u *UserSkillWithRecommendationRefresh) UpdateUserSkill(ctx context.Context, userID uuid.UUID, skillUserID uuid.UUID, in UpdateUserSkillInput) (UserSkillItem, error) {
	out, err := u.base.UpdateUserSkill(ctx, userID, skillUserID, in)
	if err == nil {
		u.refreshAsync(userID)
	}
	return out, err
}

func (u *UserSkillWithRecommendationRefresh) DeleteUserSkill(ctx context.Context, userID uuid.UUID, skillUserID uuid.UUID) error {
	err := u.base.DeleteUserSkill(ctx, userID, skillUserID)
	if err == nil {
		u.refreshAsync(userID)
	}
	return err
}

func (u *UserSkillWithRecommendationRefresh) RemoveUserSkill(ctx context.Context, userID uuid.UUID, skillID uuid.UUID) error {
	err := u.base.RemoveUserSkill(ctx, userID, skillID)
	if err == nil {
		u.refreshAsync(userID)
	}
	return err
}

func (u *UserSkillWithRecommendationRefresh) refreshAsync(userID uuid.UUID) {
	if userID == uuid.Nil {
		return
	}
	if u.cache == nil && u.reco == nil {
		return
	}

	go func() {
		bg := context.Background()
		if u.cache != nil {
			u.cache.InvalidateUser(bg, userID)
		}
		if u.reco == nil {
			return
		}
		ctx, cancel := context.WithTimeout(bg, 20*time.Second)
		defer cancel()
		_, _ = u.reco.GetAIRecommendations(ctx, userID)
	}()
}
