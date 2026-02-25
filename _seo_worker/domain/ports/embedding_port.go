package ports

import (
	"context"

	"seo-worker/domain/models"
)

// EmbeddingPort - Interface สำหรับ Vector Embedding (pgvector)
type EmbeddingPort interface {
	// GenerateEmbedding สร้าง vector จาก text
	// ใช้ Gemini text-embedding-004 หรือ OpenAI ada-002
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)

	// StoreEmbedding บันทึกลง pgvector พร้อม metadata
	// ใช้ ON CONFLICT สำหรับ idempotency
	StoreEmbedding(ctx context.Context, data *models.EmbeddingData) error

	// FindSimilar หา videos ที่คล้ายกัน พร้อม filter
	FindSimilar(ctx context.Context, query *SimilarityQuery) ([]models.RelatedVideo, error)
}

// SimilarityQuery - Query สำหรับ filtered similarity search
type SimilarityQuery struct {
	Vector    []float32 // Query vector
	Limit     int       // Max results
	Threshold float64   // Min similarity (0-1)

	// Filters (optional)
	FilterCastID  string // ต้องมี cast คนนี้
	FilterMakerID string // ต้องเป็น maker นี้
	ExcludeID     string // ไม่เอา video นี้ (ตัวเอง)
}
