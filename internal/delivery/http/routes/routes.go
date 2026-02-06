package routes

import (
	"skill-sync/internal/config"
	"skill-sync/internal/database"
	"skill-sync/internal/delivery/http/handler"

	"github.com/gofiber/fiber/v3"
)

type Registry struct {
	cfg    config.Config
	db     database.DB
	health *handler.HealthHandler
}

func NewRegistry(cfg config.Config, db database.DB) *Registry {
	return &Registry{cfg: cfg, db: db, health: handler.NewHealthHandler()}
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
	RegisterV1(api.Group("/v1"), r.cfg, r.db)
}
