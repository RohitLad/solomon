package handlers

// POST /api/ai/captions {text, network} -> 3 tone variants + hashtags.

import (
	"github.com/gofiber/fiber/v2"

	"solomon/backend/models"
	"solomon/backend/networks"
)

func Captions(c *fiber.Ctx) error {
	var in struct {
		Text    string         `json:"text"`
		Network models.Network `json:"network"`
	}
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if in.Network == "" {
		in.Network = models.NetworkTwitter
	}
	variants, source, err := networks.GenerateCaptions(in.Text, in.Network)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"variants": variants, "source": source, "limits": networks.Limits()[in.Network]})
}
