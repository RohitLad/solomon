package handlers

// Smart scheduling: suggest best datetimes for a set of networks.
// Score = default best-time slot + analytics boost (your own top
// weekday-hours by engagement get +2). GET /api/schedule/suggest.

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"solomon/backend/models"
	"solomon/backend/networks"
)

type ScheduleHandler struct{ DB *gorm.DB }

type Suggestion struct {
	At       time.Time       `json:"at"`
	Score    int             `json:"score"`
	Networks []models.Network `json:"networks"`
	Reason   string          `json:"reason"`
}

func (h *ScheduleHandler) Suggest(c *fiber.Ctx) error {
	netsParam := c.Query("networks", "twitter,facebook,instagram,youtube,tiktok,linkedin,pinterest")
	count := c.QueryInt("count", 3)
	if count < 1 || count > 20 {
		count = 3
	}
	var nets []models.Network
	for _, n := range strings.Split(netsParam, ",") {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		nets = append(nets, models.Network(n))
	}
	if len(nets) == 0 {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "no networks"})
	}

	// analytics boost: avg engagement per weekday-hour from YOUR history
	boost := h.engagementBoost(nets)

	// base scores from defaults
	type key struct{ wd, hr int }
	scores := map[key]int{}
	reasons := map[key][]string{}
	for _, n := range nets {
		for _, s := range networks.DefaultSlots(n) {
			k := key{s.Weekday, s.Hour}
			scores[k] += s.Score
			reasons[k] = append(reasons[k], string(n))
		}
	}
	for k, b := range boost {
		scores[k] += b
	}

	// next 14 days, unlocked hours only (no past, no :00-minute dupes)
	now := time.Now().Truncate(time.Hour).Add(time.Hour)
	type cand struct {
		at time.Time
		k  key
	}
	var cands []cand
	for d := 0; d < 14; d++ {
		day := now.AddDate(0, 0, d)
		for hr := 0; hr < 24; hr++ {
			at := time.Date(day.Year(), day.Month(), day.Day(), hr, 0, 0, 0, day.Location())
			if at.Before(now) {
				continue
			}
			k := key{int(at.Weekday()), hr}
			if scores[k] == 0 {
				continue
			}
			cands = append(cands, cand{at, k})
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		if scores[cands[i].k] != scores[cands[j].k] {
			return scores[cands[i].k] > scores[cands[j].k]
		}
		return cands[i].at.Before(cands[j].at)
	})

	out := []Suggestion{}
	usedDay := map[string]bool{}
	for _, cd := range cands {
		if len(out) >= count {
			break
		}
		dayKey := cd.at.Format("2006-01-02")
		if usedDay[dayKey] && len(nets) > 1 {
			continue // spread across days when multi-network
		}
		usedDay[dayKey] = true
		reason := "best-time default"
		if boost[cd.k] > 0 {
			reason = "your analytics: high engagement at this hour"
		}
		out = append(out, Suggestion{At: cd.at, Score: scores[cd.k], Networks: nets, Reason: reason})
	}
	return c.JSON(fiber.Map{"suggestions": out})
}

// engagementBoost returns +2 for the top-5 weekday-hours by avg engagement.
func (h *ScheduleHandler) engagementBoost(nets []models.Network) map[struct{ wd, hr int }]int {
	return engagementBoost(h.DB, nets)
}

// BestSlot returns the single best upcoming datetime for the given target
// accounts' networks. Shared by auto_schedule on post create.
func BestSlot(db *gorm.DB, targets []targetIn) *time.Time {
	return bestSlotFor(db, targets)
}

func bestSlotFor(db *gorm.DB, targets []targetIn) *time.Time {
	netSet := map[models.Network]bool{}
	for _, t := range targets {
		var acct models.SocialAccount
		if err := db.First(&acct, "id = ?", t.AccountID).Error; err == nil {
			netSet[acct.Network] = true
		}
	}
	var nets []models.Network
	for n := range netSet {
		nets = append(nets, n)
	}
	if len(nets) == 0 {
		return nil
	}
	boost := engagementBoost(db, nets)
	type key struct{ wd, hr int }
	scores := map[key]int{}
	for _, n := range nets {
		for _, s := range networks.DefaultSlots(n) {
			scores[key{s.Weekday, s.Hour}] += s.Score
		}
	}
	for k, b := range boost {
		scores[k] += b
	}
	now := time.Now().Truncate(time.Hour).Add(time.Hour)
	bestScore := -1
	var best time.Time
	for d := 0; d < 14; d++ {
		day := now.AddDate(0, 0, d)
		for hr := 0; hr < 24; hr++ {
			at := time.Date(day.Year(), day.Month(), day.Day(), hr, 0, 0, 0, day.Location())
			if at.Before(now) {
				continue
			}
			if s := scores[key{int(at.Weekday()), hr}]; s > bestScore {
				bestScore, best = s, at
			}
		}
	}
	if bestScore <= 0 {
		t := now
		return &t
	}
	return &best
}

func engagementBoost(db *gorm.DB, nets []models.Network) map[struct{ wd, hr int }]int {
	type row struct {
		PublishedAt *time.Time
		Engagement  int64
	}
	// sum latest snapshot per target
	var targets []models.PostTarget
	db.Where("status = ? AND published_at IS NOT NULL", models.TargetPublished).Find(&targets)
	if len(targets) == 0 {
		return nil
	}
	netSet := map[models.Network]bool{}
	for _, n := range nets {
		netSet[n] = true
	}
	agg := map[struct{ wd, hr int }][]int64{}
	for _, t := range targets {
		var acct models.SocialAccount
		if err := db.First(&acct, "id = ?", t.AccountID).Error; err != nil || !netSet[acct.Network] {
			continue
		}
		var snaps []models.AnalyticsSnapshot
		db.Where("target_id = ?", t.ID).Order("fetched_at desc").Limit(1).Find(&snaps)
		if len(snaps) == 0 || t.PublishedAt == nil {
			continue
		}
		s := snaps[0]
		eng := s.Likes + s.Comments + s.Shares
		k := struct{ wd, hr int }{int(t.PublishedAt.Weekday()), t.PublishedAt.Hour()}
		agg[k] = append(agg[k], eng)
	}
	type kv struct {
		k   struct{ wd, hr int }
		avg float64
	}
	var ranked []kv
	for k, vals := range agg {
		var sum int64
		for _, v := range vals {
			sum += v
		}
		ranked = append(ranked, kv{k, float64(sum) / float64(len(vals))})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].avg > ranked[j].avg })
	out := map[struct{ wd, hr int }]int{}
	for i := 0; i < len(ranked) && i < 5; i++ {
		out[ranked[i].k] = 2
	}
	return out
}
