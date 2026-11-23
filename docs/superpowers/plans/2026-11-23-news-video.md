# News Video Implementation Plan

> **For agentic workers:** execute task-by-task with TDD. Tests are written by the
> `test-author` opencode agent (`opencode run --agent test-author "<brief>"`), the
> primary session writes production code, `reviewer` reviews at the end.

**Goal:** One video per news article: an mp4/webm file (≤ 500 MB) uploaded straight to
R2 through a presigned PUT, or a YouTube/Vimeo link. It is rendered in the middle of
the article text.

**Spec:** `docs/superpowers/specs/2026-11-23-news-video-design.md`

**Tech Stack:** Go, Gin, pgx, goose, aws-sdk-go-v2 (s3 + PresignClient), testify ·
React 19, TypeScript, Tailwind, shadcn/ui, node:test.

## Global Constraints

- Clean Architecture: handlers are thin, and all video rules (key format, sniffing,
  embed normalization, update semantics) live in the service.
- Every service/repo function starts with `const fc = "<name>_<Func>"` and wraps with `%w`.
  Logging happens only in services.
- Client-facing errors go only through `errResps` / `ValidationError`. Storage outages map
  to `ErrServiceUnavailable`.
- The backend never reads video bodies beyond the first 512 bytes.
- Primary never edits `*_test.go`, `*.test.ts(x)`, `testdata/`.
- The migration only adds columns and a check; it touches no existing data.
- Repository integration tests truncate tables. Run them only against `mandat_test`
  and only after the user's explicit permission.
- No commits or pushes without the user's command; backend and frontend are separate commits.

## Review Focus

- `video_key` can never reference anything outside `videos/pending/<32 hex>.<mp4|webm>`.
- Content is sniffed from bytes; a mismatched extension, size > 500 MB, or a missing object
  each give 400.
- Ordering on create/update: copy → save → delete old/pending. A failed save removes only
  the new copy, and a failed create removes the half-created row.
- Embed: only YouTube/Vimeo ids are accepted and stored in canonical embed form; no other
  host can reach an `<iframe src>`.
- A foreign storage URL in `video_url` is rejected. The current file URL keeps the video.
- Presigned URL signs content type and length and expires in 15 minutes; the endpoint is
  admin-only.
- Public detail exposes `video`; list/feed responses do not.

## File Structure

Backend
- Create `migrations/00015_news_video.sql`.
- Modify `internal/models/news.go` (`VideoKind`, `Video`, `News.Video`).
- Modify `internal/repository/postgres/news.go` (columns, scan, insert, update).
- Create `pkg/videotype/videotype.go` (sniff mp4/webm, ext↔MIME).
- Modify `internal/storage/r2/r2.go` (S3API gains Head/Get/Copy; `PresignPut`, `Inspect`, `Copy`).
- Create `internal/service/news_video.go` (VideoStorage interface, constants, presign,
  prepare, embed normalize, marker check, attach/replace helpers).
- Modify `internal/service/news.go`, `internal/service/news_validation.go`.
- Modify `internal/transport/http/dto/request.go`, `dto/response.go`.
- Modify `internal/transport/http/handler/news.go` (multipart fields, `POST /video-uploads`).
- Modify `cmd/main.go`, `README.md`.

Frontend
- Modify `src/types/index.ts`, `src/api/news.ts` (video types, presign + XHR upload).
- Create `src/lib/video-marker.ts` (split text by marker).
- Create `src/components/admin/NewsVideoField.tsx`; modify `NewsEditorDialog.tsx`.
- Create `src/components/ArticleVideo.tsx`; modify `src/pages/Article.tsx`.

## Tasks

### Task 1: Migration and model
- [ ] `00015_news_video.sql` per spec; `models.VideoKind` (`file`/`embed`, `Valid`), `models.Video`, `News.Video *Video`.
- [ ] `go build ./...`, `go vet ./...`.

### Task 2: Repository
- [ ] test-author: extend `news_integration_test.go`. Cover the round-trip of file and
      embed video and nil video on Create/Update/GetBySlug/List, and assert that the DB
      check rejects kind without url.
- [ ] Add `video_kind`, `video_url` to `newsColumns`, scan (NULL kind → nil Video),
      insert and update.
- [ ] Ask the user's permission, then run integration tests against `mandat_test`.

### Task 3: videotype
- [ ] test-author: `pkg/videotype` tests. mp4 (`ftyp` at offset 4) and webm (EBML magic)
      are detected; images, text and short input are not. Cover `ExtForMIME`/`MIMEForExt`
      for mp4/webm and rejection of others.
- [ ] Implement `Sniff`, `ExtForMIME`, `MIMEForExt`.

