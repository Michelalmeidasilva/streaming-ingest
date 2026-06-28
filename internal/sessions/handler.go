package sessions

import (
	"context"

	"github.com/gofiber/fiber/v2"
)

// Reader is the read surface the handler needs.
type Reader interface {
	List(ctx context.Context, filter SessionFilter) ([]Session, error)
}

// Writer is the write surface the handler needs.
type Writer interface {
	CreateSession(ctx context.Context, sess Session) error
}

// Handler handles HTTP requests for benchmark sessions.
type Handler struct {
	reader Reader
	writer Writer
}

// NewHandler constructs a Handler with the given Reader.
func NewHandler(reader Reader) *Handler {
	return &Handler{reader: reader}
}

// SetWriter configures the write dependency.
func (h *Handler) SetWriter(w Writer) { h.writer = w }

// CreateSession handles POST /api/v1/benchmark-sessions.
// It stores the launched-session document and returns 202 on success.
func (h *Handler) CreateSession(c *fiber.Ctx) error {
	var sess Session
	if err := c.BodyParser(&sess); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if h.writer == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "session writer not configured"})
	}
	if err := h.writer.CreateSession(c.Context(), sess); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"status": "recorded"})
}

// ListSessions handles GET /api/v1/benchmark-sessions.
// Accepts optional ?sessionId= query parameter to filter results.
func (h *Handler) ListSessions(c *fiber.Ctx) error {
	filter := SessionFilter{
		SessionID: c.Query("sessionId"),
	}
	sessions, err := h.reader.List(c.Context(), filter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if sessions == nil {
		sessions = []Session{}
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"sessions": sessions})
}
