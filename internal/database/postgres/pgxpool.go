package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"skill-sync/internal/config"
	"skill-sync/internal/database"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

type Pool struct {
	pool  *pgxpool.Pool
	sqlDB *sql.DB
}

func Connect(ctx context.Context, cfg config.DatabaseConfig) (database.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		strings.TrimSpace(cfg.DBHost),
		strings.TrimSpace(cfg.DBPort),
		strings.TrimSpace(cfg.DBUser),
		cfg.DBPassword,
		strings.TrimSpace(cfg.DBName),
		strings.TrimSpace(cfg.DBSSLMode),
	)

	pcfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}

	if cfg.ConnectTimeout > 0 {
		pcfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	}
	if cfg.PoolMaxConns > 0 {
		pcfg.MaxConns = cfg.PoolMaxConns
	}
	if cfg.PoolMinConns > 0 {
		pcfg.MinConns = cfg.PoolMinConns
	}
	if cfg.PoolMaxConnLifetime > 0 {
		pcfg.MaxConnLifetime = cfg.PoolMaxConnLifetime
	}
	if cfg.PoolMaxConnIdleTime > 0 {
		pcfg.MaxConnIdleTime = cfg.PoolMaxConnIdleTime
	}
	if cfg.PoolHealthCheckPeriod > 0 {
		pcfg.HealthCheckPeriod = cfg.PoolHealthCheckPeriod
	}

	p, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, err
	}

	pingCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		pingCtx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	if err := p.Ping(pingCtx); err != nil {
		p.Close()
		return nil, err
	}

	sqldb := stdlib.OpenDBFromPool(p)
	return &Pool{pool: p, sqlDB: sqldb}, nil
}

func (p *Pool) Ping(ctx context.Context) error {
	if p == nil || p.pool == nil {
		return fmt.Errorf("nil db")
	}
	return p.pool.Ping(ctx)
}

func (p *Pool) Close() error {
	if p == nil {
		return nil
	}
	if p.sqlDB != nil {
		_ = p.sqlDB.Close()
	}
	if p.pool != nil {
		p.pool.Close()
	}
	return nil
}

func (p *Pool) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	if p == nil || p.pool == nil {
		return 0, fmt.Errorf("nil db")
	}
	tag, err := p.pool.Exec(ctx, query, args...)
	return tag.RowsAffected(), err
}

func (p *Pool) Query(ctx context.Context, query string, args ...any) (database.Rows, error) {
	if p == nil || p.pool == nil {
		return nil, fmt.Errorf("nil db")
	}
	r, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return pgxRows{rows: r}, nil
}

func (p *Pool) QueryRow(ctx context.Context, query string, args ...any) database.Row {
	if p == nil || p.pool == nil {
		return pgxRow{row: nilRow{}}
	}
	return pgxRow{row: p.pool.QueryRow(ctx, query, args...)}
}

func (p *Pool) Begin(ctx context.Context) (database.Tx, error) {
	if p == nil || p.pool == nil {
		return nil, fmt.Errorf("nil db")
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return pgxTx{tx: tx}, nil
}

func (p *Pool) SQLDB() *sql.DB {
	if p == nil {
		return nil
	}
	return p.sqlDB
}

type pgxTx struct {
	tx pgx.Tx
}

func (t pgxTx) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	tag, err := t.tx.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return commandTag(tag).RowsAffected(), nil
}

func (t pgxTx) Query(ctx context.Context, query string, args ...any) (database.Rows, error) {
	r, err := t.tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return pgxRows{rows: r}, nil
}

func (t pgxTx) QueryRow(ctx context.Context, query string, args ...any) database.Row {
	return pgxRow{row: t.tx.QueryRow(ctx, query, args...)}
}

func (t pgxTx) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t pgxTx) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}

type pgxRows struct {
	rows pgx.Rows
}

func (r pgxRows) Close() {
	r.rows.Close()
}

func (r pgxRows) Next() bool {
	return r.rows.Next()
}

func (r pgxRows) Scan(dest ...any) error {
	return r.rows.Scan(dest...)
}

func (r pgxRows) Err() error {
	return r.rows.Err()
}

type pgxRow struct {
	row pgx.Row
}

func (r pgxRow) Scan(dest ...any) error {
	return r.row.Scan(dest...)
}

type nilRow struct{}

func (nilRow) Scan(_ ...any) error {
	return fmt.Errorf("nil db")
}

type commandTag pgconn.CommandTag

func (t commandTag) RowsAffected() int64 {
	return pgconn.CommandTag(t).RowsAffected()
}
