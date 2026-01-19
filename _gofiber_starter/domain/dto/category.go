package dto

import (
	"time"

	"github.com/google/uuid"
	"gofiber-template/domain/models"
)

// === Requests ===

type CreateCategoryRequest struct {
	Name     string     `json:"name" validate:"required,min=1,max=100"`
	Slug     string     `json:"slug" validate:"required,min=1,max=100"`
	ParentID *uuid.UUID `json:"parentId"`
}

type UpdateCategoryRequest struct {
	Name      *string    `json:"name" validate:"omitempty,min=1,max=100"`
	Slug      *string    `json:"slug" validate:"omitempty,min=1,max=100"`
	ParentID  *uuid.UUID `json:"parentId"`
	SortOrder *int       `json:"sortOrder"`
}

type ReorderCategoriesRequest struct {
	Categories []CategoryOrderItem `json:"categories" validate:"required,dive"`
}

type CategoryOrderItem struct {
	ID        uuid.UUID  `json:"id" validate:"required"`
	ParentID  *uuid.UUID `json:"parentId"`
	SortOrder int        `json:"sortOrder"`
}

// === Responses ===

type CategoryResponse struct {
	ID         uuid.UUID           `json:"id"`
	Name       string              `json:"name"`
	Slug       string              `json:"slug"`
	ParentID   *uuid.UUID          `json:"parentId"`
	SortOrder  int                 `json:"sortOrder"`
	VideoCount int64               `json:"videoCount"`
	CreatedAt  time.Time           `json:"createdAt"`
	Children   []*CategoryResponse `json:"children,omitempty"`
}

type CategoryListResponse struct {
	Categories []CategoryResponse `json:"categories"`
}

// === Mappers ===

func CategoryToCategoryResponse(category *models.Category) *CategoryResponse {
	if category == nil {
		return nil
	}
	return &CategoryResponse{
		ID:        category.ID,
		Name:      category.Name,
		Slug:      category.Slug,
		ParentID:  category.ParentID,
		SortOrder: category.SortOrder,
		CreatedAt: category.CreatedAt,
	}
}

func CategoryToCategoryResponseWithChildren(category *models.Category) *CategoryResponse {
	if category == nil {
		return nil
	}
	resp := &CategoryResponse{
		ID:        category.ID,
		Name:      category.Name,
		Slug:      category.Slug,
		ParentID:  category.ParentID,
		SortOrder: category.SortOrder,
		CreatedAt: category.CreatedAt,
	}
	if len(category.Children) > 0 {
		resp.Children = make([]*CategoryResponse, len(category.Children))
		for i, child := range category.Children {
			childCopy := child
			resp.Children[i] = CategoryToCategoryResponseWithChildren(&childCopy)
		}
	}
	return resp
}

func CategoriesToCategoryResponses(categories []*models.Category) []CategoryResponse {
	responses := make([]CategoryResponse, len(categories))
	for i, category := range categories {
		responses[i] = *CategoryToCategoryResponse(category)
	}
	return responses
}

func CategoriesToTreeResponses(categories []*models.Category) []CategoryResponse {
	responses := make([]CategoryResponse, len(categories))
	for i, category := range categories {
		responses[i] = *CategoryToCategoryResponseWithChildren(category)
	}
	return responses
}
