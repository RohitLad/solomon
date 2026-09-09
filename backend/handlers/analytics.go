package handlers

// Analytics: list + on-demand refresh.
// GET  /api/analytics            -> totals + per-target latest snapshots
// POST /api/analytics/refresh    -> re-fetch all published targets (demo = instant)

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"solomon/backend/models"
	"solomon/backend/networks"
)

type AnalyticsHandler struct{ DB *gorm.DB }

type TargetAnalytics struct {
	TargetID      string `json:"target_id"`
	PostID        string `json:"post_id"`
	Network       models.Network `json:"network"`
	AccountName   string `json:"account_name"`
	NetworkPostID string `json:"network_post_id"`
	Views   int64 `json:"views"`
	Likes   int64 `json:"likes"`
	Comments int64 `json:"comments"`
	Shares  int64 `json:"shares"`
	IsDemo  bool  `json:"is_demo"`
	// Deleted marks rows of in-app-deleted (published) posts. History is kept
	// by design; the UI shows a "deleted" badge. Deleted posts are excluded
	// from the best-time engagement boost and from stat refreshes.
	Deleted bool  `json:"deleted"`
	// External marks native posts made outside the app ("Outside Solomon").
	// Read-only rows: analytics + dimmed calendar dots, with text/snippet.
	External bool  `json:"external"`
	Text     string `json:"text,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

func (h *AnalyticsHandler) List(c *fiber.Ctx) error {
	rows, err := h.latest()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	var tv, tl, tc, ts int64
	for _, r := range rows {
		tv += r.Views
		tl += r.Likes
		tc += r.Comments
		ts += r.Shares
	}
	// Outside-Solomon rows carry their own mini-totals (app totals stay clean).
	var ov, ol, oc, os int64
	oposts := 0
	for _, r := range rows {
		if !r.External {
			continue
		}
		ov += r.Views
		ol += r.Likes
		oc += r.Comments
		os += r.Shares
		oposts++
	}
	return c.JSON(fiber.Map{
		"totals": fiber.Map{"views": tv, "likes": tl, "comments": tc, "shares": ts, "posts": len(rows)},
		"outside": fiber.Map{"views": ov, "likes": ol, "comments": oc, "shares": os, "posts": oposts},
		"rows": rows,
	})
}

func (h *AnalyticsHandler) latest() ([]TargetAnalytics, error) {
	var targets []models.PostTarget
	if err := h.DB.Where("status = ?", models.TargetPublished).Find(&targets).Error; err != nil {
		return nil, err
	}
	rows := []TargetAnalytics{}
	deleted := deletedPostIDs(h.DB)
	for _, t := range targets {
		var acct models.SocialAccount
		if err := h.DB.First(&acct, "id = ?", t.AccountID).Error; err != nil {
			continue
		}
		var snaps []models.AnalyticsSnapshot
		h.DB.Where("target_id = ?", t.ID).Order("fetched_at desc").Limit(1).Find(&snaps)
		r := TargetAnalytics{
			TargetID: t.ID, PostID: t.PostID,
			Network: acct.Network, AccountName: acct.Name, NetworkPostID: t.NetworkPostID,
			Deleted: deleted[t.PostID],
		}
		if len(snaps) > 0 {
			r.Views, r.Likes, r.Comments, r.Shares, r.IsDemo =
				snaps[0].Views, snaps[0].Likes, snaps[0].Comments, snaps[0].Shares, snaps[0].IsDemo
		}
		rows = append(rows, r)
	}
	// Outside Solomon: native posts with their latest snapshots.
	var ext []models.ExternalPost
	if err := h.DB.Find(&ext).Error; err == nil && len(ext) > 0 {
		var accts []models.SocialAccount
		h.DB.Find(&accts)
		byAcct := make(map[string]models.SocialAccount, len(accts))
		for _, a := range accts {
			byAcct[a.ID] = a
		}
		for _, e := range ext {
			var snaps []models.AnalyticsSnapshot
			h.DB.Where("external_post_id = ?", e.ID).Order("fetched_at desc").Limit(1).Find(&snaps)
			r := TargetAnalytics{
				TargetID: "ext:" + e.ID, Network: e.Network,
				AccountName: byAcct[e.AccountID].Name, NetworkPostID: e.NetworkPostID,
				External: true, Text: e.Text, PublishedAt: e.PublishedAt,
			}
			if len(snaps) > 0 {
				r.Views, r.Likes, r.Comments, r.Shares, r.IsDemo =
					snaps[0].Views, snaps[0].Likes, snaps[0].Comments, snaps[0].Shares, snaps[0].IsDemo
			}
			rows = append(rows, r)
		}
	}
	return rows, nil
}

// Refresh fetches stats for every published target and stores snapshots.
// deletedPostIDs returns the IDs of soft-deleted posts (history kept for
// analytics display, but excluded from boost/refresh/evergreen/scheduler).
func deletedPostIDs(db *gorm.DB) map[string]bool {
	var ids []string
	db.Model(&models.Post{}).Where("deleted_at IS NOT NULL").Pluck("id", &ids)
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

func (h *AnalyticsHandler) Refresh(c *fiber.Ctx) error {
	n, errs := RefreshAll(h.DB)
	d, notes := DiscoverExternal(h.DB)
	return c.JSON(fiber.Map{"refreshed": n, "discovered": d, "errors": append(errs, notes...)})
}

func RefreshAll(db *gorm.DB) (int, []string) {
	var targets []models.PostTarget
	db.Where("status = ?", models.TargetPublished).Find(&targets)
	done := 0
	errs := []string{}
	deleted := deletedPostIDs(db)
	for _, t := range targets {
		if deleted[t.PostID] {
			continue // frozen history: deleted posts keep old snapshots
		}
		var acct models.SocialAccount
		if err := db.First(&acct, "id = ?", t.AccountID).Error; err != nil {
			continue
		}
		firstID := t.NetworkPostID
		if i := indexComma(firstID); i >= 0 {
			firstID = firstID[:i] // threads: stats for first tweet
		}
		st, err := networks.FetchStats(string(acct.Network), acct.AccessToken, firstID)
		if err != nil {
			errs = append(errs, string(acct.Network)+" "+acct.Name+": "+err.Error())
			continue
		}
		db.Create(&models.AnalyticsSnapshot{
			TargetID: t.ID, NetworkPostID: firstID,
			Views: st.Views, Likes: st.Likes, Comments: st.Comments, Shares: st.Shares,
			IsDemo: st.IsDemo, FetchedAt: time.Now(),
		})
		done++
	}
	return done, errs
}

func indexComma(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			return i
		}
	}
	return -1
}
