package videos

import (
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ListVideos(c *fiber.Ctx) error {
	videos, err := h.service.ListAllVideos(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"videos": videos,
	})
}

func (h *Handler) SearchVideos(c *fiber.Ctx) error {
	query := c.Query("q")
	if query == "" {
		return h.ListVideos(c)
	}

	videos, err := h.service.SearchVideos(c.Context(), query)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"videos": videos,
	})
}
