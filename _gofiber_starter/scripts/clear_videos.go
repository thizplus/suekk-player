package main

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Connection string ตาม .env
	dsn := "host=localhost user=postgres password=n147369 dbname=suekk_stream port=5432 sslmode=disable"

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Delete all videos
	result := db.Exec("DELETE FROM videos")
	if result.Error != nil {
		log.Fatal("Failed to delete videos:", result.Error)
	}

	fmt.Printf("Deleted %d videos\n", result.RowsAffected)

	// Clear MinIO data as well (optional)
	fmt.Println("Done! Videos table cleared.")
}
