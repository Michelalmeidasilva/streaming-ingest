# Transcode Runs

## Overview

The `transcode_runs` MongoDB collection stores one document per completed transcode job. It is
populated as a best-effort side effect when the Event Gateway receives a `transcode.completed`
event on `POST /api/v1/events`. The data enables the upload platform to compare codec processing
performance across EC2 instance types without any dedicated metrics infrastructure.

## Data Model

### Run document fields

| Field (bson) | JSON | Type | Description |
|---|---|---|---|
| `job_id` | `jobId` | string | Unique transcode job identifier. Unique index — idempotency key. |
| `video_id` | `videoId` | string | Platform video UUID |
| `machine_label` | `machineLabel` | string | `TRANSCODE_MACHINE_LABEL` env var from the worker, or `os.Hostname()` fallback |
| `hostname` | `hostname` | string | Actual worker container/instance hostname |
| `cpu_cores` | `cpuCores` | int | `runtime.NumCPU()` on the worker |
| `profile` | `profile` | string | Rendition profile identifier |
| `elapsed_seconds` | `elapsedSeconds` | float64 | Wall-clock seconds for the full transcode job |
| `rtf` | `rtf` | float64 | Real-time factor (elapsed / source duration) |
| `source_file_size_bytes` | `sourceFileSizeBytes` | int64 | Size of the downloaded raw source file |
| `total_output_size_bytes` | `totalOutputSizeBytes` | int64 | Sum of all packaged output file sizes |
| `completed_at` | `completedAt` | time.Time | Completion timestamp from the event payload |
| `created_at` | `createdAt` | time.Time | Time the document was first written (`$setOnInsert`) |
| `renditions` | `renditions` | array | Per-rendition metrics (see below) |

### Rendition sub-document fields

| Field | Type | Description |
|---|---|---|
| `name` | string | Rendition label (e.g. `360p`, `1080p`) |
| `codec` | string | Codec ID (e.g. `h264`, `h265`, `av1`) |
| `width` | int | Output width in pixels |
| `height` | int | Output height in pixels |
| `preset` | string | ffmpeg preset (e.g. `fast`, `medium`) |
| `targetBitrateKbps` | int | Configured target bitrate |
| `outputBitrateKbps` | int | Actual measured output bitrate |
| `elapsedSeconds` | float64 | Wall-clock seconds for this rendition |
| `avgCpuPercent` | float64 | Average CPU % during this rendition's ffmpeg process |
| `maxCpuPercent` | float64 | Peak CPU % |
| `avgMemoryMb` | float64 | Average RSS memory in MB |
| `maxMemoryMb` | float64 | Peak RSS memory in MB |

## Mapping from transcode.completed Payload

The `transcode.completed` event body carries a top-level `machineLabel` field (for direct
indexing) and a nested `observability` object with all other metrics. The gateway maps these
to the run document as follows:

```
event.jobId              → job_id
event.videoId            → video_id
event.machineLabel       → machine_label
event.observability.*    → remaining run fields
event.observability.renditions[] → renditions[]
```

`completed_at` is taken from `event.completedAt` (or the current server time as fallback).
`created_at` is set via `$setOnInsert` so re-delivery of the same `jobId` does not update it.

## Idempotency

The upsert uses `filter: { job_id: <jobId> }` with `$set` for all mutable fields and
`$setOnInsert: { created_at: now }` for the insert-only timestamp. If the same event is
delivered twice (e.g. RabbitMQ redelivery or EventBridge retry), the second upsert is a
no-op for `created_at` and overwrites the other fields with identical values — effectively
idempotent.

## Indexes

Both indexes are ensured at startup via `internal/mongoindex` (idempotent, non-fatal on failure):

| Index | Type | Purpose |
|---|---|---|
| `{ job_id: 1 }` | unique | Idempotency guard and single-run lookup |
| `{ machine_label: 1, completed_at: -1 }` | compound | Filter-by-label queries sorted by recency |

## Read Endpoints

### GET /api/v1/runs

Returns all run documents sorted by `completed_at` descending.

Query params:
- `machineLabel` — filter by exact machine label
- `codec` — filter by codec (matches any rendition's `codec` field)

### GET /api/v1/runs/:videoId

Returns the single run document for the given `videoId`. Returns `404` if no run has been
recorded for that video yet (the transcode may still be in progress, or the run write may
have failed silently).

## Best-Effort Guarantee

A failure to write the run document (e.g. MongoDB unavailable, malformed payload) is logged
at WARN level but does **not** fail the `POST /api/v1/events` response or block the RabbitMQ
publish. The Event Gateway's contract — receive the event, publish to the exchange — is
preserved regardless of the run-write outcome. The `transcode_runs` collection is a
read-path enrichment; it is never in the critical write path.
