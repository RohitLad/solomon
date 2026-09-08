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
	return c.JSON(fiber.Map{
		"totals": fiber.Map{"views": tv, "likes": tl, "comments": tc, "shares": ts, "posts": len(rows)},
		"rows": rows,
	})
}

func (h *AnalyticsHandler) latest() ([]TargetAnalytics, error) {
	var targets []models.PostTarget
	if err := h.DB.Where("status = ?", models.TargetPublished).Find(&targets).Error; err != nil {
		return nil, err
	}
	rows := []TargetAnalytics{}
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
		}
		if len(snaps) > 0 {
			r.Views, r.Likes, r.Comments, r.Shares, r.IsDemo =
				snaps[0].Views, snaps[0].Likes, snaps[0].Comments, snaps[0].Shares, snaps[0].IsDemo
		}
		rows = append(rows, r)
	}
	return rows, nil
}

// Refresh fetches stats for every published target and stores snapshots.
func (h *AnalyticsHandler) Refresh(c *fiber.Ctx) error {
	n, errs := RefreshAll(h.DB)
	return c.JSON(fiber.Map{"refreshed": n, "errors": errs})
}

func RefreshAll(db *gorm.DB) (int, []string) {
	var targets []models.PostTarget
	db.Where("status = ?", models.TargetPublished).Find(&targets)
	done := 0
	errs := []string{}
	for _, t := range targets {
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
