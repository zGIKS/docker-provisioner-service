package httpapi

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"

	"go-service/internal/provisioner"
)

type Handler struct {
	service *provisioner.Service
}

type deprovisionRequest struct {
	ResourceID string `json:"resource_id"`
}

type errorResponse struct {
	Error      string `json:"error"`
	ResourceID string `json:"resource_id,omitempty"`
}

func NewHandler(service *provisioner.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(app *fiber.App) {
	app.Get("/healthz", h.healthz)
	app.Post("/api/v1/provision/tenants", h.provisionTenant)
	app.Delete("/api/v1/provision/resources/:resource_id", h.deprovisionByPath)
	app.Post("/api/v1/provision/deprovision", h.deprovisionByBody)
}

func (h *Handler) healthz(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) provisionTenant(c *fiber.Ctx) error {
	var req provisioner.ProvisionRequest
	if err := c.BodyParser(&req); err != nil {
		return writeError(c, fiber.StatusBadRequest, "invalid JSON body")
	}

	ctx := c.UserContext()
	if ctx == nil {
		ctx = context.Background()
	}

	result, err := h.service.ProvisionTenant(ctx, req)
	if err != nil {
		if errors.Is(err, provisioner.ErrInvalidTenant) {
			return writeError(c, fiber.StatusBadRequest, err.Error())
		}
		var alreadyProvisioned *provisioner.ErrAlreadyProvisioned
		if errors.As(err, &alreadyProvisioned) {
			return c.Status(fiber.StatusConflict).JSON(errorResponse{
				Error:      alreadyProvisioned.Error(),
				ResourceID: alreadyProvisioned.ResourceID,
			})
		}
		log.Printf(
			"provision failed request_id=%q tenant=%q tenant_id=%q: %v",
			c.Get("X-Request-ID"),
			req.TenantName,
			req.TenantID,
			err,
		)
		return writeError(c, fiber.StatusInternalServerError, "failed to provision tenant database")
	}

	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *Handler) deprovisionByPath(c *fiber.Ctx) error {
	resourceID := strings.TrimSpace(c.Params("resource_id"))
	return h.deprovision(c, resourceID)
}

func (h *Handler) deprovisionByBody(c *fiber.Ctx) error {
	var req deprovisionRequest
	if err := c.BodyParser(&req); err != nil {
		return writeError(c, fiber.StatusBadRequest, "invalid JSON body")
	}
	return h.deprovision(c, req.ResourceID)
}

func (h *Handler) deprovision(c *fiber.Ctx, resourceID string) error {
	ctx := c.UserContext()
	if ctx == nil {
		ctx = context.Background()
	}

	if err := h.service.Deprovision(ctx, resourceID); err != nil {
		if errors.Is(err, provisioner.ErrInvalidResource) {
			return writeError(c, fiber.StatusBadRequest, err.Error())
		}
		log.Printf(
			"deprovision failed request_id=%q resource=%q: %v",
			c.Get("X-Request-ID"),
			resourceID,
			err,
		)
		return writeError(c, fiber.StatusInternalServerError, "failed to deprovision resource")
	}

	return c.JSON(fiber.Map{
		"status":      "deprovisioned",
		"resource_id": strings.TrimSpace(resourceID),
	})
}

func writeError(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(errorResponse{Error: message})
}
