package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

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
	// DeletedAt marks an in-app delete of an already-published post.
	// The row (and its targets/snapshots) is KEPT for analytics history;
	// list endpoints hide deleted posts unless ?include_deleted=1.
	// Scheduled/draft deletes are hard deletes instead (no history value).
	DeletedAt *time.Time `json:"deleted_at" gorm:"index"`
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
	ID            string        `json:"id" gorm:"primaryKey"`
	PostID        string        `json:"post_id" gorm:"index"`
	AccountID     string        `json:"account_id" gorm:"index"`
	Account       SocialAccount `json:"account" gorm:"foreignKey:AccountID"`
	CustomText    string        `json:"custom_text"` // empty = use Post.Content
	FirstComment  string        `json:"first_comment"`
	Status        TargetStatus  `json:"status" gorm:"default:pending"`
	NetworkPostID string        `json:"network_post_id"` // id(s) returned, comma-joined for threads
	Error         string        `json:"error"`
	PublishedAt   *time.Time    `json:"published_at"`
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
