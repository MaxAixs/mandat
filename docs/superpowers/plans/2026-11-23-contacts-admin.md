# Contacts Admin Implementation Plan

> **For agentic workers:** execute task-by-task with TDD. Tests are written by the
> `test-author` opencode agent (`opencode run --agent test-author "<brief>"`), the
> primary session writes production code, `reviewer` reviews at the end.

**Goal:** Manage the `/contacts` page contacts (CRUD, ↑/↓ order, typed values with
derived icon and link) from the admin panel; the public page reads them from the API.

**Spec:** `docs/superpowers/specs/2026-11-23-contacts-admin-design.md`

**Tech Stack:** Go, Gin, pgx, goose, testify · React 19, TypeScript, Tailwind, shadcn/ui, node:test.

## Global Constraints

- Clean Architecture; handlers thin; validation, normalization and href building in the service.
- Every service/repo function starts with `const fc = "<name>_<Func>"`, wraps with `%w`.
- Logging only in services (slog, `fc`); expected client errors are not logged.
- Errors reach clients only through `errResps` / `ValidationError`.
- Primary never edits `*_test.go`, `*.test.ts(x)`, `testdata/`.
- Repository integration tests truncate tables: run them only against `mandat_test`
  and only after the user's explicit permission.
- The migration only creates and seeds `contacts`; it touches no existing data.
- No commits/pushes without the user's command; backend and frontend are separate commits.

## Review Focus

- Every type rejects values of other types (no `javascript:` or foreign hosts in `href`).
- URL forms are normalized to the bare handle; `href` is always built by the server.
- `Move` on the first «up» / last «down» is a no-op; swaps are atomic (deferred unique).
- Unknown id → 404, invalid uuid → 400, bad type/value/direction → 400.
- Moderator and admin can manage; anonymous gets 401.

## File Structure

Backend
- Create `migrations/00014_contacts.sql`.
- Create `internal/models/contact.go`.
- Modify `internal/errs/errs.go`, `internal/transport/http/errors.go`.
- Create `internal/repository/postgres/contact.go`.
- Create `internal/service/contact.go`.
- Create `internal/transport/http/dto/contact.go`.
- Create `internal/transport/http/handler/contact.go` (admin routes and the public list in one handler).
- Modify `internal/transport/http/routes/routes.go`, `cmd/main.go`, `README.md`.

Frontend
- Modify `src/types/index.ts`; create `src/api/contacts.ts`.
- Create `src/components/admin/ContactsPanel.tsx`, `src/components/admin/ContactEditorDialog.tsx`.
- Modify `src/pages/Admin.tsx` (section «Контакти»).
- Modify `src/pages/Contacts.tsx`, `src/constants/contacts.ts` (drop `CONTACTS`).

## Tasks

### Task 1: Migration, model, not-found error
- [ ] `00014_contacts.sql` per spec (seed email + telegram), `models.Contact`, error + table entry.
- [ ] `go test ./internal/transport/http/...`, `go vet ./...`.

### Task 2: Repository
- [ ] test-author: `contact_integration_test.go` — List order, Create appends, Update,
      Delete/not found, Move up/down/no-op edges.
- [ ] Implement `ContactRepo` (List, Create, Update, Delete, Move in tx).
- [ ] Ask the user's permission, then run integration tests against `mandat_test`.

### Task 3: Service
- [ ] test-author: unit tests with an in-memory repo — per-type validation and
      normalization (table in spec, incl. URL forms and foreign hosts rejected),
      label trim/limit, display/href per type, unknown type, move direction, not found.
- [ ] Implement `ContactService` + `ContactInput{Type, Value, Label}`.

### Task 4: HTTP + wiring
- [ ] test-author: handler tests — JSON create/update, move, 400/404, auth,
      public list shape `{"items":[{id,type,value,label,display,href,position}]}`.
- [ ] DTO, `ContactHandler` (admin routes + public `contactPublicList`), routes, `cmd/main.go`, README.
- [ ] `gofmt`, `go vet ./...`, `go test ./...`; rebuild/restart the backend; curl smoke test.

### Task 5: Frontend API + types
- [ ] test-author: `tests/contacts-api.test.ts` with stubbed fetch — URLs, methods,
      JSON bodies, move payload, error message extraction.
- [ ] Types and `src/api/contacts.ts`.

### Task 6: Admin UI
- [ ] test-author: admin tests — «Контакти» section renders the panel; editor dialog
      submits type/value/label and shows server errors.
- [ ] `ContactsPanel`, `ContactEditorDialog`, Admin section integration.

### Task 7: Public page
- [ ] test-author: `/contacts` renders items from the API (label or type name, display, href, icon).
- [ ] `Contacts.tsx` loads via API; drop the constant.
- [ ] `npm run lint`, `npm run build`, frontend tests; screenshots of admin and `/contacts`.

### Task 8: Review
- [ ] `opencode run --agent reviewer "<scope>"` → `.superpowers/reviews/2026-11-23-contacts-admin.txt`; fix findings.
- [ ] Final `go test ./...`, `go vet ./...`, frontend checks; report to the user.
