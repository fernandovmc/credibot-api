package main

import (
	"credibot-api/config"
	"credibot-api/handlers"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
	config.LoadConfig()

	app := fiber.New(fiber.Config{
		StrictRouting: true,
		CaseSensitive: true,
		ErrorHandler: func(ctx *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return ctx.Status(code).JSON(fiber.Map{
				"error":   true,
				"message": err.Error(),
			})
		},
	})

	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path} ${latency}\n",
	}))

	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowMethods:     "GET,POST,HEAD,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization",
		AllowCredentials: false,
	}))

	// Debug middleware to log all requests
	app.Use(func(c *fiber.Ctx) error {
		log.Printf("[DEBUG] Method: %s, Path: %s, Headers: %v", c.Method(), c.Path(), c.GetReqHeaders())
		return c.Next()
	})

	// HEALTH
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "Credibot API running"})
	})

	// DEBUG endpoint to test POST
	app.Post("/test", func(c *fiber.Ctx) error {
		log.Printf("[DEBUG] POST /test received")
		return c.JSON(fiber.Map{
			"success": true,
			"message": "POST test endpoint working",
			"body":    string(c.Body()),
		})
	})

	// API Routes
	api := app.Group("/api/v1")

	// CHAT
	api.Post("/chat", handlers.Chat)
	api.Post("/smart-chat", handlers.SmartChat)

	// SUPABASE (READ-ONLY)
	api.Get("/data/:table", handlers.GetData)

	// START
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(app.Listen("0.0.0.0:" + port))
}
