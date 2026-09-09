package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SocialAccount = one connected profile/page/channel.
// A user can add MANY accounts per network.
type SocialAccount struct {
	ID           string   `json:"id" gorm:"primaryKey"`
	Network      Network  `json:"network" gorm:"index"`
	Name         string   `json:"name"` // e.g. "@acme" or Page name
	ExternalID   string   `json:"external_id"`
	AccessToken  string   `json:"-"` // never expose to frontend
	RefreshToken string   `json:"-"`
	ExpiresAt    *time.Time `json:"expires_at"`
	// Extra holds network-specific bits as JSON string:
	// facebook/instagram: page_id, ig_user_id | youtube: channel_id |
	// linkedin: person_urn/org_urn | pinterest: board_id_default | tiktok: open_id
	Extra     string    `json:"extra"`
	IsActive  bool      `json:"is_active" gorm:"default:true"`
	CreatedAt time.Time `json:"created_at"`
}

func (a *SocialAccount) BeforeCreate(_ *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	return nil
}
