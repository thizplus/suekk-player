package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// StringArray สำหรับเก็บ JSON array ใน PostgreSQL (JSONB)
type StringArray []string

func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	return json.Marshal(a)
}

func (a *StringArray) Scan(value interface{}) error {
	if value == nil {
		*a = StringArray{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, a)
}

type Series struct {
	ID                uuid.UUID  `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Code              string     `gorm:"size:50;uniqueIndex;not null"`
	Title             string     `gorm:"size:500;not null"`
	ThaiTitle         string     `gorm:"size:500;default:''"`
	Slug              string     `gorm:"size:255;uniqueIndex;not null"`
	Description       string     `gorm:"type:text;default:''"`
	PosterPath        string     `gorm:"size:500;default:''"` // S3 path: series/{code}/poster.jpg
	Year              int        `gorm:"default:0"`
	Rating            float64    `gorm:"type:decimal(3,1);default:0"`
	Quality           string     `gorm:"size:20;default:'HD'"`
	AudioType         string     `gorm:"size:50;default:''"` // "Thai" | "Sound Track"
	TrailerYoutubeID  string     `gorm:"size:20;default:''"`
	TotalEpisodes     int        `gorm:"default:0"`
	IsCompleted       bool       `gorm:"default:false"`
	SeriesCategoryID  *uuid.UUID `gorm:"type:uuid;index"`
	Platforms         StringArray `gorm:"type:jsonb;default:'[]'"` // ["NETFLIX", "VIU", "HBO"]
	Genres            StringArray `gorm:"type:jsonb;default:'[]'"` // ["Drama", "Comedy", "Action"]
	Status            string      `gorm:"size:20;default:'active';index"` // active | hidden | draft

	// Source tracking (Bot ใช้ป้องกันซ้ำ)
	SourceSite string `gorm:"size:100;default:''"`
	SourceID   int    `gorm:"default:0"`
	SourceURL  string `gorm:"size:1000;default:''"`

	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time

	// Relations
	SeriesCategory *SeriesCategory  `gorm:"foreignKey:SeriesCategoryID"`
	Episodes       []SeriesEpisode  `gorm:"foreignKey:SeriesID"`
}

func (Series) TableName() string {
	return "series"
}

type SeriesEpisode struct {
	ID            uuid.UUID  `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	SeriesID      uuid.UUID  `gorm:"type:uuid;not null;index"`
	EpisodeNumber int        `gorm:"not null"`
	VideoID       *uuid.UUID `gorm:"type:uuid;index"` // FK ไป videos (nullable = ยังไม่ upload)
	SourceURL     string     `gorm:"size:1000;default:''"`
	Status        string     `gorm:"size:20;default:'pending';index"` // pending | uploading | ready | failed

	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time

	// Relations
	Series *Series `gorm:"foreignKey:SeriesID"`
	Video  *Video  `gorm:"foreignKey:VideoID"`
}

func (SeriesEpisode) TableName() string {
	return "series_episodes"
}

type SeriesCategory struct {
	ID        uuid.UUID  `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Name      string     `gorm:"size:100;not null"`
	Slug      string     `gorm:"size:100;uniqueIndex;not null"`
	ParentID  *uuid.UUID `gorm:"type:uuid;index"`
	SortOrder int        `gorm:"default:0"`
	CreatedAt time.Time

	// Relations
	Parent   *SeriesCategory  `gorm:"foreignKey:ParentID"`
	Children []SeriesCategory `gorm:"foreignKey:ParentID"`
}

func (SeriesCategory) TableName() string {
	return "series_categories"
}
