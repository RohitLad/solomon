package main

// Solomon — social media scheduler.
// Run:  cd backend && go mod tidy && go run .   (API on :8080)
// Frontend (../frontend) talks to it at http://localhost:8080.

import (
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"

	"solomon/backend/db"
	"solomon/backend/handlers"
	"solomon/backend/scheduler"
)

func main() {
	_ = godotenv.Load()
	_ = godotenv.Load("../.env")

	database, err := db.Connect(env("DB_PATH", "./solomon.db"))
	if err != nil {
		log.Fatal(err)
	}

	app := fiber.New(fiber.Config{BodyLimit: 100 * 1024 * 1024}) // 100MB video uploads
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{AllowOrigins: "*", AllowHeaders: "Origin, Content-Type, Accept, Authorization"}))
	app.Static("/uploads", handlers.UploadDir())

	accounts := &handlers.AccountHandler{DB: database}
	posts := &handlers.PostHandler{DB: database}
	tokens := &handlers.TokenHandler{DB: database}
	analytics := &handlers.AnalyticsHandler{DB: database}
	schedule := &handlers.ScheduleHandler{DB: database}
	bulk := &handlers.BulkHandler{DB: database}
	evergreen := &handlers.EvergreenHandler{DB: database}

	api := app.Group("/api")
	api.Get("/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })
	api.Get("/limits", handlers.Limits)

	api.Get("/accounts", accounts.List)
	api.Post("/accounts", accounts.Create)
	api.Delete("/accounts/:id", accounts.Delete)
	api.Get("/accounts/:network/auth-url", accounts.AuthURL)
	api.Get("/accounts/expiring", tokens.Expiring)
	api.Post("/accounts/refresh", tokens.RefreshNow)

	api.Get("/posts", posts.List)
	api.Post("/posts", posts.Create)
	api.Patch("/posts/:id", posts.Update)
	api.Post("/posts/preview", posts.Preview)
	api.Delete("/posts/:id", posts.Delete)
	api.Post("/upload", handlers.Upload(database))
	api.Post("/posts/bulk", bulk.Import)

	api.Post("/ai/captions", handlers.Captions)

	api.Get("/schedule/suggest", schedule.Suggest)

	api.Get("/analytics", analytics.List)
	api.Post("/analytics/refresh", analytics.Refresh)

	api.Get("/evergreen", evergreen.List)
	api.Post("/evergreen", evergreen.Create)
	api.Delete("/evergreen/:id", evergreen.Delete)

	scheduler.Start(database, 15*time.Second)

	port := env("PORT", "8080")
	log.Printf("Solomon API on :%s", port)
	log.Fatal(app.Listen(":" + port))
}

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
