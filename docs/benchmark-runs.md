# Benchmark Runs Endpoint

## Overview

`POST /api/v1/benchmark-runs` is a dedicated write path for the codec benchmark harness
(`streaming-transcode cmd/benchmark`). It inserts one `transcode_runs` document per
codec×resolution×clip×repeat cell, tagged `benchmark=true`, with no side-effects on the
video catalog or upload-state lifecycle.

## Document Model

Benchmark run documents share the `transcode_runs` MongoDB collection with production run
documents. The `benchmark` boolean field is the partition key.

| Field (bson) | JSON | Type | Description |
|---|---|---|---|
| `job_id` | `jobId` | string | Unique per benchmark cell (unique index key) |
| `machine_label` | `machineLabel` | string | EC2 instance type or `BENCHMARK_MACHINE_LABEL` |
| `hostname` | `hostname` | string | Container/instance hostname |
| `cpu_cores` | `cpuCores` | int | CPU core count |
| `clip` | `clip` | string | S3 key of the corpus clip |
| `repetition` | `repetition` | int | Repeat index within the matrix cell (1-based) |
| `benchmark` | `benchmark` | bool | Always `true` for documents written via this endpoint |
| `elapsed_seconds` | `elapsedSeconds` | float64 | Wall-clock encode time for all renditions |
| `source_width` | `sourceWidth` | int | Source clip width in pixels (0 if probe failed) |
| `source_height` | `sourceHeight` | int | Source clip height in pixels (0 if probe failed) |
| `source_duration_seconds` | `sourceDurationSeconds` | float64 | Source clip duration in seconds (0 if probe failed) |
| `source_fps` | `sourceFps` | float64 | Source clip frame rate (0 if probe failed) |
| `source_codec` | `sourceCodec` | string | Source clip video codec, e.g. `h264` (empty if probe failed) |
| `source_bitrate_kbps` | `sourceBitrateKbps` | int | Source clip container bitrate in kbps (0 if probe failed) |
| `source_file_size_bytes` | `sourceFileSizeBytes` | int64 | Source clip file size in bytes (0 if probe failed) |
| `renditions` | `renditions` | array | Per-rendition metrics (name, codec, elapsed, avg/max CPU, output bitrate, etc.) |
| `completed_at` | `completedAt` | time.Time | Timestamp from the request payload |
| `created_at` | `createdAt` | time.Time | Time of insert |

Production-only fields (`videoId`, `rtf`, `totalOutputSizeBytes`, `profile`) are not set
by benchmark runs. `sourceFileSizeBytes` is shared — production runs set it from the raw
source; benchmark runs set it from the probed corpus clip.

## Endpoint Contract

**`POST /api/v1/benchmark-runs`**

Request:
```json
{
  "jobId": "<uuid>",
  "machineLabel": "c5.xlarge",
  "hostname": "ip-10-0-1-42.us-east-2.compute.internal",
  "cpuCores": 4,
  "clip": "benchmark/corpus/sample-720p.mp4",
  "repetition": 1,
  "benchmark": true,
  "elapsedSeconds": 28.4,
  "sourceWidth": 1280,
  "sourceHeight": 720,
  "sourceDurationSeconds": 48.7,
  "sourceFps": 29.97,
  "sourceCodec": "h264",
  "sourceBitrateKbps": 4200,
  "sourceFileSizeBytes": 25559040,
  "completedAt": "2026-06-10T12:00:00Z",
  "renditions": [
    {
      "name": "720p",
      "codec": "h264",
      "width": 1280,
      "height": 720,
      "preset": "fast",
      "targetBitrateKbps": 2800,
      "outputBitrateKbps": 2743,
      "elapsedSeconds": 28.4,
      "avgCpuPercent": 94.2,
      "maxCpuPercent": 99.8,
      "avgMemoryMb": 320.0,
      "maxMemoryMb": 410.0
    }
  ]
}
```

Response `202`:
```json
{ "message": "Benchmark run recorded" }
```

- `400` — validation failure (missing required fields)
- `500` — database error (e.g. unique-index violation on `jobId`)

The write is a plain **insert** — not upsert — so re-posting the same `jobId` fails at
the unique index. The benchmark harness logs the failure and continues with the next cell.

## Isolation from the Video Catalog

Benchmark runs:
- Are **never** patched onto upload-state video records (`videos` collection).
- Are **never** written to `videos_catalog`.
- Do not trigger RabbitMQ publishes.
- Do not update any video lifecycle status.

The gateway remains ingest-and-publish only. `POST /api/v1/benchmark-runs` is a pure
side-channel store into the `transcode_runs` partition.

## Production vs. Benchmark Filter

`GET /api/v1/runs` accepts a `benchmark` query parameter:

| Parameter | MongoDB filter | Semantics |
|---|---|---|
| `?benchmark=true` | `{ benchmark: true }` | Benchmark runs only |
| `?benchmark=false` (default) | `{ benchmark: { $ne: true } }` | Production runs only |

The `$ne: true` form ensures that production run documents written before the `benchmark`
field was introduced (i.e. where the field is absent) appear in the production view. New
production runs written via `POST /api/v1/events` (the `transcode.completed` path) never
set `benchmark`, so they naturally satisfy `$ne: true`.

## Consumed By

- `streaming-transcode cmd/benchmark` — posts one document per matrix cell.
- `streaming-platform-upload GET /api/runs?benchmark=true` — feeds the Benchmark view in
  the Metrics tab, grouping results by `machineLabel` with per codec×resolution tables.
