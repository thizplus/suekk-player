package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Config from .env
const (
	// PostgreSQL
	DB_HOST     = "localhost"
	DB_PORT     = "5432"
	DB_USER     = "postgres"
	DB_PASSWORD = "n147369"
	DB_NAME     = "suekk_stream"

	// NATS
	NATS_URL    = "nats://localhost:4222"
	STREAM_NAME = "TRANSCODE_STREAM"
)

func main() {
	fmt.Println("============================================")
	fmt.Println("  SUEKK Stream - Clear All Data")
	fmt.Println("============================================")
	fmt.Println()

	// 1. Clear PostgreSQL
	clearPostgreSQL()

	// 2. Clear NATS JetStream
	clearNATS()

	fmt.Println()
	fmt.Println("============================================")
	fmt.Println("  Done! Ready for fresh testing.")
	fmt.Println("============================================")
}

func clearPostgreSQL() {
	fmt.Println("[1/2] Clearing PostgreSQL...")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		DB_HOST, DB_USER, DB_PASSWORD, DB_NAME, DB_PORT)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("     Failed to connect to database: %v\n", err)
		return
	}

	// Tables to truncate (order matters for foreign keys)
	tables := []string{
		"ad_impressions",
		"streaming_sessions",
		"daily_stats",
		"preroll_ads",
		"profile_domains",
		"allowed_domains",
		"whitelist_profiles",
		"videos",
		"categories",
		"files",
	}

	// Truncate each table
	for _, table := range tables {
		result := db.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if result.Error != nil {
			fmt.Printf("     Warning: Could not truncate %s: %v\n", table, result.Error)
		}
	}

	fmt.Println("     PostgreSQL cleared successfully!")
}

func clearNATS() {
	fmt.Println("[2/2] Clearing NATS JetStream...")

	// Connect to NATS
	nc, err := nats.Connect(NATS_URL)
	if err != nil {
		fmt.Printf("     NATS not available: %v (skipping)\n", err)
		return
	}
	defer nc.Close()

	// Get JetStream context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	js, err := jetstream.New(nc)
	if err != nil {
		fmt.Printf("     JetStream not available: %v\n", err)
		return
	}

	// Try to purge the stream
	stream, err := js.Stream(ctx, STREAM_NAME)
	if err != nil {
		fmt.Printf("     Stream '%s' not found (OK - nothing to clear)\n", STREAM_NAME)
		return
	}

	// Purge all messages
	err = stream.Purge(ctx)
	if err != nil {
		fmt.Printf("     Failed to purge stream: %v\n", err)
		return
	}

	// Get stream info to show result
	info, _ := stream.Info(ctx)
	fmt.Printf("     NATS stream purged! Messages: %d\n", info.State.Msgs)
}

func init() {
	// Suppress GORM logs
	os.Setenv("GORM_SILENCE", "true")
}
