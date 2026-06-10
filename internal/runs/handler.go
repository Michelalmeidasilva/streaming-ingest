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

// BenchmarkWriter records a single benchmark measurement. *Service satisfies it.
type BenchmarkWriter interface {
	RecordBenchmarkRun(ctx context.Context, run Run) error
}

type Handler struct {
	reader Reader
	writer BenchmarkWriter
}

func NewHandler(reader Reader) *Handler {
	return &Handler{reader: reader}
}

func (h *Handler) SetBenchmarkWriter(w BenchmarkWriter) { h.writer = w }

func (h *Handler) CreateBenchmarkRun(c *fiber.Ctx) error {
	var run Run
	if err := c.BodyParser(&run); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if h.writer == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "benchmark writer not configured"})
	}
	if err := h.writer.RecordBenchmarkRun(c.Context(), run); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"status": "recorded"})
}

func (h *Handler) ListRuns(c *fiber.Ctx) error {
	filter := Filter{
		MachineLabel: c.Query("machineLabel"),
		Codec:        c.Query("codec"),
	}
	switch c.Query("benchmark") {
	case "true":
		v := true
		filter.Benchmark = &v
	case "false":
		v := false
		filter.Benchmark = &v
	}
	runs, err := h.reader.List(c.Context(), filter)
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
