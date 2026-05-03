package uploadstate

import (
	"errors"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) SaveState(c *fiber.Ctx) error {
	var state UploadState
	if err := c.BodyParser(&state); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if state.Session.ID == "" || state.Video.ID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "session.id and video.id are required"})
	}
	if err := h.service.SaveState(c.UserContext(), state); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) GetState(c *fiber.Ctx) error {
	state, err := h.service.GetState(c.UserContext(), c.Params("sessionId"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "state not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(state)
}

func (h *Handler) DeleteSession(c *fiber.Ctx) error {
	if err := h.service.DeleteSession(c.UserContext(), c.Params("sessionId")); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) SaveVideo(c *fiber.Ctx) error {
	var video Video
	if err := c.BodyParser(&video); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if video.ID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "video.id is required"})
	}
	if err := h.service.SaveVideo(c.UserContext(), video); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) ListVideos(c *fiber.Ctx) error {
	videos, err := h.service.ListVideos(c.UserContext(), c.Query("q"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"videos": videos})
}

func (h *Handler) GetVideo(c *fiber.Ctx) error {
	video, err := h.service.GetVideo(c.UserContext(), c.Params("videoId"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "video not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"video": video})
}

func (h *Handler) PatchVideo(c *fiber.Ctx) error {
	var patch map[string]any
	if err := c.BodyParser(&patch); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	video, err := h.service.PatchVideo(c.UserContext(), c.Params("videoId"), patch)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "video not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"video": video})
}

func (h *Handler) DeleteVideo(c *fiber.Ctx) error {
	if err := h.service.DeleteVideo(c.UserContext(), c.Params("videoId")); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
