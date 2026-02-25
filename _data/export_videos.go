package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	apiURL = "https://api.suekk.com/api/v1"
	token  = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImluZm9AdGhpenBsdXMuY29tIiwiZXhwIjoxNzcxNjI1NjkzLCJpYXQiOjE3NzEwMjA4OTMsInJvbGUiOiJ1c2VyIiwidXNlcl9pZCI6IjJiNzI4MDU0LWJhYWItNGFjOS1hNzM2LWQ2Y2JhZGRiN2E1NiIsInVzZXJuYW1lIjoiaW5mb19tZ3NicHQifQ.pzOy0R0npioJViItk9bmP7RT7X6SsGis6vWSWrT1HUc"
)

type Category struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	VideoCount int    `json:"videoCount"`
}

type Video struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Duration    int    `json:"duration"`
	Quality     string `json:"quality"`
}

type VideoResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		ID          string `json:"id"`
		Code        string `json:"code"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Status      string `json:"status"`
		Duration    int    `json:"duration"`
		Quality     string `json:"quality"`
	} `json:"data"`
	Meta struct {
		Total      int  `json:"total"`
		Page       int  `json:"page"`
		TotalPages int  `json:"totalPages"`
		HasNext    bool `json:"hasNext"`
	} `json:"meta"`
}

type CategoriesResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Categories []Category `json:"categories"`
	} `json:"data"`
}

type ExportData struct {
	Category   string  `json:"category"`
	CategoryID string  `json:"categoryId"`
	Total      int     `json:"total"`
	Videos     []Video `json:"videos"`
}

func apiGet(endpoint string) ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", apiURL+endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func getCategories() ([]Category, error) {
	data, err := apiGet("/categories")
	if err != nil {
		return nil, err
	}

	var resp CategoriesResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	return resp.Data.Categories, nil
}

func getVideosByCategory(categoryID string) ([]Video, int, error) {
	var allVideos []Video
	page := 1
	limit := 100

	for {
		endpoint := fmt.Sprintf("/videos?categoryId=%s&limit=%d&page=%d", categoryID, limit, page)
		data, err := apiGet(endpoint)
		if err != nil {
			return nil, 0, err
		}

		var resp VideoResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, 0, err
		}

		for _, v := range resp.Data {
			allVideos = append(allVideos, Video{
				ID:          v.ID,
				Code:        v.Code,
				Title:       v.Title,
				Description: v.Description,
				Status:      v.Status,
				Duration:    v.Duration,
				Quality:     v.Quality,
			})
		}

		fmt.Printf("  Page %d/%d - fetched %d videos\n", page, resp.Meta.TotalPages, len(resp.Data))

		if !resp.Meta.HasNext {
			return allVideos, resp.Meta.Total, nil
		}
		page++
	}
}

func getUncategorizedVideos() ([]Video, int, error) {
	var allVideos []Video
	page := 1
	limit := 100

	for {
		// Videos without category
		endpoint := fmt.Sprintf("/videos?limit=%d&page=%d", limit, page)
		data, err := apiGet(endpoint)
		if err != nil {
			return nil, 0, err
		}

		var resp VideoResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, 0, err
		}

		// Filter videos without category (check if category is null in raw response)
		for _, v := range resp.Data {
			allVideos = append(allVideos, Video{
				ID:          v.ID,
				Code:        v.Code,
				Title:       v.Title,
				Description: v.Description,
				Status:      v.Status,
				Duration:    v.Duration,
				Quality:     v.Quality,
			})
		}

		fmt.Printf("  Page %d/%d - fetched %d videos\n", page, resp.Meta.TotalPages, len(resp.Data))

		if !resp.Meta.HasNext {
			return allVideos, resp.Meta.Total, nil
		}
		page++
	}
}

func main() {
	outputDir := "videos_export"
	os.MkdirAll(outputDir, 0755)

	// Get categories
	fmt.Println("Fetching categories...")
	categories, err := getCategories()
	if err != nil {
		fmt.Printf("Error getting categories: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d categories\n\n", len(categories))

	// Export each category
	for _, cat := range categories {
		fmt.Printf("Fetching videos for: %s (%d videos)\n", cat.Name, cat.VideoCount)

		videos, total, err := getVideosByCategory(cat.ID)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}

		exportData := ExportData{
			Category:   cat.Name,
			CategoryID: cat.ID,
			Total:      total,
			Videos:     videos,
		}

		// Create safe filename
		filename := strings.ReplaceAll(strings.ToLower(cat.Slug), " ", "_") + ".json"
		filepath := filepath.Join(outputDir, filename)

		jsonData, err := json.MarshalIndent(exportData, "", "  ")
		if err != nil {
			fmt.Printf("  Error marshaling JSON: %v\n", err)
			continue
		}

		if err := os.WriteFile(filepath, jsonData, 0644); err != nil {
			fmt.Printf("  Error writing file: %v\n", err)
			continue
		}

		fmt.Printf("  Saved to: %s (%d videos)\n\n", filepath, len(videos))
	}

	// Also export all videos (for reference)
	fmt.Println("Fetching ALL videos...")
	allVideos, total, err := getUncategorizedVideos()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		exportData := ExportData{
			Category:   "All Videos",
			CategoryID: "",
			Total:      total,
			Videos:     allVideos,
		}

		jsonData, _ := json.MarshalIndent(exportData, "", "  ")
		filepath := filepath.Join(outputDir, "_all_videos.json")
		os.WriteFile(filepath, jsonData, 0644)
		fmt.Printf("Saved to: %s (%d videos)\n", filepath, len(allVideos))
	}

	fmt.Println("\nDone!")
}
