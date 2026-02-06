package app

import (
	"fmt"
	"strings"

	"skill-sync/internal/config"
	"skill-sync/internal/delivery/http/middleware"
	"skill-sync/internal/pkg/response"

	"github.com/gofiber/fiber/v3"
)

type App struct {
	Fiber *fiber.App
}

func New(cfg config.Config) *App {
	f := fiber.New(fiber.Config{})
	_ = cfg

	registerGlobalMiddleware(f)
	registerRoutes(f)

	return &App{Fiber: f}
}

func Bootstrap(cfg config.Config) (*App, func() error, error) {
	app := New(cfg)
	return app, func() error { return nil }, nil
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

	app.Get("/health", func(c fiber.Ctx) error {
		return response.Success(c, fiber.StatusOK, response.MessageOK, nil)
	})
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
