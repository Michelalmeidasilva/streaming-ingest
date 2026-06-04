# Subtitle reference propagation

## Motivation
Sidecar `.srt` subtitles are uploaded with their video but the transcoder is
triggered by the *video's* storage webhook, which knows nothing about the
subtitle objects. The gateway carries the references across, exactly like raw
video geometry (see [raw-video-metadata-propagation.md](./raw-video-metadata-propagation.md)).

## Design
1. **Persist** — the upload platform includes `subtitles` (objectKey/language/
   label) in the `upload.started` payload. `events.Service`
   (`subtitlesFromPayload`) stores them on the video record
   (`Video.Subtitles`, `adapters.SubtitleRef`). Entries without an objectKey are
   skipped.
2. **Forward** — on the video's `ObjectCreated` webhook, `webhooks.Service`
   looks the record up (`FindByVideoID`) and attaches `Subtitles` to the
   published `video.upload.completed` `DomainEvent` (and re-applies them on the
   webhook `Save`).
3. **Ignore subtitle webhooks** — `.srt` objects are uploaded under the
   `subtitles/` prefix, which is added to `pipelineOutputPrefixes`, so a
   subtitle upload does not get ingested as a new video and trigger its own
   transcode.

## Contract changes
- `adapters.SubtitleRef { ObjectKey, Language, Label }`.
- `adapters.DomainEvent.Subtitles []SubtitleRef`.
- `videos.Video.Subtitles` (`bson:"subtitles"`).

## Caveats
- Same ordering assumption as raw video: `upload.started` (with the subtitle
  refs) is processed before the video's `ObjectCreated` webhook. If the subtitle
  bytes have not finished uploading when the transcoder runs, the conversion of
  that track fails the job — the client uploads `.srt` (small) right after
  initiate, well before the (larger) video finishes.
