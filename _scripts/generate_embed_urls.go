package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Config
const (
	// API URL
	API_BASE_URL = "https://api.suekk.com"
	// Embed URL base
	EMBED_BASE_URL = "https://play.suekk.com/embed"
	// Input/Output files
	INPUT_CSV  = "../_gofiber_starter/AVMONO - censored.csv"
	OUTPUT_CSV = "../_gofiber_starter/AVMONO - censored_with_embed.csv"
	// Concurrency
	WORKERS = 20
)

type LoginResponse struct {
	Success bool `json:"success"`
	Data    *struct {
		Token string `json:"token"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type VideoResponse struct {
	Success bool `json:"success"`
	Data    *struct {
		Code   string `json:"code"`
		Status string `json:"status"`
	} `json:"data"`
}

type VideoListResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		ID     string `json:"id"`
		Code   string `json:"code"`
		Title  string `json:"title"`
		Status string `json:"status"`
	} `json:"data"`
}

type Result struct {
	Index    int
	Code     string
	Found    bool
	EmbedURL string
}

var authToken string

func main() {
	// Credentials for api.suekk.com
	email := "info@thizplus.com"
	password := "Log2Window$P@ssWord"

	fmt.Printf("Logging in as %s...\n", email)

	// Login to get token
	token, err := login(email, password)
	if err != nil {
		log.Fatal("Failed to login:", err)
	}
	authToken = token
	fmt.Println("Login successful!")

	// Read input CSV
	inputFile, err := os.Open(INPUT_CSV)
	if err != nil {
		log.Fatal("Failed to open input CSV:", err)
	}
	defer inputFile.Close()

	reader := csv.NewReader(inputFile)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatal("Failed to read CSV:", err)
	}
	fmt.Printf("Read %d records from CSV\n", len(records))

	// Create channels
	jobs := make(chan struct {
		Index int
		Code  string
	}, len(records))
	results := make(chan Result, len(records))

	// Start workers
	var wg sync.WaitGroup
	client := &http.Client{Timeout: 10 * time.Second}

	for w := 0; w < WORKERS; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				found, embedURL := checkVideo(client, job.Code)
				results <- Result{
					Index:    job.Index,
					Code:     job.Code,
					Found:    found,
					EmbedURL: embedURL,
				}
			}
		}()
	}

	// Send jobs
	go func() {
		for i, record := range records {
			if i == 0 || len(record) < 4 {
				continue
			}
			code := strings.TrimSpace(record[2])
			if code != "" {
				jobs <- struct {
					Index int
					Code  string
				}{i, code}
			}
		}
		close(jobs)
	}()

	// Wait for workers to finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	matchCount := 0
	notFoundCodes := []string{}
	processed := 0
	total := len(records) - 1

	for result := range results {
		processed++
		if result.Found {
			records[result.Index][3] = result.EmbedURL
			matchCount++
		} else {
			notFoundCodes = append(notFoundCodes, result.Code)
		}

		// Progress
		if processed%100 == 0 {
			fmt.Printf("\rProcessed: %d/%d (%.1f%%) - Found: %d", processed, total, float64(processed)/float64(total)*100, matchCount)
		}
	}

	fmt.Printf("\n\nMatched %d videos\n", matchCount)
	fmt.Printf("Not found: %d videos\n", len(notFoundCodes))

	// Show first 20 not found codes
	if len(notFoundCodes) > 0 && len(notFoundCodes) <= 50 {
		fmt.Println("\nNot found codes:")
		for _, code := range notFoundCodes {
			fmt.Printf("  - %s\n", code)
		}
	} else if len(notFoundCodes) > 50 {
		fmt.Println("\nFirst 20 not found codes:")
		for i := 0; i < 20; i++ {
			fmt.Printf("  - %s\n", notFoundCodes[i])
		}
	}

	// Write output CSV
	outputFile, err := os.Create(OUTPUT_CSV)
	if err != nil {
		log.Fatal("Failed to create output CSV:", err)
	}
	defer outputFile.Close()

	writer := csv.NewWriter(outputFile)
	if err := writer.WriteAll(records); err != nil {
		log.Fatal("Failed to write CSV:", err)
	}

	fmt.Printf("\nOutput written to %s\n", OUTPUT_CSV)
	fmt.Printf("Summary: %d/%d videos matched (%.1f%%)\n", matchCount, total, float64(matchCount)/float64(total)*100)
}

func login(email, password string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/auth/login", API_BASE_URL)

	payload := map[string]string{
		"email":    email,
		"password": password,
	}

	jsonData, _ := json.Marshal(payload)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", err
	}

	if !loginResp.Success || loginResp.Data == nil {
		msg := "unknown error"
		if loginResp.Error != nil {
			msg = loginResp.Error.Message
		}
		return "", fmt.Errorf("login failed: %s", msg)
	}

	return loginResp.Data.Token, nil
}

func checkVideo(client *http.Client, code string) (bool, string) {
	// Search by title (video code) with status=ready
	searchURL := fmt.Sprintf("%s/api/v1/videos?search=%s&status=ready&limit=1", API_BASE_URL, code)

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return false, ""
	}
	req.Header.Set("Authorization", "Bearer "+authToken)

	resp, err := client.Do(req)
	if err != nil {
		return false, ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false, ""
	}

	var listResp VideoListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return false, ""
	}

	// Check if we found a video with matching title
	if listResp.Success && len(listResp.Data) > 0 {
		video := listResp.Data[0]
		// Title should contain the code (case insensitive)
		if strings.Contains(strings.ToUpper(video.Title), strings.ToUpper(code)) {
			embedURL := fmt.Sprintf("%s/%s", EMBED_BASE_URL, video.Code)
			return true, embedURL
		}
	}

	return false, ""
}
