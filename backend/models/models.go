package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Network is one of the 7 supported social networks.
type Network string

const (
	NetworkTwitter   Network = "twitter"   // X API v2
	NetworkFacebook  Network = "facebook"  // Graph API v21
	NetworkInstagram Network = "instagram" // Instagram Graph API (via FB Page)
	NetworkYouTube   Network = "youtube"   // YouTube Data API v3
	NetworkTikTok    Network = "tiktok"    // TikTok Content Posting API v2
	NetworkLinkedIn  Network = "linkedin"  // LinkedIn Posts API (versioned)
	NetworkPinterest Network = "pinterest" // Pinterest API v5
)

func AllNetworks() []Network {
	return []Network{
		NetworkTwitter, NetworkFacebook, NetworkInstagram,
		NetworkYouTube, NetworkTikTok, NetworkLinkedIn, NetworkPinterest,
	}
}

// SocialAccount = one connected profile/page/channel.
// A user can add MANY accounts per network.
type SocialAccount struct {
	ID           string  `json:"id" gorm:"primaryKey"`
	Network      Network `json:"network" gorm:"index"`
	Name         string  `json:"name"` // e.g. "@acme" or Page name
	ExternalID   string  `json:"external_id"`
	AccessToken  string  `json:"-"` // never expose to frontend
	RefreshToken string  `json:"-"`
	ExpiresAt    *time.Time `json:"expires_at"`
	// Extra holds network-specific bits as JSON string:
	// facebook/instagram: page_id, ig_user_id | youtube: channel_id |
	// linkedin: person_urn/org_urn | pinterest: board_id_default | tiktok: open_id
	Extra    string `json:"extra"`
	IsActive bool   `json:"is_active" gorm:"default:true"`
	CreatedAt time.Time `json:"created_at"`
}

func (a *SocialAccount) BeforeCreate(_ *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	return nil
}

// MediaType distinguishes image / video uploads.
type MediaType string

const (
	MediaImage MediaType = "image"
	MediaVideo MediaType = "video"
)

// MediaAsset is a file attached to a Post (up to N per network).
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

// PostStatus lifecycle.
type PostStatus string

const (
	StatusDraft     PostStatus = "draft"
	StatusScheduled PostStatus = "scheduled"
	StatusPublished PostStatus = "published"
	StatusPartial   PostStatus = "partial" // some targets failed
	StatusFailed    PostStatus = "failed"
)

// Post is the MASTER post. Per-account tweaks live on PostTarget.CustomText.
type Post struct {
	ID          string     `json:"id" gorm:"primaryKey"`
	Title       string     `json:"title"` // required by YouTube/Pinterest, optional elsewhere
	Content     string     `json:"content"`
	Link        string     `json:"link"`
	ScheduledAt *time.Time `json:"scheduled_at"`
	Status      PostStatus `json:"status" gorm:"default:draft"`
	// PostType is derived: text | image | carousel | video | short
	PostType  string       `json:"post_type"`
	Targets   []PostTarget `json:"targets" gorm:"foreignKey:PostID"`
	Media     []MediaAsset `json:"media" gorm:"foreignKey:PostID"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

func (p *Post) BeforeCreate(_ *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	return nil
}

// TargetStatus per account.
type TargetStatus string

const (
	TargetPending   TargetStatus = "pending"
	TargetPublished TargetStatus = "published"
	TargetFailed    TargetStatus = "failed"
	TargetSkipped   TargetStatus = "skipped"
)

// PostTarget = "publish this Post to this account, with optional custom text".
type PostTarget struct {
	ID            string       `json:"id" gorm:"primaryKey"`
	PostID        string       `json:"post_id" gorm:"index"`
	AccountID     string       `json:"account_id" gorm:"index"`
	Account       SocialAccount `json:"account" gorm:"foreignKey:AccountID"`
	CustomText    string       `json:"custom_text"` // empty = use Post.Content
	FirstComment  string       `json:"first_comment"`
	Status        TargetStatus `json:"status" gorm:"default:pending"`
	NetworkPostID string       `json:"network_post_id"` // id(s) returned, comma-joined for threads
	Error         string       `json:"error"`
	PublishedAt   *time.Time   `json:"published_at"`
}

func (t *PostTarget) BeforeCreate(_ *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	return nil
}

// EffectiveText returns the per-account override or the master text.
func (t PostTarget) EffectiveText(master string) string {
	if t.CustomText != "" {
		return t.CustomText
	}
	return master
}

// AnalyticsSnapshot = one metrics pull for a published target.
// Works in demo mode (deterministic pseudo-stats) and, where the network
// exposes a simple endpoint + token, from the real API (see networks/stats.go).
type AnalyticsSnapshot struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	TargetID    string    `json:"target_id" gorm:"index"`
	NetworkPostID string  `json:"network_post_id"`
	Views       int64     `json:"views"`
	Likes       int64     `json:"likes"`
	Comments    int64     `json:"comments"`
	Shares      int64     `json:"shares"`
	IsDemo      bool      `json:"is_demo"`
	FetchedAt   time.Time `json:"fetched_at"`
}

func (s *AnalyticsSnapshot) BeforeCreate(_ *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	return nil
}

// EvergreenRule recycles a pool of posts forever:
// every IntervalHours the scheduler republishes the next pool post
// (round-robin) to AccountIDs as a brand-new Post.
type EvergreenRule struct {
	ID         string     `json:"id" gorm:"primaryKey"`
	Name       string     `json:"name"`
	PoolPostIDs string    `json:"pool_post_ids"` // JSON ["postid",...]
	AccountIDs  string    `json:"account_ids"`   // JSON ["acctid",...]
	IntervalHours int     `json:"interval_hours"`
	Cursor     int        `json:"cursor"`
	Active     bool       `json:"active" gorm:"default:true"`
	LastRunAt  *time.Time `json:"last_run_at"`
	NextRunAt  *time.Time `json:"next_run_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

func (r *EvergreenRule) BeforeCreate(_ *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	return nil
}
