package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func CorsMiddleware() fiber.Handler {
	return cors.New(cors.Config{
		// Allow origins for development and production
		AllowOrigins:     "http://localhost:5173,http://localhost:5174,http://localhost:3000,https://cdn.suekk.com,https://suekk.com,https://*.suekk.com",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS,HEAD",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,Range,Cache-Control,X-Requested-With,X-Stream-Token",
		ExposeHeaders:    "Content-Length,Content-Range,Accept-Ranges,Content-Type",
		AllowCredentials: true, // เปิด credentials สำหรับ cookies/auth
	})
}