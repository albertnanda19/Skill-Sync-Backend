package routes

import (
	"skill-sync/internal/delivery/http/handler"

	"github.com/gofiber/fiber/v3"
)

type Registry struct {
	health *handler.HealthHandler
}

func NewRegistry() *Registry {
	return &Registry{health: handler.NewHealthHandler()}
}

func (r *Registry) Register(app *fiber.App) {
	if app == nil {
		return
	}

	r.registerHealth(app)
	r.registerAPI(app)
}

func (r *Registry) registerHealth(app *fiber.App) {
	r.health.RegisterRoutes(app)
}

func (r *Registry) registerAPI(app *fiber.App) {
	api := app.Group("/api")
	RegisterV1(api.Group("/v1"))
}
