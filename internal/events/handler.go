package events

import (
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
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

	if err := h.service.ProcessEvent(event); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to process event",
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": "Event accepted",
	})
}
