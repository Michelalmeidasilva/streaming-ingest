package webhooks

import (
	"log"
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) HandleProviderWebhook(c *fiber.Ctx) error {
	provider := c.Params("provider")
	payload := c.Body()

	if err := h.service.ProcessWebhook(provider, payload); err != nil {
		log.Printf("Webhook processing error: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Webhook processed successfully",
	})
}
