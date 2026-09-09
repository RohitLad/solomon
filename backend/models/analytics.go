package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AnalyticsSnapshot = one metrics pull for a published target.
// Works in demo mode (deterministic pseudo-stats) and, where the network
// exposes a simple endpoint + token, from the real API (see networks/stats.go).
type AnalyticsSnapshot struct {
	ID         string `json:"id" gorm:"primaryKey"`
	TargetID   string `json:"target_id" gorm:"index"`
	// ExternalPostID links snapshots of native (non-app) posts discovered per
	// account ("Outside Solomon"). Exactly one of TargetID / ExternalPostID set.
	ExternalPostID *string `json:"external_post_id" gorm:"index"`
	NetworkPostID  string  `json:"network_post_id"`
	Views          int64   `json:"views"`
	Likes          int64   `json:"likes"`
	Comments       int64   `json:"comments"`
	Shares         int64   `json:"shares"`
	IsDemo         bool    `json:"is_demo"`
	FetchedAt      time.Time `json:"fetched_at"`
}

func (s *AnalyticsSnapshot) BeforeCreate(_ *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	return nil
}
