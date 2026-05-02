package events

import (
	"github.com/gofiber/fiber/v2"
)

type EventProcessor interface {
	ProcessEvent(event FrontEndEvent) error
}

type Handler struct {
	processor EventProcessor
}

func NewHandler(processor EventProcessor) *Handler {
	return &Handler{processor: processor}
}

func (h *Handler) ReceiveEvent(c *fiber.Ctx) error {
	var event FrontEndEvent
	if err := c.BodyParser(&event); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot parse JSON",
		})
	}

	if event.EventType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "eventType is required",
		})
	}

	if err := h.processor.ProcessEvent(event); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to process event",
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": "Event accepted",
	})
}
