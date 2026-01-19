package models

import (
	"time"

	"github.com/google/uuid"
)

type Category struct {
	ID        uuid.UUID  `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Name      string     `gorm:"size:100;not null"`
	Slug      string     `gorm:"size:100;uniqueIndex;not null"`
	ParentID  *uuid.UUID `gorm:"type:uuid;index"`
	SortOrder int        `gorm:"default:0"`
	CreatedAt time.Time

	// Relations
	Parent   *Category  `gorm:"foreignKey:ParentID"`
	Children []Category `gorm:"foreignKey:ParentID"`
}

func (Category) TableName() string {
	return "categories"
}
