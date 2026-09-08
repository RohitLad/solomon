package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"solomon/backend/models"
)

// UploadDir stores user media (images/videos). Served at /uploads/*.
func UploadDir() string {
	d := os.Getenv("UPLOAD_DIR")
	if d == "" {
		d = "./uploads"
	}
	_ = os.MkdirAll(d, 0o755)
	return d
}

// Upload accepts multipart files (field "files"). Returns MediaAssets (unattached until post created).
func Upload(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		form, err := c.MultipartForm()
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "send multipart files as 'files'"})
		}
		files := form.File["files"]
		if len(files) == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "no files"})
		}
		out := []models.MediaAsset{}
		for _, fh := range files {
			ext := strings.ToLower(filepath.Ext(fh.Filename))
			mt := models.MediaImage
			switch ext {
			case ".mp4", ".mov", ".webm", ".mkv":
				mt = models.MediaVideo
			case ".jpg", ".jpeg", ".png", ".gif", ".webp":
				mt = models.MediaImage
			default:
				return c.Status(400).JSON(fiber.Map{"error": fmt.Sprintf("unsupported file type %s", ext)})
			}
			name := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), uuid.NewString()[:8], ext)
			dst := filepath.Join(UploadDir(), name)
			if err := c.SaveFile(fh, dst); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			m := models.MediaAsset{FilePath: dst, MediaType: mt}
			// PostID empty until attached to a post
			if err := db.Create(&m).Error; err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			out = append(out, m)
		}
		return c.Status(201).JSON(out)
	}
}
