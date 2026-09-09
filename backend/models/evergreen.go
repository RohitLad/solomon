package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EvergreenRule recycles a pool of posts forever:
// every IntervalHours the scheduler republishes the next pool post
// (round-robin) to AccountIDs as a brand-new Post.
// PoolPostIDs referencing deleted/missing posts are skipped at run time
// (and pruned on hard post delete); an empty pool never runs.
type EvergreenRule struct {
	ID            string     `json:"id" gorm:"primaryKey"`
	Name          string     `json:"name"`
	PoolPostIDs   string     `json:"pool_post_ids"` // JSON ["postid",...]
	AccountIDs    string     `json:"account_ids"`   // JSON ["acctid",...]
	IntervalHours int        `json:"interval_hours"`
	Cursor        int        `json:"cursor"`
	Active        bool       `json:"active" gorm:"default:true"`
	LastRunAt     *time.Time `json:"last_run_at"`
	NextRunAt     *time.Time `json:"next_run_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

func (r *EvergreenRule) BeforeCreate(_ *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	return nil
}
