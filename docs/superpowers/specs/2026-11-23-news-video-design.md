# News Video — Design

## Goal

An editor attaches one video to a news article: either an uploaded file stored in
Cloudflare R2 or a YouTube/Vimeo link. The video appears in the middle of the article text.

## Decisions (agreed with the user)

- One video per article.
- Two sources: an uploaded file (mp4/webm, ≤ 500 MB) or a YouTube/Vimeo link.
- Placement: automatic, in the middle of the text (no marker; simplified after review
  with the user).
- Files go from the browser straight to R2 through a presigned PUT URL; the backend
  never proxies video bytes.

## Data

Migration `00015_news_video.sql` (adds columns only, no data changes):

```sql
ALTER TABLE news
    ADD COLUMN video_kind TEXT CHECK (video_kind IN ('file', 'embed')),
    ADD COLUMN video_url  TEXT NOT NULL DEFAULT '',
    ADD CONSTRAINT news_video_consistent CHECK ((video_kind IS NULL) = (video_url = ''));
```

Down drops the constraint and both columns.

Domain: `models.News` gains `Video *models.Video` with `Kind VideoKind` (`file` | `embed`)
and `URL string`. Nil means no video.

## Upload flow

1. `POST /api/admin/news/video-uploads` (admin auth) with `{"content_type", "size"}`.
   The service accepts `video/mp4` and `video/webm` and `0 < size ≤ 500 MB`, and generates
   the key `pending/videos/<32 hex>.<mp4|webm>`. It returns
   `{"key", "upload_url", "expires_at"}`: a presigned PUT URL that lives 15 minutes and
   signs `Content-Type` and `Content-Length`.
2. The browser PUTs the file to `upload_url` and shows its progress.
3. The create/update request carries `video_key`. Before any DB write the service:
   - requires the `pending/videos/` prefix and the key format above (no arbitrary keys);
   - `HeadObject`: the object exists and its size is ≤ 500 MB;
   - reads the first 512 bytes (ranged GET) and sniffs the content. mp4 means `ftyp` at
     offset 4; webm means the EBML magic `1A 45 DF A3`. The sniffed type must match the
     key extension, and the client-declared MIME is not trusted.
   - A pending object that exists but fails the size or signature check is deleted right
     away (the key is verified to be ours). A malformed key never leads to a delete.
4. The service copies the object server-side to `videos/<slug>-<8 hex>.<ext>`, saves the
   article with the public URL, then deletes the pending object. If the save fails, it
   deletes the copy and does not delete the old video.
5. An R2 lifecycle rule removes leftovers under `pending/` after 1 day (abandoned
   uploads). This is configured in Cloudflare, not in code.

`ImageStorage` stays as is. A separate service-owned `VideoStorage` interface
(`PresignPut`, `Inspect` (size and head bytes), `Copy`, `Delete`) is implemented by the
same `r2.Client`. No new dependencies are needed: `s3.PresignClient` comes from the
existing `aws-sdk-go-v2/service/s3`.

## Embed links

`video_url` accepts only:

- `youtube.com/watch?v=<id>`, `youtu.be/<id>`, `youtube.com/shorts/<id>`,
  `youtube.com/embed/<id>`, `youtube-nocookie.com/embed/<id>` → stored as
  `https://www.youtube-nocookie.com/embed/<id>` (id `[A-Za-z0-9_-]{11}`);
- `vimeo.com/<id>`, `player.vimeo.com/video/<id>` → `https://player.vimeo.com/video/<id>`
  (id is digits).

Any other host or format is a validation error (400).

## Update semantics

Create and update are full-replace, the same as the cover:

| request                                  | result                                   |
|------------------------------------------|------------------------------------------|
| `video_key` set                          | new uploaded file (wins over `video_url`) |
| `video_url` = the article's current file URL | keep the current file                 |
| `video_url` = YouTube/Vimeo link         | embed; the previous owned file is deleted after the save |
| both empty                               | no video; the previous owned file is deleted after the save |

Any other `video_url` pointing at our storage is a validation error. Deleting an article
also deletes its owned video file, logged and not fatal, like the cover.

## Placement

The frontend renders the video in the middle of the text:

- paragraphs are the non-empty lines of the text;
- the first `ceil(n / 2)` paragraphs go before the video and the rest after it. For example,
  with 6 paragraphs the video comes after the 3rd, and with 5 paragraphs after the 3rd;
- with one paragraph the video comes after the text; with empty text only the video is shown;
- the original line breaks of each half are kept.

The backend does not interpret the text.

## HTTP API

- `NewsRequest` (JSON and multipart) gains `video_key` and `video_url`.
- `NewsResponse` (admin) and `PublicNewsDetailResponse` gain
  `video: {"kind": "file"|"embed", "url": "..."} | null`. List/feed responses are unchanged.
- The presign endpoint uses the default 1 MB body limit, and rate limiting is the same as
  other admin writes.

## Frontend

- `NewsEditorDialog`: a «Відео» block with a «Файл / Посилання» switch.
  - File: picker (mp4/webm, size checked client-side), upload progress (XHR), cancel,
    and remove.
  - Link: an input for a YouTube/Vimeo URL.
  - Saving is disabled while an upload is in progress.
- `Article.tsx` splits the text in the middle (`src/lib/video-placement.ts`):
  - `file` → `<video controls preload="metadata" poster={imageUrl}>`;
  - `embed` → responsive 16:9 `<iframe allowfullscreen>` with a title.

## Operations (documented in README)

- R2 bucket CORS: allow `PUT` from the admin origin with the `Content-Type` header.
- R2 lifecycle: delete objects with the prefix `pending/` after 1 day (shared with image uploads).
- The CSP of the site (if any) must allow `frame-src` for `www.youtube-nocookie.com`
  and `player.vimeo.com`, and `media-src` for the R2 public base URL.

## Testing

- Service:
  - presign validation (type, size, key format);
  - `video_key` checks (prefix, missing object, oversized, wrong signature, extension
    mismatch);
  - copy, save and cleanup ordering, plus failure paths;
  - embed URL normalization (table-driven);
  - update semantics table;
  - delete cleanup.
- r2: presign signs content type/length; `Inspect`/`Copy`/`Delete` against a fake S3 API.
- Repository integration: video columns round-trip and the consistency constraint.
- Handler: presign endpoint status codes; video fields in JSON and multipart requests;
  `video` in admin and public responses.
- Frontend: middle splitting, editor upload/link states.
