package scheduler

// Tickers: due posts (15s) + evergreen rules (60s) + token refresh (hourly).

import (
	"log"
	"time"

	"gorm.io/gorm"

	"solomon/backend/handlers"
	"solomon/backend/models"
)

func Start(db *gorm.DB, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	go func() {
		for {
			var due []models.Post
			db.Where("status = ? AND scheduled_at IS NOT NULL AND scheduled_at <= ? AND deleted_at IS NULL",
				models.StatusScheduled, time.Now()).Find(&due)
			for _, p := range due {
				log.Printf("scheduler: publishing post %s", p.ID)
				if _, err := handlers.PublishDuePost(db, p.ID); err != nil {
					log.Printf("scheduler: post %s error: %v", p.ID, err)
				}
			}
			time.Sleep(interval)
		}
	}()
	go func() {
		for {
			time.Sleep(time.Minute)
			if n := handlers.RunDueRules(db); n > 0 {
				log.Printf("scheduler: evergreen ran %d rule(s)", n)
			}
		}
	}()
	go func() {
		for {
			time.Sleep(time.Hour)
			if n, errs := handlers.RefreshDueAccounts(db); n > 0 || len(errs) > 0 {
				log.Printf("scheduler: token refresh: %d ok, %d errors", n, len(errs))
				for _, e := range errs {
					log.Printf("scheduler: token refresh error: %s", e)
				}
			}
			if d, notes := handlers.DiscoverExternal(db); d > 0 || len(notes) > 0 {
				log.Printf("scheduler: outside-solomon: %d new, %d notes", d, len(notes))
				for _, e := range notes {
					log.Printf("scheduler: discover note: %s", e)
				}
			}
		}
	}()
}
