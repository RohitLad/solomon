package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ExternalPost = a native post made OUTSIDE Solomon on a connected account,
// discovered by periodic per-account listing ("Outside Solomon").
// Read-only in the app: analytics + dimmed calendar dots, no reschedule.
type ExternalPost struct {
	ID            string     `json:"id" gorm:"primaryKey"`
	AccountID     string     `json:"account_id" gorm:"index;uniqueIndex:idx_ext_acct_post"`
	Network       Network    `json:"network"`
	NetworkPostID string     `json:"network_post_id" gorm:"uniqueIndex:idx_ext_acct_post"`
	Text          string     `json:"text"`
	Permalink     string     `json:"permalink"`
	PublishedAt   *time.Time `json:"published_at" gorm:"index"`
	LastStatsAt   *time.Time `json:"last_stats_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

func (e *ExternalPost) BeforeCreate(_ *gorm.DB) error {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	return nil
}
