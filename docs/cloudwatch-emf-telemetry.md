# CloudWatch EMF Telemetry

## Motivation

The production target for `streaming-ingest` is AWS Lambda (or ECS Fargate), where
pull-based scrape endpoints are invalid: there is no persistent process to scrape and no
Prometheus-compatible collector running alongside the function. The previous pipeline —
`fiberprometheus` middleware exposing `GET /metrics` + an OTLP push pipeline pointing at
`streaming-telemetry` — was dead weight once the collector was removed. Stdout is the
only reliable, zero-infrastructure output channel for both Lambda and local development.

## What Changed

- **Removed:** `internal/otel/` package (OTLP gRPC push pipeline), the `otelfiber`
  middleware registered in `cmd/api/main.go`, and the `fiberprometheus` middleware that
  drove `GET /metrics`. The `GET /metrics` endpoint no longer exists.
- **Added:** `internal/telemetry/emf.go` — a Fiber middleware that emits one structured
  JSON log line per HTTP request in CloudWatch Embedded Metric Format (EMF). The record
  includes RED metrics (`RequestCount`, `RequestLatency`, `ErrorCount`) with dimensions
  `service`, `route`, and `method`.

## EMF Contract

Each completed HTTP request produces a single line written to stdout:

```json
{
  "_aws": {
    "Timestamp": 1717689600000,
    "CloudWatchMetrics": [{
      "Namespace": "VOD/streaming-ingest",
      "Dimensions": [["service","route","method"]],
      "Metrics": [
        {"Name":"RequestCount","Unit":"Count"},
        {"Name":"RequestLatency","Unit":"Milliseconds"},
        {"Name":"ErrorCount","Unit":"Count"}
      ]
    }]
  },
  "service": "streaming-ingest",
  "route": "/api/v1/events",
  "method": "POST",
  "RequestCount": 1,
  "RequestLatency": 42.5,
  "ErrorCount": 0
}
```

`ErrorCount` is `1` when the response status code is `>= 500`, otherwise `0`.

## Dev / Prod Data Flow

The same EMF JSON is emitted to stdout in both environments; in production, CloudWatch
Logs Agent (Lambda built-in, or the ECS log driver) captures stdout, and CloudWatch Logs
automatically extracts the embedded metrics into the `VOD/streaming-ingest` namespace —
no collector, no sidecar. Local wiring (LocalStack + log group) is covered by Plan 2 of
the observability migration (infra work, not in scope here).

## Reference

Design spec: `infra/docs/design-docs/specs/2026-06-06-cloudwatch-observability-migration-design.md`
