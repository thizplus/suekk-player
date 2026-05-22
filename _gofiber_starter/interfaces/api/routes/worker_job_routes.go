package routes

import (
	"github.com/gofiber/fiber/v2"
	"gofiber-template/interfaces/api/handlers"
	"gofiber-template/interfaces/api/middleware"
)

// SetupWorkerJobRoutes sets up routes for worker queue jobs (transcode, gallery, subtitle, reel)
func SetupWorkerJobRoutes(api fiber.Router, h *handlers.Handlers) {
	workerJobs := api.Group("/worker-jobs")
	workerJobs.Use(middleware.Protected())

	// Stats endpoints (must be before /:id to avoid route conflict)
	workerJobs.Get("/stats", h.WorkerJobHandler.GetStats)
	workerJobs.Get("/stats/all", h.WorkerJobHandler.GetAllStats)

	// List and detail
	workerJobs.Get("/", h.WorkerJobHandler.ListJobs)
	workerJobs.Get("/:id", h.WorkerJobHandler.GetJob)

	// Entity-based query
	workerJobs.Get("/entity/:type/:id", h.WorkerJobHandler.GetJobsByEntity)

	// Actions
	workerJobs.Post("/:id/cancel", h.WorkerJobHandler.CancelJob)
	workerJobs.Post("/:id/retry", h.WorkerJobHandler.RetryJob)

	// Cleanup (must be before /:id to avoid route conflict)
	workerJobs.Delete("/orphaned", h.WorkerJobHandler.DeleteOrphanedJobs)
	workerJobs.Delete("/completed", h.WorkerJobHandler.DeleteCompletedJobs)
	workerJobs.Delete("/failed", h.WorkerJobHandler.DeleteFailedJobs)
	workerJobs.Delete("/:id", h.WorkerJobHandler.DeleteJob)
}
