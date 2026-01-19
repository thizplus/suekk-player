package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gofiber-template/pkg/logger"
)

const RequestIDHeader = "X-Request-ID"

// RequestIDMiddleware สร้าง request ID สำหรับทุก request
func RequestIDMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// ตรวจสอบว่ามี request ID จาก client ไหม
		requestID := c.Get(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// ใส่ request ID ใน response header
		c.Set(RequestIDHeader, requestID)

		// ใส่ request ID ใน context สำหรับ logging
		ctx := logger.ContextWithRequestID(c.Context(), requestID)
		c.SetUserContext(ctx)

		// ใส่ใน locals สำหรับ access ง่ายใน handlers
		c.Locals("request_id", requestID)

		return c.Next()
	}
}

// GetRequestIDFromContext ดึง request ID จาก fiber context
func GetRequestIDFromContext(c *fiber.Ctx) string {
	if requestID, ok := c.Locals("request_id").(string); ok {
		return requestID
	}
	return ""
}
