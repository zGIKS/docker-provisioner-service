package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/joho/godotenv"

	"go-service/internal/config"
	"go-service/internal/docker"
	"go-service/internal/httpapi"
	"go-service/internal/provisioner"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	runner := docker.NewRuntime(cfg.DockerBin, cfg.DockerCommandTimeout)
	service := provisioner.NewService(runner, cfg)
	handler := httpapi.NewHandler(service)

	app := fiber.New(fiber.Config{
		BodyLimit:    cfg.HTTPBodyLimitBytes,
		ReadTimeout:  cfg.HTTPReadTimeout,
		WriteTimeout: cfg.HTTPWriteTimeout,
		IdleTimeout:  cfg.HTTPIdleTimeout,
	})
	app.Use(requestid.New(requestid.Config{
		Header: "X-Request-ID",
	}))
	app.Use(limiter.New(limiter.Config{
		Max:        cfg.RateLimitMax,
		Expiration: cfg.RateLimitWindow,
		Next: func(c *fiber.Ctx) bool {
			return c.Path() == "/healthz"
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "rate limit exceeded",
			})
		},
	}))
	handler.Register(app)

	log.Printf("provisioner listening on :%s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
