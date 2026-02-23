package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"seo-worker/domain/models"
	"seo-worker/domain/ports"
	"seo-worker/infrastructure/auth"
)

type ArticlePublisher struct {
	apiURL     string
	authClient *auth.AuthClient
	httpClient *http.Client
	logger     *slog.Logger
}

func NewArticlePublisher(apiURL string, authClient *auth.AuthClient) *ArticlePublisher {
	return &ArticlePublisher{
		apiURL:     apiURL,
		authClient: authClient,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // Increased for large payloads
		},
		logger: slog.Default().With("component", "article_publisher"),
	}
}

type apiResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// PublishArticle ส่ง article ไปบันทึกที่ api.subth.com
// ใช้ ingest endpoint สำหรับ worker
func (p *ArticlePublisher) PublishArticle(ctx context.Context, article *models.ArticleContent) error {
	url := fmt.Sprintf("%s/api/v1/articles/ingest", p.apiURL)

	// Get token from auth client
	token, err := p.authClient.GetToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get auth token: %w", err)
	}

	jsonBody, err := json.Marshal(article)
	if err != nil {
		return fmt.Errorf("failed to marshal article: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	p.logger.InfoContext(ctx, "Publishing article",
		"video_id", article.VideoID,
		"url", url,
	)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("publish request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle 401 - invalidate token and retry once
	if resp.StatusCode == http.StatusUnauthorized {
		p.authClient.InvalidateToken()
		return p.PublishArticle(ctx, article) // Retry with new token
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("publish API error: %d - %s", resp.StatusCode, string(body))
	}

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !apiResp.Success {
		return fmt.Errorf("API error: %s", apiResp.Error)
	}

	p.logger.InfoContext(ctx, "Article published",
		"video_id", article.VideoID,
	)

	return nil
}

// UpdateArticleStatus อัพเดทสถานะ article
func (p *ArticlePublisher) UpdateArticleStatus(ctx context.Context, videoID string, status string) error {
	url := fmt.Sprintf("%s/api/v1/articles/%s/status", p.apiURL, videoID)

	// Get token from auth client
	token, err := p.authClient.GetToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get auth token: %w", err)
	}

	payload := map[string]string{"status": status}
	jsonBody, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("status update request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle 401 - invalidate token and retry once
	if resp.StatusCode == http.StatusUnauthorized {
		p.authClient.InvalidateToken()
		return p.UpdateArticleStatus(ctx, videoID, status)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status update API error: %d - %s", resp.StatusCode, string(body))
	}

	p.logger.InfoContext(ctx, "Article status updated",
		"video_id", videoID,
		"status", status,
	)

	return nil
}

// Verify interface implementation
var _ ports.ArticlePublisherPort = (*ArticlePublisher)(nil)
