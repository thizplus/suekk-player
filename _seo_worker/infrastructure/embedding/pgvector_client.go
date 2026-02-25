package embedding

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/lib/pq"
	"github.com/pgvector/pgvector-go"

	"seo-worker/domain/models"
	"seo-worker/domain/ports"
)

type PgVectorClient struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewPgVectorClient(db *sql.DB) *PgVectorClient {
	return &PgVectorClient{
		db:     db,
		logger: slog.Default().With("component", "pgvector"),
	}
}

// GenerateEmbedding สร้าง embedding vector จาก text
// TODO: ใช้ Gemini text-embedding-004 หรือ OpenAI ada-002
func (c *PgVectorClient) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Skip if DB is nil (testing mode)
	if c.db == nil {
		c.logger.WarnContext(ctx, "Skipping embedding generation - DB is nil (testing mode)")
		return nil, nil
	}

	// Placeholder: ต้อง implement เรียก embedding API
	// ตอนนี้ return dummy vector
	c.logger.WarnContext(ctx, "Using placeholder embedding - implement real embedding API")

	// Return 1536-dim zero vector as placeholder
	vector := make([]float32, 1536)
	return vector, nil
}

// StoreEmbedding บันทึก embedding ลง pgvector
// ใช้ ON CONFLICT สำหรับ idempotency
func (c *PgVectorClient) StoreEmbedding(ctx context.Context, data *models.EmbeddingData) error {
	// Skip if DB is nil (testing mode)
	if c.db == nil {
		c.logger.WarnContext(ctx, "Skipping embedding storage - DB is nil (testing mode)")
		return nil
	}

	query := `
		INSERT INTO article_embeddings (video_id, embedding, cast_ids, maker_id, tag_ids, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (video_id) DO UPDATE SET
			embedding = EXCLUDED.embedding,
			cast_ids = EXCLUDED.cast_ids,
			maker_id = EXCLUDED.maker_id,
			tag_ids = EXCLUDED.tag_ids,
			updated_at = NOW()
	`

	vec := pgvector.NewVector(data.Vector)

	_, err := c.db.ExecContext(ctx, query,
		data.VideoID,
		vec,
		pq.Array(data.CastIDs),
		data.MakerID,
		pq.Array(data.TagIDs),
		data.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to store embedding: %w", err)
	}

	c.logger.InfoContext(ctx, "Embedding stored",
		"video_id", data.VideoID,
		"vector_dim", len(data.Vector),
	)

	return nil
}

// FindSimilar หา videos ที่คล้ายกัน
func (c *PgVectorClient) FindSimilar(ctx context.Context, query *ports.SimilarityQuery) ([]models.RelatedVideo, error) {
	// Skip if DB is nil (testing mode)
	if c.db == nil {
		c.logger.WarnContext(ctx, "Skipping similarity search - DB is nil (testing mode)")
		return nil, nil
	}

	vec := pgvector.NewVector(query.Vector)

	sqlQuery := `
		SELECT
			v.id,
			v.code,
			v.title,
			v.thumbnail,
			1 - (e.embedding <=> $1) as similarity
		FROM article_embeddings e
		JOIN videos v ON v.id = e.video_id
		WHERE v.id != $2
	`
	args := []any{vec, query.ExcludeID}
	argIdx := 3

	// Add filters
	if query.FilterCastID != "" {
		sqlQuery += fmt.Sprintf(" AND $%d = ANY(e.cast_ids)", argIdx)
		args = append(args, query.FilterCastID)
		argIdx++
	}

	if query.FilterMakerID != "" {
		sqlQuery += fmt.Sprintf(" AND e.maker_id = $%d", argIdx)
		args = append(args, query.FilterMakerID)
		argIdx++
	}

	if query.Threshold > 0 {
		sqlQuery += fmt.Sprintf(" AND 1 - (e.embedding <=> $1) >= $%d", argIdx)
		args = append(args, query.Threshold)
		argIdx++
	}

	sqlQuery += " ORDER BY e.embedding <=> $1"
	sqlQuery += fmt.Sprintf(" LIMIT $%d", argIdx)
	args = append(args, query.Limit)

	rows, err := c.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query similar videos: %w", err)
	}
	defer rows.Close()

	var results []models.RelatedVideo
	for rows.Next() {
		var r models.RelatedVideo
		if err := rows.Scan(&r.ID, &r.Code, &r.Title, &r.ThumbnailURL, &r.Similarity); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		r.URL = fmt.Sprintf("/videos/%s", r.Code)
		results = append(results, r)
	}

	return results, rows.Err()
}

// Verify interface implementation
var _ ports.EmbeddingPort = (*PgVectorClient)(nil)
