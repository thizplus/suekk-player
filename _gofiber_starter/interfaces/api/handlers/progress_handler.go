package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"gofiber-template/pkg/progress"
	"gofiber-template/pkg/utils"
)

type ProgressHandler struct{}

func NewProgressHandler() *ProgressHandler {
	return &ProgressHandler{}
}

// GetProgress ดึง progress ของ video ที่กำลัง process
func (h *ProgressHandler) GetProgress(c *fiber.Ctx) error {
	idParam := c.Params("id")

	videoID, err := uuid.Parse(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	tracker := progress.GetTracker()
	data := tracker.GetProgress(videoID.String())

	if data == nil {
		return utils.SuccessResponse(c, fiber.Map{
			"videoId":  videoID.String(),
			"status":   "unknown",
			"progress": 0,
			"message":  "No active progress for this video",
		})
	}

	return utils.SuccessResponse(c, data)
}

// GetMyProgress ดึง progress ทั้งหมดของ user (ถ้า implement ในอนาคต)
func (h *ProgressHandler) GetMyProgress(c *fiber.Ctx) error {
	user, err := utils.GetUserFromContext(c)
	if err != nil {
		return utils.UnauthorizedResponse(c, "")
	}

	// สำหรับตอนนี้ return empty array
	// ในอนาคตอาจเก็บ progress per user
	return utils.SuccessResponse(c, fiber.Map{
		"userId":   user.ID.String(),
		"progress": []interface{}{},
	})
}