### Task 4: R2 video operations
- [ ] test-author: `r2_video_test.go` with fake S3 API + presigner.
      - `PresignPut` passes bucket, key, content type and length, and the TTL.
      - `Inspect` returns size from Head and the first 512 bytes via a ranged Get;
        NotFound maps to `ErrNotFound`-style validation.
      - `Copy` uses `bucket/src` as CopySource and returns the public URL.
      - Errors are wrapped.
- [ ] Extend `S3API`; add a `Presigner` interface (real one is `s3.NewPresignClient`);
      `NewClient` builds both when nil.

### Task 5: Service — video rules
- [ ] test-author: `news_video_test.go` (fake VideoStorage, in-memory repo).
      - `PresignVideoUpload`: type, size bounds, key format.
      - Embed normalization: the table from the spec, plus rejected hosts and ids.
      - Marker: 0/1 ok, 2+ → 400.
      - Create with `video_key`: wrong prefix or format, missing object, oversized,
        bad signature, ext mismatch → 400. Success copies to
        `videos/<slug>-<8hex>.<ext>`, saves, and deletes pending. Save failure deletes the
        copy and the row.
      - Update semantics table from the spec (keep / replace / embed / clear; foreign
        storage URL → 400; old file deleted only after the save).
      - Delete removes the owned video.
      - No storage configured + `video_key` → `ErrServiceUnavailable`.
- [ ] Implement in `news_video.go`. Wire `VideoStorage` into `NewNewsService`. Prepare the
      video before any write, together with the cover. After insert/update, attach the
      cover and the video in one `repo.Update`, then clean up.

### Task 6: HTTP + wiring
- [ ] test-author: handler tests.
      - `POST /api/admin/news/video-uploads`: 200 shape `{key, upload_url, expires_at}`,
        400 on bad input, 401 anonymous.
      - `video_key`/`video_url` reach the service from JSON and multipart.
      - `video` appears in admin and public detail responses (null when absent) and is
        absent from public list items.
- [ ] DTOs, handler route, `cmd/main.go`, README (R2 CORS, lifecycle rule, CSP note).
- [ ] `gofmt`, `go vet ./...`, `go test ./...`. Rebuild and restart the backend; smoke-test
      with curl (presign → PUT → create with `video_key` → GET).

### Task 7: Frontend API, types, marker
- [ ] test-author: `news-video-api.test.ts`: presign request, XHR PUT with progress and
      abort, `video_key`/`video_url` in payloads. `video-marker.test.ts`: split before,
      after and at the end; no marker → video appended; marker without video → hidden.
- [ ] Types (`NewsVideo`, form fields), `requestVideoUpload`, `uploadVideo`,
      `splitByVideoMarker`.

### Task 8: Editor UI
- [ ] test-author: `admin-news-video.test.tsx` covers:
      - the File/Link switch;
      - the size/type check before upload;
      - progress and cancel;
      - the «Вставити відео» button inserting `[video]` at the cursor;
      - save disabled during upload;
      - an existing video shown on edit;
      - removal clearing the fields.
- [ ] `NewsVideoField` + integration in `NewsEditorDialog`.

### Task 9: Article page
- [ ] test-author: `article-video.test.tsx` checks:
      - a `file` renders `<video controls>` with the cover as poster;
      - an `embed` renders an `<iframe>` with the canonical src;
      - the position follows the marker;
      - with no video, the marker text is not rendered.
- [ ] `ArticleVideo` + `Article.tsx`.
- [ ] `npm run lint`, `npm run build`, frontend tests; screenshots of the editor and an
      article with a video.

### Task 10: Review
- [ ] `opencode run --agent reviewer "<scope>"` → `.superpowers/reviews/2026-11-23-news-video.txt`; fix findings.
- [ ] Final `go test ./...`, `go vet ./...`, frontend checks; report to the user.

### Task 11: Simplify placement — video in the middle (user request after Task 10)
The `[video]` marker is dropped; the video always goes in the middle of the text
(see «Placement» in the spec). This supersedes the marker parts of Tasks 5, 7, 8 and 9.
- [ ] test-author: remove the marker tests (backend `VideoMarker` constant and marker
      count; frontend marker split/insert tests, the «Вставити відео» button and hint,
      the marker validation). Add `tests/video-placement.test.ts` for
      `splitForVideo` and update `article-video.test.tsx` to expect the middle placement.
- [ ] Remove `VideoMarker`/`countVideoMarkers` and the marker check in `validateNews`;
      replace `src/lib/video-marker.ts` with `src/lib/video-placement.ts`; drop the
      editor button/hint and the marker validation; `Article.tsx` uses `splitForVideo`.
- [ ] `go vet`, `go test ./...`, frontend lint/build/video suites; README without the marker.
