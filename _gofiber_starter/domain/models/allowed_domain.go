package models

import (
	"time"

	"github.com/google/uuid"
)

// AllowedDomain สำหรับ iframe whitelist
type AllowedDomain struct {
	ID        uuid.UUID `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	VideoID   uuid.UUID `gorm:"type:uuid;not null"`
	Domain    string    `gorm:"size:255;not null"`
	CreatedAt time.Time

	// Relations
	Video *Video `gorm:"foreignKey:VideoID"`
}

func (AllowedDomain) TableName() string {
	return "allowed_domains"
}
