package runs

import (
	"context"

	"github.com/gofiber/fiber/v2"
)

// Reader is the read surface the handler needs. *MongoRepository satisfies it.
type Reader interface {
	List(ctx context.Context, filter Filter) ([]Run, error)
	GetByVideoID(ctx context.Context, videoID string) ([]Run, error)
}

type Handler struct {
	reader Reader
}

func NewHandler(reader Reader) *Handler {
	return &Handler{reader: reader}
}

func (h *Handler) ListRuns(c *fiber.Ctx) error {
	runs, err := h.reader.List(c.Context(), Filter{
		MachineLabel: c.Query("machineLabel"),
		Codec:        c.Query("codec"),
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if runs == nil {
		runs = []Run{}
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"runs": runs})
}

func (h *Handler) GetRuns(c *fiber.Ctx) error {
	runs, err := h.reader.GetByVideoID(c.Context(), c.Params("videoId"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if runs == nil {
		runs = []Run{}
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"runs": runs})
}
