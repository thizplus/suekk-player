package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"seo-worker/domain/models"
	"seo-worker/domain/ports"
	"seo-worker/infrastructure/auth"
)

// videoCodeRegex - สกัด video code จริงจาก title (เช่น DLDSS-471, ABP-123, SSIS-001)
var videoCodeRegex = regexp.MustCompile(`^([A-Z]{2,10}-\d{2,5})`)

type MetadataFetcher struct {
	apiURL     string
	authClient *auth.AuthClient
	httpClient *http.Client
	logger     *slog.Logger
}

func NewMetadataFetcher(apiURL string, authClient *auth.AuthClient) *MetadataFetcher {
	return &MetadataFetcher{
		apiURL:     apiURL,
		authClient: authClient,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: slog.Default().With("component", "metadata_fetcher"),
	}
}

type apiResponse[T any] struct {
	Success bool   `json:"success"`
	Data    T      `json:"data"`
	Error   string `json:"error,omitempty"`
}

// paginatedResponse - response จาก paginated endpoints
type paginatedResponse[T any] struct {
	Success bool   `json:"success"`
	Data    T      `json:"data"`
	Meta    struct {
		Total      int  `json:"total"`
		Page       int  `json:"page"`
		Limit      int  `json:"limit"`
		TotalPages int  `json:"totalPages"`
		HasNext    bool `json:"hasNext"`
		HasPrev    bool `json:"hasPrev"`
	} `json:"meta"`
	Error string `json:"error,omitempty"`
}

// videoIDResponse - response จาก find-by-codes
type videoIDResponse struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	EmbedURL string `json:"embed_url"`
}

// videoFullResponse - response จาก /videos/:id (gofiber format)
type videoFullResponse struct {
	ID          string                `json:"id"`
	Code        string                `json:"code"`
	Title       string                `json:"title"`
	Thumbnail   string                `json:"thumbnail"`
	EmbedURL    string                `json:"embedUrl"`
	ReleaseDate string                `json:"releaseDate"`
	Maker       *makerResponseNested  `json:"maker"`
	Casts       []castResponseNested  `json:"casts"`
	Tags        []tagResponseNested   `json:"tags"`
	Categories  []categoryResponseNested `json:"categories"`
}

