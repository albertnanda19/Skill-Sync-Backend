package app

import (
	"net/http"

	"skill-sync/internal/config"
)

type App struct {
	Router http.Handler
}

func New(c *Container) *App {
	return &App{}
}

func Bootstrap(cfg config.Config) (*App, func() error, error) {
	c, err := NewContainer(cfg)
	if err != nil {
		return nil, nil, err
	}

	app := New(c)
	return app, c.Close, nil
}
