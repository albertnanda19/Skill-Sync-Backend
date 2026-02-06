package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"skill-sync/internal/config"
)

type PostgresDB struct {
	db *sql.DB
}

func Connect(cfg config.DatabaseConfig) (*PostgresDB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBName,
		cfg.DBSSLMode,
	)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &PostgresDB{db: db}, nil
}

func (p *PostgresDB) Close() error {
	if p == nil || p.db == nil {
		return nil
	}
	return p.db.Close()
}

func (p *PostgresDB) sqlDB() *sql.DB {
	if p == nil {
		return nil
	}
	return p.db
}

func (p *PostgresDB) SQLDB() *sql.DB {
	return p.sqlDB()
}
