package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
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

	app := fiber.New()
	handler.Register(app)

	log.Printf("provisioner listening on :%s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
