package routes

import (
	"github.com/gofiber/fiber/v2"
	"gofiber-template/interfaces/api/handlers"
	"gofiber-template/interfaces/api/middleware"
)

// SetupScheduledJobRoutes sets up routes for cron/scheduled jobs
// Note: ใช้ endpoint /jobs เหมือนเดิมเพื่อ backward compatibility
func SetupScheduledJobRoutes(api fiber.Router, h *handlers.Handlers) {
	jobs := api.Group("/jobs")
	jobs.Use(middleware.Protected())
	jobs.Use(middleware.AdminOnly()) // All job operations require admin access
	jobs.Post("/", h.ScheduledJobHandler.CreateJob)
	jobs.Get("/", h.ScheduledJobHandler.ListJobs)
	jobs.Get("/:id", h.ScheduledJobHandler.GetJob)
	jobs.Put("/:id", h.ScheduledJobHandler.UpdateJob)
	jobs.Delete("/:id", h.ScheduledJobHandler.DeleteJob)
	jobs.Post("/:id/start", h.ScheduledJobHandler.StartJob)
	jobs.Post("/:id/stop", h.ScheduledJobHandler.StopJob)
}
