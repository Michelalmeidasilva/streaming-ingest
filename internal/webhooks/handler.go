package webhooks

import (
	"log"
	"github.com/gofiber/fiber/v2"
)

type WebhookProcessor interface {
	ProcessWebhook(provider string, payload []byte) error
}

type Handler struct {
	processor WebhookProcessor
}

func NewHandler(processor WebhookProcessor) *Handler {
	return &Handler{processor: processor}
}

func (h *Handler) HandleProviderWebhook(c *fiber.Ctx) error {
	provider := c.Params("provider")
	payload := c.Body()

	if err := h.processor.ProcessWebhook(provider, payload); err != nil {
		log.Printf("Webhook processing error: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Webhook processed successfully",
	})
}
