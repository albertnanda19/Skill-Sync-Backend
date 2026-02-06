package database

import (
	"context"
	"database/sql"
)

type DB interface {
	Ping(ctx context.Context) error
	Close() error

	Exec(ctx context.Context, query string, args ...any) (int64, error)
	Query(ctx context.Context, query string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) Row

	Begin(ctx context.Context) (Tx, error)

	SQLDB() *sql.DB
}

type Tx interface {
	Exec(ctx context.Context, query string, args ...any) (int64, error)
	Query(ctx context.Context, query string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) Row

	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type Rows interface {
	Close()
	Next() bool
	Scan(dest ...any) error
	Err() error
}

type Row interface {
	Scan(dest ...any) error
}
