package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MediaType distinguishes image / video uploads.
type MediaType string

const (
	MediaImage MediaType = "image"
	MediaVideo MediaType = "video"
)

// MediaAsset is a file attached to a Post (up to N per network).
// Rows are created at upload with an empty PostID and attached on post create.
// Evergreen copies share FilePath, so hard post delete removes the file only
// when no other row references it (see PostHandler.Delete).
type MediaAsset struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	PostID    string    `json:"post_id" gorm:"index"`
	FilePath  string    `json:"file_path"`
	MediaType MediaType `json:"media_type"`
	SortOrder int       `json:"sort_order"`
}

func (m *MediaAsset) BeforeCreate(_ *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	return nil
}
