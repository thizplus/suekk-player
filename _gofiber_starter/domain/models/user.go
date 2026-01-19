package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	GoogleID  *string   `gorm:"size:255;index"` // สำหรับ Google OAuth (nullable, partial unique via migration)
	Email     string    `gorm:"uniqueIndex;not null"`
	Username  string    `gorm:"uniqueIndex;not null"`
	Password  string    // nullable สำหรับ Google OAuth users
	FirstName string
	LastName  string
	Avatar    string
	Role      string `gorm:"default:'user'"` // user, admin
	IsActive  bool   `gorm:"default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (User) TableName() string {
	return "users"
}

// IsGoogleUser ตรวจสอบว่าเป็น user ที่ login ด้วย Google
func (u *User) IsGoogleUser() bool {
	return u.GoogleID != nil && *u.GoogleID != ""
}

// IsAdmin ตรวจสอบว่าเป็น admin
func (u *User) IsAdmin() bool {
	return u.Role == "admin"
}