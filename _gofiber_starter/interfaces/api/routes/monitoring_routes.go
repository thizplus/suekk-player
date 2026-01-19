package routes

import (
	"github.com/gofiber/fiber/v2"

	"gofiber-template/interfaces/api/handlers"
)

// SetupMonitoringRoutes sets up the monitoring routes
// GET /api/v1/monitoring/jetstream - JetStream status
// GET /api/v1/monitoring/queue - Queue stats
// GET /api/v1/monitoring/health - Health check
func SetupMonitoringRoutes(app *fiber.App, h *handlers.Handlers) {
	monitoring := app.Group("/api/v1/monitoring")

	monitoring.Get("/jetstream", h.MonitoringHandler.GetJetStreamStatus)
	monitoring.Get("/queue", h.MonitoringHandler.GetQueueStats)
	monitoring.Get("/health", h.MonitoringHandler.HealthCheck)
}
