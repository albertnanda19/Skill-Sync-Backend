package app

import (
	"skill-sync/internal/config"
	"skill-sync/internal/domain/user"
	"skill-sync/internal/infrastructure/persistence/postgres"
)

type Container struct {
	Config config.Config
	DB     *postgres.PostgresDB
	Users  user.Repository
}

func NewContainer(cfg config.Config) (*Container, error) {
	db, err := postgres.Connect(cfg.Database)
	if err != nil {
		return nil, err
	}

	usersRepo, err := postgres.NewUserRepository(db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Container{Config: cfg, DB: db, Users: usersRepo}, nil
}

func (c *Container) Close() error {
	if c == nil {
		return nil
	}
	if repo, ok := c.Users.(interface{ Close() error }); ok {
		_ = repo.Close()
	}
	if c.DB == nil {
		return nil
	}
	return c.DB.Close()
}