type makerResponseNested struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type castResponseNested struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type tagResponseNested struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type categoryResponseNested struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// FetchVideoMetadataByCode ดึง metadata โดยใช้ video code (embed code)
// Step 1: เรียก /find-by-codes เพื่อได้ video ID
// Step 2: เรียก /videos/:id เพื่อได้ full metadata
func (f *MetadataFetcher) FetchVideoMetadataByCode(ctx context.Context, videoCode string) (*models.VideoMetadata, error) {
	// Step 1: Get video ID from code
	findURL := fmt.Sprintf("%s/api/v1/videos/find-by-codes", f.apiURL)
	reqBody := map[string][]string{
		"codes": {videoCode},
	}

	var findResp apiResponse[[]videoIDResponse]
	if err := f.doPostRequest(ctx, findURL, reqBody, &findResp); err != nil {
		return nil, fmt.Errorf("find-by-codes failed: %w", err)
	}

	if !findResp.Success || len(findResp.Data) == 0 {
		return nil, fmt.Errorf("video not found for code: %s", videoCode)
	}

	videoID := findResp.Data[0].ID
	f.logger.InfoContext(ctx, "Found video ID from code",
		"video_code", videoCode,
		"video_id", videoID,
	)

	// Step 2: Get full video metadata
	videoURL := fmt.Sprintf("%s/api/v1/videos/%s", f.apiURL, videoID)

	var videoResp apiResponse[videoFullResponse]
	if err := f.doRequest(ctx, videoURL, &videoResp); err != nil {
		return nil, fmt.Errorf("get video failed: %w", err)
	}

	if !videoResp.Success {
		return nil, fmt.Errorf("API error: %s", videoResp.Error)
	}

	video := videoResp.Data

	// Convert to VideoMetadata format
	metadata := &models.VideoMetadata{
		ID:          video.ID,
		Code:        video.Code,
		Title:       video.Title,
		Thumbnail:   video.Thumbnail,
		ReleaseDate: video.ReleaseDate,
	}

	// Extract real video code from title (e.g., "DLDSS-471 Sensitive..." → "DLDSS-471")
	metadata.RealCode = extractVideoCode(video.Title)
	f.logger.InfoContext(ctx, "[DEBUG] Video code extraction",
		"title", video.Title,
		"internal_code", video.Code,
		"extracted_real_code", metadata.RealCode,
	)
	if metadata.RealCode == "" {
		// Fallback: ใช้ internal code ถ้าสกัดไม่ได้
		metadata.RealCode = video.Code
		f.logger.WarnContext(ctx, "[DEBUG] Using internal code as fallback")
	}

	// Extract Maker (nested + ID)
	if video.Maker != nil {
		metadata.MakerID = video.Maker.ID
		metadata.Maker = &models.MakerMetadata{
			ID:   video.Maker.ID,
			Name: video.Maker.Name,
			Slug: video.Maker.Slug,
		}
	}

	// Extract Casts (nested + IDs)
	metadata.CastIDs = make([]string, len(video.Casts))
	metadata.Casts = make([]models.CastMetadata, len(video.Casts))
	for i, cast := range video.Casts {
		metadata.CastIDs[i] = cast.ID
		metadata.Casts[i] = models.CastMetadata{
			ID:   cast.ID,
			Name: cast.Name,
			Slug: cast.Slug,
		}
	}

	// Extract Tags (nested + IDs)
	metadata.TagIDs = make([]string, len(video.Tags))
	metadata.Tags = make([]models.TagMetadata, len(video.Tags))
	for i, tag := range video.Tags {
		metadata.TagIDs[i] = tag.ID
		metadata.Tags[i] = models.TagMetadata{
			ID:   tag.ID,
			Name: tag.Name,
			Slug: tag.Slug,
		}
	}

	// Extract first CategoryID (for compatibility)
	if len(video.Categories) > 0 {
		metadata.CategoryID = video.Categories[0].ID
	}

	f.logger.InfoContext(ctx, "Video metadata fetched",
		"internal_code", videoCode,
		"real_code", metadata.RealCode,
		"video_id", metadata.ID,
		"casts_count", len(metadata.CastIDs),
		"tags_count", len(metadata.TagIDs),
		"thumbnail", metadata.Thumbnail,
		"release_date", metadata.ReleaseDate,
	)

	return metadata, nil
}

func (f *MetadataFetcher) FetchCasts(ctx context.Context, castIDs []string) ([]models.CastMetadata, error) {
	if len(castIDs) == 0 {
		return nil, nil
	}

	// Batch fetch: /api/v1/casts?ids=id1,id2,id3
	url := fmt.Sprintf("%s/api/v1/casts?ids=%s", f.apiURL, strings.Join(castIDs, ","))

	var resp apiResponse[[]models.CastMetadata]
	if err := f.doRequest(ctx, url, &resp); err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("API error: %s", resp.Error)
	}

	f.logger.InfoContext(ctx, "Casts fetched",
		"count", len(resp.Data),
	)

	return resp.Data, nil
}

// articleSummaryForPreviousWorks - response จาก /api/v1/articles/cast/{slug}
type articleSummaryForPreviousWorks struct {
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	VideoCode string `json:"videoCode"`
}

