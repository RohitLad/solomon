package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"solomon/backend/models"
	"solomon/backend/networks"
)

type AccountHandler struct{ DB *gorm.DB }

// List all connected accounts.
func (h *AccountHandler) List(c *fiber.Ctx) error {
	accounts := []models.SocialAccount{}
	if err := h.DB.Order("network, name").Find(&accounts).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	// hide tokens
	for i := range accounts {
		accounts[i].AccessToken, accounts[i].RefreshToken = "", ""
	}
	return c.JSON(accounts)
}

// Create supports TWO modes:
//  1. Manual/demo: {"network":"twitter","name":"@acme","access_token":"demo-123"} — instant, for testing.
//  2. Real OAuth: frontend opens GET /api/accounts/:network/auth-url, then POSTs the
//     exchanged token + profile here. (Token exchange code itself is provider-specific,
//     see README — paste resulting token.)
func (h *AccountHandler) Create(c *fiber.Ctx) error {
	var in struct {
		Network      models.Network `json:"network"`
		Name         string         `json:"name"`
		ExternalID   string         `json:"external_id"`
		AccessToken  string         `json:"access_token"`
		RefreshToken string         `json:"refresh_token"`
		Extra        string         `json:"extra"`
	}
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if in.Network == "" || in.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "network and name required"})
	}
	acct := models.SocialAccount{
		Network: in.Network, Name: in.Name, ExternalID: in.ExternalID,
		AccessToken: in.AccessToken, RefreshToken: in.RefreshToken,
		Extra: in.Extra, IsActive: true,
	}
	if acct.AccessToken == "" {
		acct.AccessToken = "demo-" + string(acct.Network)
	}
	// Best-effort avatar (never fails account creation; demo "" → initials UI).
	if url, err := networks.FetchAvatar(acct.Network, acct.AccessToken, acct.Extra, acct.ExternalID); err == nil {
		acct.AvatarURL = url
	}
	expires := time.Now().Add(60 * 24 * time.Hour)
	acct.ExpiresAt = &expires
	if err := h.DB.Create(&acct).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	acct.AccessToken, acct.RefreshToken = "", ""
	return c.Status(201).JSON(acct)
}

func (h *AccountHandler) Delete(c *fiber.Ctx) error {
	if err := h.DB.Delete(&models.SocialAccount{}, "id = ?", c.Params("id")).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

// AuthURL returns the OAuth consent URL for a network.
func (h *AccountHandler) AuthURL(c *fiber.Ctx) error {
	u, err := networks.AuthURL(models.Network(c.Params("network")))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"auth_url": u})
}

// Limits exposes per-network limits to the frontend (char counters etc).
func Limits(c *fiber.Ctx) error {
	return c.JSON(networks.Limits())
}
