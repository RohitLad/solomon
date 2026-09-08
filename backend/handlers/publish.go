package handlers

// PublishDuePost publishes ONE post to ALL its pending targets concurrently.
// Shared by the background scheduler and "publish now".

import (
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"solomon/backend/models"
	"solomon/backend/networks"
)

func PublishDuePost(db *gorm.DB, postID string) (int, error) {
	var post models.Post
	if err := db.Preload("Targets").Preload("Media").First(&post, "id = ?", postID).Error; err != nil {
		return 0, err
	}
	var mediaPaths []string
	var isVideo []bool
	nImg, nVid := 0, 0
	for _, m := range post.Media {
		mediaPaths = append(mediaPaths, m.FilePath)
		v := m.MediaType == models.MediaVideo
		isVideo = append(isVideo, v)
		if v {
			nVid++
		} else {
			nImg++
		}
	}

	var wg sync.WaitGroup
	for _, tgt := range post.Targets {
		if tgt.Status == models.TargetPublished {
			continue
		}
		wg.Add(1)
		go func(t models.PostTarget) {
			defer wg.Done()
			var acct models.SocialAccount
			if err := db.First(&acct, "id = ?", t.AccountID).Error; err != nil {
				db.Model(&t).Updates(map[string]any{"status": models.TargetFailed, "error": err.Error()})
				return
			}
			text := t.EffectiveText(post.Content)
			// Remedy: skip networks that REQUIRE media/title when missing (instead of failing loudly)
			plan := networks.ValidateAndAdapt(acct.Network, text, post.Title, nImg, nVid, post.Link != "")
			if !plan.OK {
				errMsg := strings.Join(plan.Errors, "; ")
				db.Model(&t).Updates(map[string]any{"status": models.TargetSkipped, "error": errMsg})
				return
			}
			res, err := networks.Publish(networks.PublishInput{
				Account: acct, Title: post.Title, Text: text,
				Link: post.Link, FirstComment: t.FirstComment,
				MediaPaths: mediaPaths, IsVideo: isVideo,
			})
			now := time.Now()
			if err != nil {
				db.Model(&t).Updates(map[string]any{"status": models.TargetFailed, "error": err.Error()})
				return
			}
			db.Model(&t).Updates(map[string]any{
				"status": models.TargetPublished, "network_post_id": strings.Join(res.NetworkPostIDs, ","),
				"published_at": &now, "error": res.Note,
			})
		}(tgt)
	}
	wg.Wait()

	// roll up post status
	var targets []models.PostTarget
	db.Where("post_id = ?", postID).Find(&targets)
	pub, fail := 0, 0
	for _, t := range targets {
		switch t.Status {
		case models.TargetPublished:
			pub++
		case models.TargetFailed:
			fail++
		}
	}
	status := models.StatusPublished
	if pub == 0 && fail > 0 {
		status = models.StatusFailed
	} else if fail > 0 || pub < len(targets) {
		status = models.StatusPartial
	}
	db.Model(&post).Update("status", status)
	return pub, nil
}
