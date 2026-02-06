package postgres

import (
	"context"
	"database/sql"

	"skill-sync/internal/domain/user"
)

type UserRepository struct {
	db *PostgresDB

	stmtCreate     *sql.Stmt
	stmtGetByID    *sql.Stmt
	stmtGetByEmail *sql.Stmt
}

func NewUserRepository(db *PostgresDB) (*UserRepository, error) {
	r := &UserRepository{db: db}

	var err error
	r.stmtCreate, err = db.sqlDB().PrepareContext(
		context.Background(),
		`INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)`,
	)
	if err != nil {
		_ = r.Close()
		return nil, err
	}

	r.stmtGetByID, err = db.sqlDB().PrepareContext(
		context.Background(),
		`SELECT id, email, password_hash, created_at, updated_at FROM users WHERE id = $1`,
	)
	if err != nil {
		_ = r.Close()
		return nil, err
	}

	r.stmtGetByEmail, err = db.sqlDB().PrepareContext(
		context.Background(),
		`SELECT id, email, password_hash, created_at, updated_at FROM users WHERE email = $1`,
	)
	if err != nil {
		_ = r.Close()
		return nil, err
	}

	return r, nil
}

func (r *UserRepository) Close() error {
	var firstErr error
	closeStmt := func(s *sql.Stmt) {
		if s == nil {
			return
		}
		if err := s.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	closeStmt(r.stmtCreate)
	closeStmt(r.stmtGetByID)
	closeStmt(r.stmtGetByEmail)

	return firstErr
}

func (r *UserRepository) Create(ctx context.Context, u user.User) error {
	_, err := r.stmtCreate.ExecContext(ctx, u.ID, u.Email, u.PasswordHash)
	return err
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (user.User, error) {
	row := r.stmtGetByID.QueryRowContext(ctx, id)
	return scanUser(row)
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (user.User, error) {
	row := r.stmtGetByEmail.QueryRowContext(ctx, email)
	return scanUser(row)
}

type userRow interface {
	Scan(dest ...any) error
}

func scanUser(row userRow) (user.User, error) {
	var u user.User
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return user.User{}, user.ErrNotFound
		}
		return user.User{}, err
	}
	return u, nil
}
