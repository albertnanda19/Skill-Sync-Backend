package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"skill-sync/internal/database"
	"skill-sync/internal/domain/user"
)

type UserRepository struct {
	db database.DB
}

func NewUserRepository(db database.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) CreateUser(ctx context.Context, u user.User) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)`,
		u.ID, u.Email, u.PasswordHash,
	)
	return err
}

func (r *UserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (user.User, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, email, password_hash, created_at, updated_at FROM users WHERE id = $1`,
		id,
	)
	return scanUserRow(row)
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (user.User, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, email, password_hash, created_at, updated_at FROM users WHERE email = $1`,
		email,
	)
	return scanUserRow(row)
}

func (r *UserRepository) UpdateUser(ctx context.Context, u user.User) error {
	rowsAffected, err := r.db.Exec(ctx,
		`UPDATE users SET email = $1, password_hash = $2, updated_at = now() WHERE id = $3`,
		u.Email, u.PasswordHash, u.ID,
	)
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return user.ErrNotFound
	}
	return nil
}

func (r *UserRepository) DeleteUser(ctx context.Context, id uuid.UUID) error {
	rowsAffected, err := r.db.Exec(ctx,
		`DELETE FROM users WHERE id = $1`,
		id,
	)
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return user.ErrNotFound
	}
	return nil
}

func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var exists bool
	row := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`,
		email,
	)
	if err := row.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (r *UserRepository) GetProfileByUserID(ctx context.Context, userID uuid.UUID) (user.Profile, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, user_id, full_name, experience_level, preferred_roles, created_at, updated_at FROM user_profiles WHERE user_id = $1`,
		userID,
	)

	var p user.Profile
	var roles []string
	if err := row.Scan(&p.ID, &p.UserID, &p.FullName, &p.ExperienceLevel, &roles, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows || errors.Is(err, pgx.ErrNoRows) {
			return user.Profile{}, user.ErrNotFound
		}
		return user.Profile{}, err
	}
	p.PreferredRoles = roles
	return p, nil
}

func (r *UserRepository) UpdateProfile(ctx context.Context, p user.Profile) error {
	if p.UserID == nil {
		return user.ErrNotFound
	}

	_, err := r.db.Exec(ctx,
		`INSERT INTO user_profiles (id, user_id, full_name, experience_level, preferred_roles)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (user_id) DO UPDATE SET
		  full_name = EXCLUDED.full_name,
		  experience_level = EXCLUDED.experience_level,
		  preferred_roles = EXCLUDED.preferred_roles,
		  updated_at = now()`,
		p.ID, *p.UserID, p.FullName, p.ExperienceLevel, p.PreferredRoles,
	)
	if err != nil {
		return err
	}
	return nil
}

func scanUserRow(row database.Row) (user.User, error) {
	var u user.User
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if err == sql.ErrNoRows || errors.Is(err, pgx.ErrNoRows) {
			return user.User{}, user.ErrNotFound
		}
		return user.User{}, err
	}
	return u, nil
}
