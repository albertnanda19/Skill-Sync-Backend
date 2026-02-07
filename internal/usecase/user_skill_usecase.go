package usecase

import (
	"context"
	"errors"

	"skill-sync/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrSkillAlreadyExists      = errors.New("skill already exists")
	ErrSkillNotFound           = errors.New("skill not found")
	ErrForbidden               = errors.New("forbidden")
	ErrInvalidProficiencyLevel = errors.New("invalid proficiency level")
	ErrInvalidInput            = errors.New("invalid input")
)

type AddUserSkillInput struct {
	SkillID          uuid.UUID
	ProficiencyLevel int
	YearsExperience  int
}

type UpdateUserSkillInput struct {
	ProficiencyLevel int
	YearsExperience  int
}

type UserSkillItem struct {
	ID               uuid.UUID
	SkillID          uuid.UUID
	SkillName        string
	ProficiencyLevel int
	YearsExperience  int
}

type UserSkillUsecase interface {
	ListUserSkills(ctx context.Context, userID uuid.UUID) ([]UserSkillItem, error)
	AddUserSkill(ctx context.Context, userID uuid.UUID, in AddUserSkillInput) (UserSkillItem, error)
	UpdateUserSkill(ctx context.Context, userID uuid.UUID, skillUserID uuid.UUID, in UpdateUserSkillInput) (UserSkillItem, error)
	DeleteUserSkill(ctx context.Context, userID uuid.UUID, skillUserID uuid.UUID) error
	RemoveUserSkill(ctx context.Context, userID uuid.UUID, skillID uuid.UUID) error
}

type UserSkill struct {
	repo repository.UserSkillRepository
}

func NewUserSkillUsecase(repo repository.UserSkillRepository) *UserSkill {
	return &UserSkill{repo: repo}
}

func (u *UserSkill) ListUserSkills(ctx context.Context, userID uuid.UUID) ([]UserSkillItem, error) {
	items, err := u.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, ErrInternal
	}
	out := make([]UserSkillItem, 0, len(items))
	for _, it := range items {
		out = append(out, UserSkillItem{
			ID:               it.ID,
			SkillID:          it.SkillID,
			SkillName:        it.SkillName,
			ProficiencyLevel: it.ProficiencyLevel,
			YearsExperience:  it.YearsExperience,
		})
	}
	return out, nil
}

func (u *UserSkill) AddUserSkill(ctx context.Context, userID uuid.UUID, in AddUserSkillInput) (UserSkillItem, error) {
	if in.SkillID == uuid.Nil {
		return UserSkillItem{}, ErrInvalidInput
	}
	if !isValidProficiency(in.ProficiencyLevel) {
		return UserSkillItem{}, ErrInvalidProficiencyLevel
	}
	if in.YearsExperience < 0 {
		return UserSkillItem{}, ErrInvalidInput
	}

	exists, err := u.repo.SkillExistsByID(ctx, in.SkillID)
	if err != nil {
		return UserSkillItem{}, ErrInternal
	}
	if !exists {
		return UserSkillItem{}, ErrSkillNotFound
	}

	_, err = u.repo.FindByUserAndSkill(ctx, userID, in.SkillID)
	if err == nil {
		return UserSkillItem{}, ErrSkillAlreadyExists
	}
	if !errors.Is(err, repository.ErrUserSkillNotFound) {
		return UserSkillItem{}, ErrInternal
	}

	created, err := u.repo.Create(ctx, repository.UserSkill{
		ID:               uuid.New(),
		UserID:           userID,
		SkillID:          in.SkillID,
		ProficiencyLevel: in.ProficiencyLevel,
		YearsExperience:  in.YearsExperience,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return UserSkillItem{}, ErrSkillAlreadyExists
		}
		if isForeignKeyViolation(err) {
			return UserSkillItem{}, ErrSkillNotFound
		}
		return UserSkillItem{}, ErrInternal
	}

	return UserSkillItem{
		ID:               created.ID,
		SkillID:          created.SkillID,
		SkillName:        created.SkillName,
		ProficiencyLevel: created.ProficiencyLevel,
		YearsExperience:  created.YearsExperience,
	}, nil
}

func (u *UserSkill) UpdateUserSkill(ctx context.Context, userID uuid.UUID, skillUserID uuid.UUID, in UpdateUserSkillInput) (UserSkillItem, error) {
	if skillUserID == uuid.Nil {
		return UserSkillItem{}, ErrInvalidInput
	}
	if !isValidProficiency(in.ProficiencyLevel) {
		return UserSkillItem{}, ErrInvalidProficiencyLevel
	}
	if in.YearsExperience < 0 {
		return UserSkillItem{}, ErrInvalidInput
	}

	updated, err := u.repo.Update(ctx, repository.UserSkill{
		ID:               skillUserID,
		UserID:           userID,
		ProficiencyLevel: in.ProficiencyLevel,
		YearsExperience:  in.YearsExperience,
	})
	if err != nil {
		if errors.Is(err, repository.ErrUserSkillNotFound) {
			return UserSkillItem{}, ErrSkillNotFound
		}
		return UserSkillItem{}, ErrInternal
	}
	return UserSkillItem{
		ID:               updated.ID,
		SkillID:          updated.SkillID,
		SkillName:        updated.SkillName,
		ProficiencyLevel: updated.ProficiencyLevel,
		YearsExperience:  updated.YearsExperience,
	}, nil
}

func (u *UserSkill) DeleteUserSkill(ctx context.Context, userID uuid.UUID, skillUserID uuid.UUID) error {
	if skillUserID == uuid.Nil {
		return ErrInvalidInput
	}
	err := u.repo.Delete(ctx, skillUserID, userID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrUserSkillNotFound):
			return ErrSkillNotFound
		case errors.Is(err, repository.ErrUserSkillForbidden):
			return ErrForbidden
		default:
			return ErrInternal
		}
	}
	return nil
}

func (u *UserSkill) RemoveUserSkill(ctx context.Context, userID uuid.UUID, skillID uuid.UUID) error {
	if skillID == uuid.Nil {
		return ErrInvalidInput
	}
	if err := u.repo.DeleteUserSkill(ctx, userID, skillID); err != nil {
		switch {
		case errors.Is(err, repository.ErrUserSkillNotFound):
			return ErrSkillNotFound
		case errors.Is(err, repository.ErrUserSkillForbidden):
			return ErrForbidden
		default:
			return ErrInternal
		}
	}
	return nil
}

func isValidProficiency(v int) bool {
	return v >= 1 && v <= 5
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23503"
	}
	return false
}
