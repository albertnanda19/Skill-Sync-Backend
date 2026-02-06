package app

import (
	"context"
	"time"

	"skill-sync/internal/config"
	"skill-sync/internal/database"
	dbpostgres "skill-sync/internal/database/postgres"
)

type Container struct {
	Config config.Config
	DB     database.DB
}

func NewContainer(cfg config.Config) (*Container, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := dbpostgres.Connect(ctx, cfg.Database)
	if err != nil {
		return nil, err
	}

	return &Container{Config: cfg, DB: db}, nil
}

func (c *Container) Close() error {
	if c == nil {
		return nil
	}
	if c.DB == nil {
		return nil
	}
	return c.DB.Close()
}
