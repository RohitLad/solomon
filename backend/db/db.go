package db

import (
	"solomon/backend/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func Connect(path string) (*gorm.DB, error) {
	database, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := database.AutoMigrate(
		&models.SocialAccount{},
		&models.Post{},
		&models.PostTarget{},
		&models.MediaAsset{},
		&models.AnalyticsSnapshot{},
		&models.EvergreenRule{},
	); err != nil {
		return nil, err
	}
	return database, nil
}
