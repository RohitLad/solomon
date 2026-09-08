package handlers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"solomon/backend/models"
	"solomon/backend/networks"
)

type PostHandler struct{ DB *gorm.DB }

// List posts with targets+media (newest first). ?status=scheduled filters.
func (h *PostHandler) List(c *fiber.Ctx) error {
	posts := []models.Post{}
	q := h.DB.Preload("Targets.Account").Preload("Media").Order("created_at desc")
	if s := c.Query("status"); s != "" {
		q = q.Where("status = ?", s)
	}
	if err := q.Find(&posts).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	for i := range posts {
		if posts[i].Targets == nil {
			posts[i].Targets = []models.PostTarget{}
		}
		if posts[i].Media == nil {
			posts[i].Media = []models.MediaAsset{}
		}
		for j := range posts[i].Targets {
			posts[i].Targets[j].Account.AccessToken = ""
			posts[i].Targets[j].Account.RefreshToken = ""
		}
	}
	return c.JSON(posts)
}

type targetIn struct {
	AccountID    string `json:"account_id"`
	CustomText   string `json:"custom_text"`
	FirstComment string `json:"first_comment"`
}

// Create a post. Send EITHER scheduled_at (schedule) or publish_now:true.
func (h *PostHandler) Create(c *fiber.Ctx) error {
	var in struct {
		Title       string     `json:"title"`
		Content     string     `json:"content"`
		Link        string     `json:"link"`
		ScheduledAt *time.Time `json:"scheduled_at"`
		PublishNow  bool       `json:"publish_now"`
		AutoSchedule bool      `json:"auto_schedule"` // pick best slot via smart scheduling
		MediaIDs    []string   `json:"media_ids"`
		Targets     []targetIn `json:"targets"`
	}
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if strings.TrimSpace(in.Content) == "" {
		return c.Status(400).JSON(fiber.Map{"error": "content required"})
	}
	if len(in.Targets) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "select at least one account target"})
	}
	status := models.StatusDraft
	if in.PublishNow {
		status = models.StatusScheduled // scheduler picks it up within seconds; or call publish directly
		now := time.Now()
		in.ScheduledAt = &now
	} else if in.ScheduledAt != nil {
		status = models.StatusScheduled
	} else if in.AutoSchedule {
		// smart scheduling: best slot for these target networks
		if slot := bestSlotFor(h.DB, in.Targets); slot != nil {
			in.ScheduledAt = slot
			status = models.StatusScheduled
		}
	}
	post := models.Post{
		Title: in.Title, Content: in.Content, Link: in.Link,
		ScheduledAt: in.ScheduledAt, Status: status,
	}
	if err := h.DB.Create(&post).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	// attach uploaded media
	for i, mid := range in.MediaIDs {
		h.DB.Model(&models.MediaAsset{}).Where("id = ?", mid).Updates(map[string]any{"post_id": post.ID, "sort_order": i})
	}
	for _, t := range in.Targets {
		h.DB.Create(&models.PostTarget{
			PostID: post.ID, AccountID: t.AccountID,
			CustomText: t.CustomText, FirstComment: t.FirstComment,
		})
	}
	// publish-now: run synchronously through shared helper
	if in.PublishNow {
		n, _ := PublishDuePost(h.DB, post.ID)
		_ = n
	}
	var out models.Post
	h.DB.Preload("Targets.Account").Preload("Media").First(&out, "id = ?", post.ID)
	return c.Status(201).JSON(out)
}

// Preview runs ValidateAndAdapt for every target WITHOUT posting.
func (h *PostHandler) Preview(c *fiber.Ctx) error {
	var in struct {
		Title   string     `json:"title"`
		Content string     `json:"content"`
		Link    string     `json:"link"`
		Media   []string   `json:"media_types"` // ["image","video",...]
		Targets []targetIn `json:"targets"`
	}
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	nImg, nVid := 0, 0
	for _, m := range in.Media {
		if m == "video" {
			nVid++
		} else {
			nImg++
		}
	}
	type row struct {
		AccountID string               `json:"account_id"`
		Network   models.Network       `json:"network"`
		Text      string               `json:"text"`
		Plan      networks.PlanResult  `json:"plan"`
	}
	var rows []row = []row{}
	for _, t := range in.Targets {
		var acct models.SocialAccount
		if err := h.DB.First(&acct, "id = ?", t.AccountID).Error; err != nil {
			continue
		}
		text := in.Content
		if t.CustomText != "" {
			text = t.CustomText
		}
		rows = append(rows, row{
			AccountID: acct.ID, Network: acct.Network, Text: text,
			Plan: networks.ValidateAndAdapt(acct.Network, text, in.Title, nImg, nVid, in.Link != ""),
		})
	}
	return c.JSON(fiber.Map{"targets": rows, "limits": networks.Limits()})
}

func (h *PostHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	h.DB.Delete(&models.PostTarget{}, "post_id = ?", id)
	h.DB.Delete(&models.MediaAsset{}, "post_id = ?", id)
	if err := h.DB.Delete(&models.Post{}, "id = ?", id).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}
