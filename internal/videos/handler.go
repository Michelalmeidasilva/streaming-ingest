package videos

import (
	"context"
	"github.com/gofiber/fiber/v2"
)

type VideoService interface {
	ListAllVideos(ctx context.Context) ([]VideoResponse, error)
	ListDatabaseVideos(ctx context.Context) ([]VideoResponse, error)
	SearchVideos(ctx context.Context, query string) ([]VideoResponse, error)
}

type Handler struct {
	service VideoService
}

func NewHandler(service VideoService) *Handler {
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

func (h *Handler) ListDatabaseVideos(c *fiber.Ctx) error {
	videos, err := h.service.ListDatabaseVideos(c.Context())
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
