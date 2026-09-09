package handlers

// Outside-Solomon discovery: periodically list native (non-app) posts per
// account so analytics covers the whole account — the Buffer/Hootsuite
// standard. Runs on manual analytics refresh and hourly via the scheduler.
//
// Buffer-parity guardrails: 30-day backfill on first run (resume from newest
// known after that), ≤100 posts/account/run, stat refresh frozen 20 days
// after publishing and at most daily per post (quota-friendly). App-published
// IDs are deduped (threads: first ID). Unsupported networks (e.g. TikTok
// without an audited app) yield a per-account note, never a failure.

import (
	"strings"
	"time"

	"gorm.io/gorm"

	"solomon/backend/models"
	"solomon/backend/networks"
)

const (
	discoverBackfillDays = 30
	discoverDailyCap     = 100
	statsFreezeDays      = 20
	statsRefreshAfter    = 24 * time.Hour
)

// DiscoverExternal pulls native posts for every active account and refreshes
// stats of recent ones. Returns (# newly stored posts, per-account notes).
func DiscoverExternal(db *gorm.DB) (int, []string) {
	var accounts []models.SocialAccount
	db.Where("is_active = ?", true).Find(&accounts)
	found := 0
	var notes []string
	for _, a := range accounts {
		n, note := discoverAccount(db, a)
		found += n
		if note != "" {
			notes = append(notes, a.Name+" ("+string(a.Network)+"): "+note)
		}
	}
	if notes == nil {
		notes = []string{}
	}
	return found, notes
}

func discoverAccount(db *gorm.DB, a models.SocialAccount) (int, string) {
	cutoff := time.Now().AddDate(0, 0, -discoverBackfillDays)
	since := cutoff
	var last models.ExternalPost
	if err := db.Where("account_id = ? AND published_at IS NOT NULL", a.ID).Order("published_at DESC").First(&last).Error; err == nil {
		if last.PublishedAt != nil && last.PublishedAt.After(since) {
			since = *last.PublishedAt
		}
	}
	cands, err := networks.ListRecentPosts(a.Network, a.AccessToken, a.Extra, a.ExternalID, since, discoverDailyCap)
	if err != nil {
		return 0, err.Error()
	}
	// Own app-published IDs must not reappear as "outside".
	own := map[string]bool{}
	var targets []models.PostTarget
	db.Where("account_id = ? AND status = ? AND network_post_id != ''", a.ID, models.TargetPublished).Find(&targets)
	for _, t := range targets {
		id := t.NetworkPostID
		if i := strings.IndexByte(id, ','); i >= 0 {
			id = id[:i]
		}
		own[id] = true
	}
	added := 0
	for _, c := range cands {
		if c.NetworkPostID == "" || own[c.NetworkPostID] {
			continue
		}
		var ep models.ExternalPost
		if err := db.Where("account_id = ? AND network_post_id = ?", a.ID, c.NetworkPostID).First(&ep).Error; err == nil {
			continue // already known
		}
		pub := c.PublishedAt
		ep = models.ExternalPost{
			AccountID: a.ID, Network: a.Network, NetworkPostID: c.NetworkPostID,
			Text: c.Text, Permalink: c.Permalink, PublishedAt: &pub,
		}
		if err := db.Create(&ep).Error; err != nil {
			continue
		}
		added++
	}
	// Stats for posts still fresh (≤20d old), at most daily per post.
	now := time.Now()
	freeze := now.AddDate(0, 0, -statsFreezeDays)
	var fresh []models.ExternalPost
	db.Where("account_id = ? AND published_at IS NOT NULL AND published_at > ? AND (last_stats_at IS NULL OR last_stats_at <= ?)",
		a.ID, freeze, now.Add(-statsRefreshAfter)).Find(&fresh)
	for _, ep := range fresh {
		st, err := networks.FetchStats(string(a.Network), a.AccessToken, ep.NetworkPostID)
		if err != nil {
			continue
		}
		db.Create(&models.AnalyticsSnapshot{
			ExternalPostID: &ep.ID, NetworkPostID: ep.NetworkPostID,
			Views: st.Views, Likes: st.Likes, Comments: st.Comments, Shares: st.Shares,
			IsDemo: st.IsDemo, FetchedAt: now,
		})
		db.Model(&ep).Update("last_stats_at", &now)
	}
	return added, ""
}
