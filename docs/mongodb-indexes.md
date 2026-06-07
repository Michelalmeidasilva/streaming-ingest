# MongoDB indexes

## Motivation

A performance audit found the Event Gateway had **no MongoDB indexes** defined
anywhere. Every lookup by `video_id` / `session_id` and the upload-state list
sort fell back to a full collection scan (COLLSCAN). These run on hot paths:

- the storage webhook looks up the canonical video by `video_id` before publishing;
- the admin UI polls `GET /api/v1/upload-state/videos` (sorted by `created_at`)
  every 3 seconds per open dashboard;
- multipart sessions are read/written by `session_id` on every chunk.

As the catalog grows, these scans dominate latency.

## Design

`internal/mongoindex` owns the index definitions and a single idempotent
`EnsureIndexes` entry point. It is decoupled from the repositories via a small
`IndexManager` seam (`CreateMany`), satisfied by `*mongo.IndexView` from
`Collection.Indexes()` and by a recording fake in tests — so the repository
mocks were left untouched.

`cmd/api/main.go` calls `ensureMongoIndexes` right after the Mongo connection is
established.

### Indexes created (db `streaming`)

| Collection | Index | Purpose |
|---|---|---|
| `videos` | `video_id` (unique) | per-video lookup/upsert; also guards the duplicate-document bug |
| `videos` | `created_at` (desc) | upload-state list sort polled by the admin UI |
| `videos_catalog` | `video_id` (unique) | catalog upsert/lookup key |
| `upload_sessions` | `session_id` (unique) | multipart session lookup key |

## Behaviour

- **Idempotent**: re-running creates nothing new; safe on every boot.
- **Non-fatal**: failures are logged (`WARNING: could not ensure MongoDB
  indexes`) and startup continues. A missing index degrades to a scan rather
  than an outage.
- **Unique-index caveat**: if pre-existing duplicate `video_id` / `session_id`
  documents exist, the unique index cannot be built and the warning fires; clean
  up duplicates, then restart to let the index build.

## Coordination with streaming-distribution

streaming-distribution shares the `videos` collection and also ensures a unique
`video_id` index (plus `status`). Identical key + options make the duplicate
creation a conflict-free no-op.

## Tests

`internal/mongoindex/indexes_test.go` asserts that the expected unique/regular
index keys are requested per collection.
