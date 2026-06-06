// Package telemetry emits CloudWatch Embedded Metric Format (EMF) lines to stdout.
// CloudWatch Logs extracts RequestCount/RequestLatency/ErrorCount into metrics; no
// collector or scrape endpoint is involved.
package telemetry

import (
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Emitter writes one EMF JSON object per call to Out.
type Emitter struct {
	Service string
	Out     io.Writer
	Now     func() time.Time
}

// New returns an Emitter writing to stdout with the real clock.
func New(service string) *Emitter {
	return &Emitter{Service: service, Out: os.Stdout, Now: time.Now}
}

// Emit writes a single RED EMF record.
func (e *Emitter) Emit(route, method string, status int, latency time.Duration) {
	errCount := 0
	if status >= 500 {
		errCount = 1
	}
	record := map[string]any{
		"_aws": map[string]any{
			"Timestamp": e.Now().UnixMilli(),
			"CloudWatchMetrics": []map[string]any{{
				"Namespace":  "VOD/" + e.Service,
				"Dimensions": [][]string{{"service", "route", "method"}},
				"Metrics": []map[string]string{
					{"Name": "RequestCount", "Unit": "Count"},
					{"Name": "RequestLatency", "Unit": "Milliseconds"},
					{"Name": "ErrorCount", "Unit": "Count"},
				},
			}},
		},
		"service":        e.Service,
		"route":          route,
		"method":         method,
		"RequestCount":   1,
		"RequestLatency": float64(latency.Microseconds()) / 1000.0,
		"ErrorCount":     errCount,
	}
	b, err := json.Marshal(record)
	if err != nil {
		return
	}
	_, _ = e.Out.Write(append(b, '\n'))
}

// Middleware returns a Fiber handler that emits one EMF record per request.
// route uses the matched route pattern (low cardinality).
func (e *Emitter) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := e.Now()
		err := c.Next()
		route := c.Route().Path
		if route == "" {
			route = c.Path()
		}
		e.Emit(route, c.Method(), c.Response().StatusCode(), e.Now().Sub(start))
		return err
	}
}
