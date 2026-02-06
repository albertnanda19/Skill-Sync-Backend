package v1

import (
	"skill-sync/internal/delivery/http/handler"

	"github.com/gofiber/fiber/v3"
)

func RegisterJobs(r fiber.Router, jobRecommendationHandler *handler.JobRecommendationHandler) {
	if r == nil {
		return
	}
	if jobRecommendationHandler == nil {
		return
	}

	jobRecommendationHandler.RegisterRoutes(r)
}
