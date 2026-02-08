package routes

import (
	"context"
	"log"
	"skill-sync/internal/config"
	"skill-sync/internal/database"
	"skill-sync/internal/delivery/http/handler"
	"skill-sync/internal/infrastructure/cache"
	searchctl "skill-sync/internal/search/controller"
	"skill-sync/internal/ws"
	"strings"
	"time"

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

	logger := log.Default()
	wsHub := ws.NewHub(logger)
	ws.SetDefaultHub(wsHub)
	redisCache := cache.NewRedis(logger)
	canceler := searchctl.NewTaskCanceler(r.cfg.ScraperBaseURL, r.cfg.InternalToken, logger)
	ws.SetOnZeroClients(func(keyword string) {
		if canceler == nil || redisCache == nil {
			return
		}
		kw := strings.ToLower(strings.Join(strings.Fields(keyword), " "))
		if kw == "" {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		lockKey := "scrape:cancel:lock:" + kw
		ok, _ := redisCache.SetIfNotExists(ctx, lockKey, "1", 10*time.Second)
		if !ok {
			return
		}
		if wsHub.ClientCount(kw) > 0 {
			_ = redisCache.Delete(context.Background(), lockKey)
			return
		}

		taskID, exists, _ := redisCache.GetString(ctx, "scrape:task:"+kw)
		if !exists || strings.TrimSpace(taskID) == "" {
			_ = redisCache.Delete(context.Background(), lockKey)
			return
		}
		if wsHub.ClientCount(kw) > 0 {
			_ = redisCache.Delete(context.Background(), lockKey)
			return
		}

		ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
		_, _ = canceler.DeleteScrapeTask(ctx2, taskID, true)
		cancel2()
		_ = redisCache.Delete(context.Background(), "scrape:lock:"+kw)
		_ = redisCache.Delete(context.Background(), "scrape:task:"+kw)
		_ = redisCache.Delete(context.Background(), lockKey)
	})
	go wsHub.Run()
	wsHandler := ws.NewHandler(wsHub, logger)
	app.Get("/ws/jobs", wsHandler.HandleJobsWS)

	r.registerHealth(app)
	r.registerInternal(app)
	r.registerAPI(app)
}

func (r *Registry) registerHealth(app *fiber.App) {
	r.health.RegisterRoutes(app)
}

func (r *Registry) registerAPI(app *fiber.App) {
	api := app.Group("/api")
	RegisterV1(api.Group("/v1"), r.cfg, r.db)
}

func (r *Registry) registerInternal(app *fiber.App) {
	logger := log.Default()
	redisCache := cache.NewRedis(logger)
	internalHandler := handler.NewScrapeCompletedHandler(r.cfg, redisCache, logger)

	internal := app.Group("/internal")
	internal.Post("/scrape-completed", internalHandler.HandleScrapeCompleted)
}
