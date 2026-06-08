package dto

import (
	"time"

	"github.com/google/uuid"
	"gofiber-template/domain/models"
)

// ═══════════════════════════════════════════
// Requests
// ═══════════════════════════════════════════

type CreateSeriesRequest struct {
	Title            string     `json:"title" validate:"required,min=1,max=500"`
	ThaiTitle        string     `json:"thaiTitle" validate:"max=500"`
	Slug             string     `json:"slug" validate:"required,min=1,max=255"`
	Description      string     `json:"description" validate:"max=10000"`
	Year             int        `json:"year"`
	Rating           float64    `json:"rating"`
	Quality          string     `json:"quality"`
	AudioType        string     `json:"audioType"`
	TrailerYoutubeID string     `json:"trailerYoutubeId"`
	TotalEpisodes    int        `json:"totalEpisodes"`
	IsCompleted      bool       `json:"isCompleted"`
	CategoryID       *uuid.UUID `json:"categoryId"`
	SourceSite       string     `json:"sourceSite"`
	SourceID         int        `json:"sourceId"`
	SourceURL        string     `json:"sourceUrl"`
}

type UpdateSeriesRequest struct {
	Title            *string    `json:"title" validate:"omitempty,min=1,max=500"`
	ThaiTitle        *string    `json:"thaiTitle" validate:"omitempty,max=500"`
	Description      *string    `json:"description" validate:"omitempty,max=10000"`
	Year             *int       `json:"year"`
	Rating           *float64   `json:"rating"`
	Quality          *string    `json:"quality"`
	AudioType        *string    `json:"audioType"`
	TrailerYoutubeID *string    `json:"trailerYoutubeId"`
	TotalEpisodes    *int       `json:"totalEpisodes"`
	IsCompleted      *bool      `json:"isCompleted"`
	CategoryID       *uuid.UUID `json:"categoryId"`
	Status           *string    `json:"status"`
	PosterPath       *string    `json:"posterPath"`
}

// Upsert: สร้างหรืออัพเดท by source_site + source_id
type UpsertSeriesRequest struct {
	CreateSeriesRequest
}

type AddEpisodesRequest struct {
	Episodes []EpisodeItem `json:"episodes" validate:"required,dive"`
}

type EpisodeItem struct {
	EpisodeNumber int    `json:"episodeNumber" validate:"required,min=1"`
	SourceURL     string `json:"sourceUrl"`
}

type UpdateEpisodeRequest struct {
	VideoID   *uuid.UUID `json:"videoId"`
	Status    *string    `json:"status"`
	SourceURL *string    `json:"sourceUrl"`
}

type SeriesFilterRequest struct {
	Search     string `query:"search"`
	CategoryID string `query:"categoryId"`
	AudioType  string `query:"audioType"`
	Year       int    `query:"year"`
	Status     string `query:"status"`
	SortBy     string `query:"sortBy"`     // created_at | year | rating | title
	SortOrder  string `query:"sortOrder"`  // asc | desc
	Page       int    `query:"page"`
	Limit      int    `query:"limit"`
}

// ═══════════════════════════════════════════
// Responses
// ═══════════════════════════════════════════

type SeriesResponse struct {
	ID               uuid.UUID                `json:"id"`
	Code             string                   `json:"code"`
	Title            string                   `json:"title"`
	ThaiTitle        string                   `json:"thaiTitle"`
	Slug             string                   `json:"slug"`
	Description      string                   `json:"description"`
	PosterPath       string                   `json:"posterPath"`
	Year             int                      `json:"year"`
	Rating           float64                  `json:"rating"`
	Quality          string                   `json:"quality"`
	AudioType        string                   `json:"audioType"`
	TrailerYoutubeID string                   `json:"trailerYoutubeId"`
	TotalEpisodes    int                      `json:"totalEpisodes"`
	IsCompleted      bool                     `json:"isCompleted"`
	Status           string                   `json:"status"`
	Category         *SeriesCategoryResponse  `json:"category,omitempty"`
	Episodes         []SeriesEpisodeResponse  `json:"episodes,omitempty"`
	CreatedAt        time.Time                `json:"createdAt"`
	UpdatedAt        time.Time                `json:"updatedAt"`
}

type SeriesEpisodeResponse struct {
	ID            uuid.UUID `json:"id"`
	EpisodeNumber int       `json:"episodeNumber"`
	VideoCode     string    `json:"videoCode,omitempty"`
	VideoStatus   string    `json:"videoStatus,omitempty"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"createdAt"`
}

type SeriesCategoryResponse struct {
	ID       uuid.UUID                 `json:"id"`
	Name     string                    `json:"name"`
	Slug     string                    `json:"slug"`
	ParentID *uuid.UUID                `json:"parentId,omitempty"`
	Children []SeriesCategoryResponse  `json:"children,omitempty"`
}

type SeriesListResponse struct {
	Series []SeriesResponse `json:"series"`
}

// ═══════════════════════════════════════════
// Mappers
// ═══════════════════════════════════════════

func SeriesToSeriesResponse(s *models.Series) *SeriesResponse {
	if s == nil {
		return nil
	}

	resp := &SeriesResponse{
		ID:               s.ID,
		Code:             s.Code,
		Title:            s.Title,
		ThaiTitle:        s.ThaiTitle,
		Slug:             s.Slug,
		Description:      s.Description,
		PosterPath:       s.PosterPath,
		Year:             s.Year,
		Rating:           s.Rating,
		Quality:          s.Quality,
		AudioType:        s.AudioType,
		TrailerYoutubeID: s.TrailerYoutubeID,
		TotalEpisodes:    s.TotalEpisodes,
		IsCompleted:      s.IsCompleted,
		Status:           s.Status,
		CreatedAt:        s.CreatedAt,
		UpdatedAt:        s.UpdatedAt,
	}

	if s.SeriesCategory != nil {
		resp.Category = SeriesCategoryToResponse(s.SeriesCategory)
	}

	if s.Episodes != nil {
		resp.Episodes = make([]SeriesEpisodeResponse, len(s.Episodes))
		for i, ep := range s.Episodes {
			resp.Episodes[i] = *SeriesEpisodeToResponse(&ep)
		}
	}

	return resp
}

func SeriesEpisodeToResponse(ep *models.SeriesEpisode) *SeriesEpisodeResponse {
	if ep == nil {
		return nil
	}

	resp := &SeriesEpisodeResponse{
		ID:            ep.ID,
		EpisodeNumber: ep.EpisodeNumber,
		Status:        ep.Status,
		CreatedAt:     ep.CreatedAt,
	}

	if ep.Video != nil {
		resp.VideoCode = ep.Video.Code
		resp.VideoStatus = string(ep.Video.Status)
	}

	return resp
}

func SeriesCategoryToResponse(c *models.SeriesCategory) *SeriesCategoryResponse {
	if c == nil {
		return nil
	}

	resp := &SeriesCategoryResponse{
		ID:       c.ID,
		Name:     c.Name,
		Slug:     c.Slug,
		ParentID: c.ParentID,
	}

	if c.Children != nil {
		resp.Children = make([]SeriesCategoryResponse, len(c.Children))
		for i, child := range c.Children {
			resp.Children[i] = *SeriesCategoryToResponse(&child)
		}
	}

	return resp
}

func SeriesToSeriesResponses(seriesList []*models.Series) []SeriesResponse {
	responses := make([]SeriesResponse, len(seriesList))
	for i, s := range seriesList {
		responses[i] = *SeriesToSeriesResponse(s)
	}
	return responses
}
