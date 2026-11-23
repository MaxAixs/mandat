# News Images Implementation Plan

> **For agentic workers:** execute task-by-task with TDD. Tests are written by the
> `test-author` opencode agent (`opencode run --agent test-author "<brief>"`), the
> primary session writes production code, `reviewer` reviews at the end.

**Goal:** Up to 10 extra images per article, added as files (direct upload to R2) or links
(downloaded by the backend). The public page spreads them through the text automatically.

**Spec:** `docs/superpowers/specs/2026-11-23-news-images-design.md`

**Builds on:** the uncommitted news video work (`docs/superpowers/plans/2026-11-23-news-video.md`).

**Tech Stack:** Go, Gin, pgx, goose, aws-sdk-go-v2, testify · React 19, TypeScript, Tailwind, node:test.

## Global Constraints

- All image rules (item kinds, limits, ownership, ordering) live in the service. Handlers
  only map fields.
- Every service/repo function starts with `const fc = "<name>_<Func>"`, wraps with `%w`,
  and logs only in services.
- All checks and downloads happen before any DB write (on create the row is inserted
  first because object keys need the final slug). After that the order is: store new
  objects → save → delete obsolete and pending objects. A failed save removes only the
  objects stored in this call.
- The backend never trusts client MIME types; it sniffs bytes.
- Primary never edits test files.
- The migration only adds a column and a check. Integration tests run against
  `mandat_test` only with the user's permission.
- No commits or pushes without the user's command.

## Review Focus

- A pending image key cannot reference anything outside `pending/images/<32 hex>.<image ext>`.
- A storage URL that is not one of the article's current images is rejected (no adopting
  other articles' files).
- Duplicates and more than 10 images are rejected before any storage call.
- External links are downloaded with the existing SSRF-guarded fetcher.
- Removed images are deleted only after a successful save, and all images are deleted on
  article delete.
- Video pending keys moved to `pending/videos/` everywhere (service, tests, README, spec).

## File Structure

Backend
- Create `migrations/00016_news_images.sql`.
- Modify `internal/models/news.go` (`ImageURLs`).
- Modify `internal/repository/postgres/news.go` (column, scan, insert, update).
- Modify `internal/service/news_video.go` (rename to `MediaStorage`/`WithMediaStorage`,
  pending prefix `pending/videos/`).
- Create `internal/service/news_images.go` (image constants, presign, prepare, copy/download, cleanup).
- Modify `internal/service/news.go` (prepare images, `saveWithMedia`, `discardMedia`,
  `releaseMedia`, `Delete`).
- Modify DTOs, `handler/news.go` (multipart `images`; `NewsHandler` also serves
  `POST /video-uploads` and `POST /image-uploads`), `cmd/main.go`, `README.md`, the video spec.

Frontend
- Modify `src/types/index.ts`, `src/api/news.ts` (image presign, reuse `uploadVideo` as a
  generic `uploadFile`), `src/api/index.ts` (map `images`).
- Rename `src/lib/video-placement.ts` → `src/lib/media-placement.ts` (images + video).
- Create `src/hooks/use-image-uploads.ts`, `src/components/admin/NewsImagesField.tsx`;
  modify `NewsEditorDialog.tsx`, `src/lib/news-validation.ts`.
- Create `src/components/ArticleImage.tsx` (img + full-screen viewer); modify `src/pages/Article.tsx`.

## Tasks

### Task 1: Migration, model, repository
- [ ] `00016_news_images.sql`, `News.ImageURLs`.
- [ ] test-author: integration tests — round-trip of 0/3 images in order through
      Create/Update/GetBySlug/List; the DB check rejects 11 items.
- [ ] Repository changes; ask permission, run against `mandat_test`.

### Task 2: Media storage rename and video pending prefix
- [ ] test-author: update video tests for `MediaStorage`/`WithMediaStorage` and
      `pending/videos/` keys.
- [ ] Rename and move the prefix; update README and the video spec.

### Task 3: Service — images
- [ ] test-author: `news_images_test.go` covering:
      - `PresignImageUpload` (types, size, key format, TTL, no storage);
      - Create and Update with pending, kept and external items: order preserved, keys
        `images/<slug>-<8hex>.<ext>`;
      - rejections (bad key, foreign storage URL, another article's image, duplicate,
        11 items, non-URL), without touching storage;
      - inspect failures (missing → 400, outage → 503, oversized or bad signature → 400
        and the pending object deleted);
      - fetch failures for links;
      - save failure removes the new copies and keeps the old ones;
      - images that drop out are deleted after the save, pending objects too;
      - Delete removes all owned images.
- [ ] Implement `news_images.go` and wire it into Create/Update/Delete.

### Task 4: HTTP + wiring
- [ ] test-author: handler tests:
      - `POST /api/admin/news/image-uploads` (200 shape, 400, 503, 401);
      - `images` from JSON and repeated multipart fields;
      - `images` array in admin and public detail responses (empty array, not null);
      - absent from public list items.
- [ ] DTOs, handler, routes, `cmd/main.go`, README. Run `gofmt`, `go vet`, `go test ./...`.

### Task 5: Frontend API, types, placement
- [ ] test-author: API tests (`requestImageUpload`, generic `uploadFile`, `images` in
      payloads, mapping in `getPublicNewsArticle`); `media-placement` tests (images only,
      with video, shared gap order, more images than gaps, empty text).
- [ ] Implement; remove `video-placement.ts` after moving its behavior.

### Task 6: Editor UI
- [ ] test-author: static tests for `NewsImagesField` covering:
      - empty state with picker and link input;
      - thumbnails in order with ↑/↓ and remove;
      - progress of an uploading item;
      - the limit «10 / 10» hides the picker;
      - per-item error;
      - validation helpers for files and the max count.
      Also dialog integration: the «ФОТО» section, existing images on edit.
- [ ] `use-image-uploads.ts`, `NewsImagesField.tsx`, dialog wiring, save disabled while uploading.

### Task 7: Article page
- [ ] test-author: images placed between paragraphs per the formula, video and image
      order at a shared gap, `<img loading="lazy" alt=title>`, viewer markup closed by
      default.
- [ ] `ArticleImage.tsx` + viewer, `Article.tsx`.
- [ ] `npm run lint`, `npm run build`, video and image suites.

### Task 8: Review
- [ ] `opencode run --agent reviewer` → `.superpowers/reviews/2026-11-23-news-images.txt`; fix findings.
- [ ] Rebuild and restart the local backend; final checks; report.
