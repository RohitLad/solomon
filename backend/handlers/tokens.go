package handlers

// Token health: expiry alerts + manual/auto refresh.
// GET  /api/accounts/expiring   -> accounts expiring within 7d (or expired)
// POST /api/accounts/refresh    -> refresh all due tokens now

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"solomon/backend/models"
	"solomon/backend/networks"
)

type TokenHandler struct{ DB *gorm.DB }

func (h *TokenHandler) Expiring(c *fiber.Ctx) error {
	rows, _ := expiringAccounts(h.DB)
	if rows == nil {
		rows = []models.SocialAccount{}
	}
	for i := range rows {
		rows[i].AccessToken, rows[i].RefreshToken = "", ""
	}
	return c.JSON(rows)
}

func (h *TokenHandler) RefreshNow(c *fiber.Ctx) error {
	n, errs := RefreshDueAccounts(h.DB)
	return c.JSON(fiber.Map{"refreshed": n, "errors": errs})
}

func expiringAccounts(db *gorm.DB) ([]models.SocialAccount, error) {
	var all []models.SocialAccount
	if err := db.Find(&all).Error; err != nil {
		return nil, err
	}
	soon := time.Now().Add(7 * 24 * time.Hour)
	out := []models.SocialAccount{}
	for _, a := range all {
		if a.ExpiresAt == nil || !a.ExpiresAt.After(soon) {
			out = append(out, a)
		}
	}
	return out, nil
}

// RefreshDueAccounts refreshes tokens expiring within 7 days (or with no expiry).
// Called by the scheduler hourly and by POST /api/accounts/refresh.
func RefreshDueAccounts(db *gorm.DB) (int, []string) {
	due, _ := expiringAccounts(db)
	done := 0
	errs := []string{}
	for _, a := range due {
		res, err := networks.RefreshToken(string(a.Network), a.AccessToken, a.RefreshToken, a.Extra)
		if err != nil {
			errs = append(errs, a.Name+" ("+string(a.Network)+"): "+err.Error())
			continue
		}
		updates := map[string]any{"access_token": res.AccessToken}
		if res.RefreshToken != "" {
			updates["refresh_token"] = res.RefreshToken
		}
		exp := time.Now().Add(res.ExpiresIn)
		if res.ExpiresIn <= 0 {
			exp = time.Now().Add(60 * 24 * time.Hour)
		}
		updates["expires_at"] = &exp
		db.Model(&a).Updates(updates)
		done++
	}
	return done, errs
}
