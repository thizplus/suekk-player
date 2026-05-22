package models

import (
	"time"

	"github.com/google/uuid"
)

// ScheduledJob represents a cron/scheduled job
// Note: เดิมชื่อ Job แต่ rename เพื่อแยกจาก WorkerJob (queue jobs)
type ScheduledJob struct {
	ID        uuid.UUID  `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Name      string     `gorm:"not null"`
	CronExpr  string     `gorm:"not null"`
	Payload   string     `gorm:"type:jsonb"`
	Status    string     `gorm:"default:'active'"`
	LastRun   *time.Time
	NextRun   *time.Time
	IsActive  bool `gorm:"default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName returns the table name (keep as "jobs" for backward compatibility)
func (ScheduledJob) TableName() string {
	return "jobs"
}
