package handlers

// Bulk CSV import + evergreen (recurring repost) rules.
// POST /api/posts/bulk  (multipart "file", ?auto_schedule=1)
//   CSV columns: title,content,link,scheduled_at,accounts
//   accounts = ";"-list of account names OR "network" names (e.g. "twitter;@acme")
// GET/POST /api/evergreen, DELETE /api/evergreen/:id
//   {name, pool_post_ids:[...], account_ids:[...], interval_hours}

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"solomon/backend/models"
)

type BulkHandler struct{ DB *gorm.DB }

func (h *BulkHandler) Import(c *fiber.Ctx) error {
	fh, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "upload multipart file as 'file'"})
	}
	f, err := fh.Open()
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "bad csv: " + err.Error()})
	}
	if len(rows) < 2 {
		return c.Status(400).JSON(fiber.Map{"error": "csv needs header + at least 1 row"})
	}
	header := lowerAll(rows[0])
	idx := func(names ...string) int {
		for _, n := range names {
			for i, h := range header {
				if h == n {
					return i
				}
			}
		}
		return -1
	}
	ciTitle, ciContent, ciLink, ciAt, ciAccts :=
		idx("title"), idx("content", "text"), idx("link", "url"), idx("scheduled_at", "scheduledat", "at"), idx("accounts", "account")

	auto := c.Query("auto_schedule") == "1"
	created, skipped := 0, 0
	errs := []string{}
	nextSlot := time.Now().Add(time.Hour).Truncate(time.Hour)
	for r, row := range rows[1:] {
		get := func(i int) string {
			if i < 0 || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		content := get(ciContent)
		if content == "" {
			skipped++
			continue
		}
		var at *time.Time
		if s := get(ciAt); s != "" {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				at = &t
			} else if t, err := time.Parse("2006-01-02 15:04", s); err == nil {
				at = &t
			}
		}
		if at == nil && auto {
			t := nextSlot
			at = &t
			nextSlot = nextSlot.Add(3 * time.Hour) // space bulk posts out
		}
		acctIDs := h.resolveAccounts(get(ciAccts))
		if len(acctIDs) == 0 {
			errs = append(errs, fmt.Sprintf("row %d: no matching accounts", r+2))
			skipped++
			continue
		}
		status := models.StatusDraft
		if at != nil {
			status = models.StatusScheduled
		}
		p := models.Post{Title: get(ciTitle), Content: content, Link: get(ciLink), ScheduledAt: at, Status: status}
		if err := h.DB.Create(&p).Error; err != nil {
			errs = append(errs, fmt.Sprintf("row %d: %v", r+2, err))
			skipped++
			continue
		}
		for _, id := range acctIDs {
			h.DB.Create(&models.PostTarget{PostID: p.ID, AccountID: id})
		}
		created++
	}
	return c.JSON(fiber.Map{"created": created, "skipped": skipped, "errors": errs})
}

func lowerAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = strings.ToLower(strings.TrimSpace(s))
	}
	return out
}

// resolveAccounts matches ";"-separated tokens against account name or network.
func (h *BulkHandler) resolveAccounts(spec string) []string {
	if spec == "" {
		return nil
	}
	var all []models.SocialAccount
	h.DB.Find(&all)
	byName := map[string]string{}
	byNet := map[string][]string{}
	for _, a := range all {
		byName[strings.ToLower(a.Name)] = a.ID
		byNet[string(a.Network)] = append(byNet[string(a.Network)], a.ID)
	}
	var out []string
	seen := map[string]bool{}
	for _, tok := range strings.Split(spec, ";") {
		tok = strings.ToLower(strings.TrimSpace(tok))
		if tok == "" {
			continue
		}
		if id, ok := byName[tok]; ok && !seen[id] {
			out, seen[id] = append(out, id), true
			continue
		}
		for _, id := range byNet[tok] {
			if !seen[id] {
				out, seen[id] = append(out, id), true
			}
		}
	}
	return out
}

// ---------- evergreen ----------

type EvergreenHandler struct{ DB *gorm.DB }

func (h *EvergreenHandler) List(c *fiber.Ctx) error {
	rules := []models.EvergreenRule{}
	h.DB.Order("created_at desc").Find(&rules)
	return c.JSON(rules)
}

func (h *EvergreenHandler) Create(c *fiber.Ctx) error {
	var in struct {
		Name          string   `json:"name"`
		PoolPostIDs   []string `json:"pool_post_ids"`
		AccountIDs    []string `json:"account_ids"`
		IntervalHours int      `json:"interval_hours"`
	}
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if in.Name == "" || len(in.PoolPostIDs) == 0 || len(in.AccountIDs) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "name, pool_post_ids and account_ids required"})
	}
	if in.IntervalHours <= 0 {
		in.IntervalHours = 24
	}
	pool, _ := json.Marshal(in.PoolPostIDs)
	accts, _ := json.Marshal(in.AccountIDs)
	next := time.Now().Add(time.Duration(in.IntervalHours) * time.Hour)
	r := models.EvergreenRule{
		Name: in.Name, PoolPostIDs: string(pool), AccountIDs: string(accts),
		IntervalHours: in.IntervalHours, Active: true, NextRunAt: &next,
	}
	if err := h.DB.Create(&r).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(201).JSON(r)
}

func (h *EvergreenHandler) Delete(c *fiber.Ctx) error {
	h.DB.Delete(&models.EvergreenRule{}, "id = ?", c.Params("id"))
	return c.JSON(fiber.Map{"ok": true})
}

// RunDueRules republishes the next pool post for every due rule.
// Called by the scheduler every minute.
func RunDueRules(db *gorm.DB) int {
	var rules []models.EvergreenRule
	db.Where("active = ? AND (next_run_at IS NULL OR next_run_at <= ?)", true, time.Now()).Find(&rules)
	ran := 0
	for _, r := range rules {
		var pool []string
		var accts []string
		_ = json.Unmarshal([]byte(r.PoolPostIDs), &pool)
		_ = json.Unmarshal([]byte(r.AccountIDs), &accts)
		if len(pool) == 0 || len(accts) == 0 {
			continue
		}
		srcID := pool[r.Cursor%len(pool)]
		var src models.Post
		if err := db.Preload("Media").First(&src, "id = ?", srcID).Error; err != nil {
			continue
		}
		now := time.Now()
		// copy as a brand-new scheduled post (same files, new rows)
		cp := models.Post{Title: src.Title, Content: src.Content, Link: src.Link,
			ScheduledAt: &now, Status: models.StatusScheduled}
		if err := db.Create(&cp).Error; err != nil {
			continue
		}
		for _, m := range src.Media {
			db.Create(&models.MediaAsset{PostID: cp.ID, FilePath: m.FilePath, MediaType: m.MediaType, SortOrder: m.SortOrder})
		}
		for _, aid := range accts {
			db.Create(&models.PostTarget{PostID: cp.ID, AccountID: aid})
		}
		if _, err := PublishDuePost(db, cp.ID); err != nil {
			_ = err
		}
		next := now.Add(time.Duration(r.IntervalHours) * time.Hour)
		db.Model(&r).Updates(map[string]any{"cursor": r.Cursor + 1, "last_run_at": &now, "next_run_at": &next})
		ran++
	}
	return ran
}

var _ = io.EOF
