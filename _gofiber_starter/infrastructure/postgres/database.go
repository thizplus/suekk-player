package postgres

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gofiber-template/domain/models"
)

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

func NewDatabase(config DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC",
		config.Host, config.User, config.Password, config.DBName, config.Port, config.SSLMode)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		// IMPORTANT: Disable PrepareStmt for PgBouncer transaction pooling mode
		// Without this, you'll get "prepared statement S_1 does not exist" errors
		PrepareStmt: false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// Configure connection pool for PgBouncer
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %v", err)
	}

	// SetMaxOpenConns: Should be higher than PgBouncer's DEFAULT_POOL_SIZE (50)
	// to allow effective connection competition during peak load
	sqlDB.SetMaxOpenConns(100)

	// SetMaxIdleConns: Keep some connections ready for reuse
	sqlDB.SetMaxIdleConns(25)

	// SetConnMaxLifetime: Should be less than PgBouncer's SERVER_LIFETIME (1800s)
	// to ensure connections are refreshed before PgBouncer closes them
	sqlDB.SetConnMaxLifetime(time.Minute * 15)

	// SetConnMaxIdleTime: Close idle connections after 5 minutes
	sqlDB.SetConnMaxIdleTime(time.Minute * 5)

	return db, nil
}

func Migrate(db *gorm.DB) error {
	err := db.AutoMigrate(
		&models.User{},
		&models.Task{},
		&models.File{},
		&models.Job{},
		&models.Category{},
		&models.Video{},
		// Phase 6: Advanced Domain Whitelist & Ad Management
		&models.WhitelistProfile{},
		&models.ProfileDomain{},
		&models.PrerollAd{},
		&models.AdImpression{},
		// Admin Settings
		&models.SystemSetting{},
		&models.SettingAuditLog{},
		// Subtitles (separate table)
		&models.Subtitle{},
		// Reel Generator
		&models.Reel{},
		&models.ReelTemplate{},
	)
	if err != nil {
		return err
	}

	// Seed default reel templates
	return SeedReelTemplates(db)
}

// SeedReelTemplates เพิ่ม templates เริ่มต้น (ถ้ายังไม่มี)
func SeedReelTemplates(db *gorm.DB) error {
	// ตรวจสอบว่ามี templates อยู่แล้วหรือไม่
	var count int64
	db.Model(&models.ReelTemplate{}).Count(&count)
	if count > 0 {
		return nil // มี templates อยู่แล้ว ไม่ต้อง seed
	}

	defaultTemplates := []models.ReelTemplate{
		{
			Name:        "Clean Title",
			Description: "แสดงหัวข้อที่ด้านบนพร้อม gradient ด้านล่าง",
			DefaultLayers: models.ReelLayers{
				{
					Type:       models.ReelLayerTypeText,
					Content:    "หัวข้อ",
					FontFamily: "Google Sans",
					FontSize:   48,
					FontColor:  "#ffffff",
					FontWeight: "bold",
					X:          50,
					Y:          15,
					Opacity:    1,
					ZIndex:     10,
				},
				{
					Type:    models.ReelLayerTypeBackground,
					Style:   "gradient-dark",
					X:       0,
					Y:       0,
					Width:   100,
					Height:  100,
					Opacity: 0.4,
					ZIndex:  1,
				},
			},
			BackgroundStyle: "gradient-dark",
			FontFamily:      "Google Sans",
			PrimaryColor:    "#ffffff",
			IsActive:        true,
			SortOrder:       1,
		},
		{
			Name:        "Title & Caption",
			Description: "หัวข้อด้านบนและคำอธิบายด้านล่าง",
			DefaultLayers: models.ReelLayers{
				{
					Type:       models.ReelLayerTypeText,
					Content:    "หัวข้อ",
					FontFamily: "Google Sans",
					FontSize:   48,
					FontColor:  "#ffffff",
					FontWeight: "bold",
					X:          50,
					Y:          12,
					Opacity:    1,
					ZIndex:     10,
				},
				{
					Type:       models.ReelLayerTypeText,
					Content:    "คำอธิบายเพิ่มเติม",
					FontFamily: "Google Sans",
					FontSize:   24,
					FontColor:  "#ffffff",
					FontWeight: "normal",
					X:          50,
					Y:          88,
					Opacity:    0.9,
					ZIndex:     10,
				},
				{
					Type:    models.ReelLayerTypeBackground,
					Style:   "gradient-dark",
					X:       0,
					Y:       0,
					Width:   100,
					Height:  100,
					Opacity: 0.5,
					ZIndex:  1,
				},
			},
			BackgroundStyle: "gradient-dark",
			FontFamily:      "Google Sans",
			PrimaryColor:    "#ffffff",
			IsActive:        true,
			SortOrder:       2,
		},
		{
			Name:        "Center Text",
			Description: "ข้อความตรงกลางหน้าจอ",
			DefaultLayers: models.ReelLayers{
				{
					Type:       models.ReelLayerTypeText,
					Content:    "ข้อความ",
					FontFamily: "Google Sans",
					FontSize:   56,
					FontColor:  "#ffffff",
					FontWeight: "bold",
					X:          50,
					Y:          50,
					Opacity:    1,
					ZIndex:     10,
				},
				{
					Type:    models.ReelLayerTypeBackground,
					Style:   "solid-dark",
					X:       0,
					Y:       0,
					Width:   100,
					Height:  100,
					Opacity: 0.3,
					ZIndex:  1,
				},
			},
			BackgroundStyle: "solid-dark",
			FontFamily:      "Google Sans",
			PrimaryColor:    "#ffffff",
			IsActive:        true,
			SortOrder:       3,
		},
		{
			Name:        "Minimal",
			Description: "แสดงวิดีโอโดยไม่มี overlay",
			DefaultLayers: models.ReelLayers{},
			BackgroundStyle: "",
			FontFamily:      "Google Sans",
			PrimaryColor:    "#ffffff",
			IsActive:        true,
			SortOrder:       4,
		},
	}

	return db.Create(&defaultTemplates).Error
}