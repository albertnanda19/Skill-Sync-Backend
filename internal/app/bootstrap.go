package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"skill-sync/internal/config"
	"skill-sync/internal/database/migration"
	"skill-sync/internal/delivery/http/middleware"
	"skill-sync/internal/delivery/http/routes"

	"github.com/gofiber/fiber/v3"
)

type App struct {
	Fiber *fiber.App
	C     *Container
}

func New(cfg config.Config) *App {
	f := fiber.New(fiber.Config{})
	_ = cfg

	registerGlobalMiddleware(f)
	registerRoutes(f)

	return &App{Fiber: f}
}

func Bootstrap(cfg config.Config) (*App, func() error, error) {
	c, err := NewContainer(cfg)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	r := migration.Runner{Dir: "migrations"}
	if err := r.Run(ctx, c.DB.SQLDB()); err != nil {
		_ = c.Close()
		return nil, nil, err
	}

	app := New(cfg)
	app.C = c
	return app, c.Close, nil
}

func registerGlobalMiddleware(app *fiber.App) {
	if app == nil {
		return
	}

	errMw := middleware.NewErrorMiddleware()
	app.Use(errMw.Middleware())
}

func registerRoutes(app *fiber.App) {
	if app == nil {
		return
	}

	routes.NewRegistry().Register(app)
}

func ListenAddr(port string) (string, error) {
	p := strings.TrimSpace(port)
	if p == "" {
		return "", fmt.Errorf("empty HTTP port")
	}
	if strings.HasPrefix(p, ":") {
		return p, nil
	}
	return ":" + p, nil
}
