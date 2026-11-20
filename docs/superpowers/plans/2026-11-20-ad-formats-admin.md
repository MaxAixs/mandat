# Ad Formats Admin Implementation Plan

> **For agentic workers:** execute task-by-task with TDD. Tests are written by the
> `test-author` opencode agent (`opencode run --agent test-author "<brief>"`), the
> primary session writes production code, `reviewer` reviews at the end.

**Goal:** Manage the `/advertising` format cards (CRUD, image replace/remove, ↑/↓ order)
from the admin panel; the public page reads them from the API.

**Spec:** `docs/superpowers/specs/2026-11-20-ad-formats-admin-design.md`

**Tech Stack:** Go, Gin, pgx, goose, R2, testify · React 19, TypeScript, Tailwind, shadcn/ui, node:test.

## Global Constraints

- Clean Architecture; handlers thin; rules in the service; Gin/pgx never in models.
- Every service/repo function starts with `const fc = "<name>_<Func>"`, wraps with `%w`.
- Logging only in services (slog, `fc`); expected client errors are not logged.
- Errors reach clients only through `errResps` / `ValidationError`.
- Primary never edits `*_test.go`, `*.test.ts(x)`, `testdata/`.
- Repository integration tests truncate tables: run them only against `mandat_test`
  and only after the user's explicit permission.
- No commits/pushes without the user's command; backend and frontend are separate commits.

## Review Focus

- Replacing an image never deletes the live object before the new URL is saved.
- A failed create leaves neither a row nor an R2 object behind.
- `Move` on the first card «up» / last card «down» is a no-op, not an error.
- Swapping positions does not trip the unique constraint (deferred) and is atomic.
- Only keys under `ad-formats/` on our public base URL are ever deleted.
- Unknown id → 404, invalid uuid → 400, bad direction → 400, oversize image → 413.
- Moderator and admin can manage; anonymous gets 401.

## File Structure

Backend
- Create `migrations/00013_ad_formats.sql` (table + seed of 8 cards).
- Modify `internal/models` — new `ad_format.go`.
- Modify `internal/errs/errs.go` — `ErrAdFormatNotFound`; `internal/transport/http/errors.go` — table entry.
- Create `internal/repository/postgres/ad_format.go`.
- Create `internal/service/ad_format.go`.
- Create `internal/transport/http/dto/ad_format.go`.
- Create `internal/transport/http/handler/ad_format.go` (admin + public handlers).
- Modify `internal/transport/http/routes/routes.go`, `cmd/main.go`, `README.md`.

Frontend
- Create `src/api/adFormats.ts`; modify `src/types/index.ts`.
- Create `src/components/admin/AdFormatsPanel.tsx`, `src/components/admin/AdFormatEditorDialog.tsx`.
- Modify `src/pages/Admin.tsx` (category select item «Реклама»).
- Modify `src/pages/Advertising.tsx`, `src/components/AdvertisingFormatCard.tsx`,
  `src/constants/advertising.ts` (drop `ADVERTISING_FORMATS`).

## Tasks

### Task 1: Migration, model, not-found error
- [ ] test-author: errors table test — `ErrAdFormatNotFound` maps to 404 «Ad format not found».
- [ ] Write `00013_ad_formats.sql` per spec (deferred unique position, seed 8 rows).
- [ ] `models.AdFormat`, `errs.ErrAdFormatNotFound`, `errResps` entry.
- [ ] `go test ./internal/transport/http/...`, `go vet ./...`.

### Task 2: Repository
- [ ] test-author: integration tests in `ad_format_integration_test.go` (List order,
      Create appends position, Update, Delete/not found, Move up/down/no-op edges).
- [ ] Implement `AdFormatRepo` (List, GetByID, Create, Update, Delete, Move in tx).
- [ ] Ask the user's permission, then run integration tests against `mandat_test`.

### Task 3: Service
- [ ] test-author: unit tests with in-memory repo + fake storage: validation
      (title 1–80 trimmed, description ≤ 240, direction), create with/without image,
      upload failure removes the row, replace deletes old owned key after save,
      failed save deletes the new key, remove_image, delete removes owned key,
      foreign URLs never deleted, nil storage + image → ErrServiceUnavailable.
- [ ] Implement `AdFormatService` + `AdFormatInput{Title, Description, Image *CoverUpload, RemoveImage bool}`.

### Task 4: HTTP + wiring
- [ ] test-author: handler tests — multipart create/update, JSON move, 400/404/413,
      public list shape `{"items":[{id,title,description,image_url,position}]}`.
- [ ] DTO, `AdFormatHandler` (admin, auth + roles), `PublicAdFormatHandler`,
      route constants, `cmd/main.go`, README API table.
- [ ] `gofmt`, `go vet ./...`, `go test ./...`; rebuild container; curl smoke test.

### Task 5: Frontend API + types
- [ ] test-author: `tests/ad-formats-api.test.ts` with stubbed fetch — URLs, methods,
      FormData fields (`title`, `description`, `image`, `remove_image`), move payload, error message extraction.
- [ ] `AdFormat` type, `src/api/adFormats.ts`.

### Task 6: Admin UI
- [ ] test-author: admin tests — «Реклама» appears in the category select and is
      not part of `CATEGORIES`; selecting it renders the ad formats panel.
- [ ] `AdFormatsPanel`, `AdFormatEditorDialog`, Admin select integration.

### Task 7: Public page
- [ ] test-author: advertising page renders cards from the API; constant removed.
- [ ] `Advertising.tsx` loads via API, card uses `image_url`.
- [ ] `npm run lint`, `npm run build`, frontend tests; browser screenshot of admin and `/advertising`.

### Task 8: Review
- [ ] `opencode run --agent reviewer "<scope>"` → `.superpowers/reviews/2026-11-20-ad-formats-admin.txt`; fix findings.
- [ ] Final `go test ./...`, `go vet ./...`, frontend checks; report to the user.
