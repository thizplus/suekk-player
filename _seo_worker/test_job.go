//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

type SEOArticleJob struct {
	VideoID     string `json:"video_id"`
	VideoCode   string `json:"video_code"`
	Priority    int    `json:"priority"`
	GenerateTTS bool   `json:"generate_tts"`
	CreatedAt   int64  `json:"created_at"`
}

func main() {
	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		log.Fatal(err)
	}

	job := SEOArticleJob{
		VideoID:     "test-video-id",
		VideoCode:   "utywgage",
		Priority:    1,
		GenerateTTS: true,
		CreatedAt:   time.Now().Unix(),
	}

	data, _ := json.Marshal(job)

	_, err = js.Publish("seo.article.generate", data)
	if err != nil {
		log.Fatal("Failed to publish:", err)
	}

	fmt.Println("Job published successfully!")
	fmt.Printf("Video Code: %s\n", job.VideoCode)
}
