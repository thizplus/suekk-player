package ports

import (
	"context"

	"seo-worker/domain/models"
)

// ArticlePublisherPort - Interface สำหรับส่ง Article ไปที่ api.subth.com
type ArticlePublisherPort interface {
	// PublishArticle ส่ง article ไปบันทึก
	// ใช้ ON CONFLICT (video_id) DO UPDATE สำหรับ idempotency
	PublishArticle(ctx context.Context, article *models.ArticleContent) error

	// UpdateArticleStatus อัพเดทสถานะ (draft/published)
	UpdateArticleStatus(ctx context.Context, videoID string, status string) error
}

// Article status constants
const (
	ArticleStatusDraft     = "draft"
	ArticleStatusPublished = "published"
	ArticleStatusFailed    = "failed"
)