// FetchPreviousWorks ดึงผลงานก่อนหน้าของ cast จาก articles endpoint
// ใช้ castSlug (ไม่ใช่ ID) เพื่อเรียก /api/v1/articles/cast/{slug}
func (f *MetadataFetcher) FetchPreviousWorks(ctx context.Context, castSlug string, limit int) ([]models.PreviousWork, error) {
	if castSlug == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}

	// ใช้ articles/cast endpoint แทน (มีอยู่แล้ว)
	url := fmt.Sprintf("%s/api/v1/articles/cast/%s?limit=%d&lang=th", f.apiURL, castSlug, limit)

	var resp paginatedResponse[[]articleSummaryForPreviousWorks]
	if err := f.doRequest(ctx, url, &resp); err != nil {
		f.logger.WarnContext(ctx, "Failed to fetch previous works",
			"cast_slug", castSlug,
			"error", err,
		)
		return nil, nil // ไม่ error เพราะอาจยังไม่มี articles
	}

	if !resp.Success {
		return nil, nil
	}

	// Map to PreviousWork
	works := make([]models.PreviousWork, 0, len(resp.Data))
	for _, article := range resp.Data {
		works = append(works, models.PreviousWork{
			VideoCode: article.VideoCode, // Internal code (e.g., "3993bp6j")
			Slug:      article.Slug,      // Article slug (e.g., "dass-541")
			Title:     article.Title,
		})
	}

	f.logger.InfoContext(ctx, "Previous works fetched",
		"cast_slug", castSlug,
		"count", len(works),
	)

	return works, nil
}

func (f *MetadataFetcher) FetchGalleryImages(ctx context.Context, videoID string) ([]models.GalleryImage, error) {
	url := fmt.Sprintf("%s/api/v1/videos/%s/gallery", f.apiURL, videoID)

	var resp apiResponse[[]models.GalleryImage]
	if err := f.doRequest(ctx, url, &resp); err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("API error: %s", resp.Error)
	}

	return resp.Data, nil
}

func (f *MetadataFetcher) FetchMaker(ctx context.Context, makerID string) (*models.MakerMetadata, error) {
	if makerID == "" {
		return nil, nil
	}

	url := fmt.Sprintf("%s/api/v1/makers/%s", f.apiURL, makerID)

	var resp apiResponse[models.MakerMetadata]
	if err := f.doRequest(ctx, url, &resp); err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("API error: %s", resp.Error)
	}

	f.logger.InfoContext(ctx, "Maker fetched",
		"maker_id", makerID,
		"maker_name", resp.Data.Name,
	)

	return &resp.Data, nil
}

func (f *MetadataFetcher) FetchTags(ctx context.Context, tagIDs []string) ([]models.TagMetadata, error) {
	if len(tagIDs) == 0 {
		return nil, nil
	}

	// Batch fetch: /api/v1/tags?ids=id1,id2,id3
	url := fmt.Sprintf("%s/api/v1/tags?ids=%s", f.apiURL, strings.Join(tagIDs, ","))

	var resp apiResponse[[]models.TagMetadata]
	if err := f.doRequest(ctx, url, &resp); err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("API error: %s", resp.Error)
	}

	f.logger.InfoContext(ctx, "Tags fetched",
		"count", len(resp.Data),
	)

	return resp.Data, nil
}

func (f *MetadataFetcher) doPostRequest(ctx context.Context, url string, body any, result any) error {
	// Get token from auth client
	token, err := f.authClient.GetToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get auth token: %w", err)
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle 401 - invalidate token and retry once
	if resp.StatusCode == http.StatusUnauthorized {
		f.authClient.InvalidateToken()
		return f.doPostRequest(ctx, url, body, result)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

func (f *MetadataFetcher) doRequest(ctx context.Context, url string, result any) error {
	// Get token from auth client
	token, err := f.authClient.GetToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get auth token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle 401 - invalidate token and retry once
	if resp.StatusCode == http.StatusUnauthorized {
		f.authClient.InvalidateToken()
		return f.doRequest(ctx, url, result) // Retry with new token
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// extractVideoCode สกัด video code จริงจาก title
// เช่น "DLDSS-471 Sensitive, Caution..." → "DLDSS-471"
func extractVideoCode(title string) string {
	matches := videoCodeRegex.FindStringSubmatch(title)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// Verify interface implementation
var _ ports.MetadataFetcherPort = (*MetadataFetcher)(nil)
