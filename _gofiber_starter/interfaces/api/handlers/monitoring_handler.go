package handlers

import (
	"github.com/gofiber/fiber/v2"

	natspkg "gofiber-template/infrastructure/nats"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/utils"
)

// MonitoringHandler handles JetStream monitoring endpoints
type MonitoringHandler struct {
	publisher *natspkg.Publisher
}

// NewMonitoringHandler creates a new MonitoringHandler
func NewMonitoringHandler(publisher *natspkg.Publisher) *MonitoringHandler {
	return &MonitoringHandler{
		publisher: publisher,
	}
}

// GetJetStreamStatus GET /api/v1/monitoring/jetstream
// ดึงสถานะของ JetStream stream และ consumer
func (h *MonitoringHandler) GetJetStreamStatus(c *fiber.Ctx) error {
	ctx := c.UserContext()

	if h.publisher == nil {
		logger.WarnContext(ctx, "NATS publisher not available")
		return utils.ErrorResponse(c, fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "NATS not available", nil)
	}

	status, err := h.publisher.GetJetStreamStatus(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get JetStream status", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, status)
}

// GetQueueStats GET /api/v1/monitoring/queue
// ดึงสถิติของ job queue
func (h *MonitoringHandler) GetQueueStats(c *fiber.Ctx) error {
	ctx := c.UserContext()

	if h.publisher == nil {
		logger.WarnContext(ctx, "NATS publisher not available")
		return utils.ErrorResponse(c, fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "NATS not available", nil)
	}

	stats, err := h.publisher.GetQueueStats(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get queue stats", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, stats)
}

// HealthCheck GET /api/v1/monitoring/health
// ตรวจสอบ health ของระบบ
func (h *MonitoringHandler) HealthCheck(c *fiber.Ctx) error {
	health := fiber.Map{
		"status": "ok",
		"nats":   h.publisher != nil,
	}

	return utils.SuccessResponse(c, health)
}
