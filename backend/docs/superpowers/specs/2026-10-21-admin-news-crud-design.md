# Design: Backend Bootstrap + Admin News CRUD

Date: 2026-10-21
Status: Approved by project owner (with goose as the migration tool)

## 1. Intent and Why

The Mandat news site currently renders from frontend mock data (`frontend/src/api/mock.ts`).
The frontend types are explicitly marked as mirroring the future Go backend.
This iteration bootstraps the Go backend and delivers the first real capability:
editors can create, list, read, update, and delete news articles through an admin API.

This is the foundation everything else (auth, images, AI, public feed) builds on,
so the iteration also establishes project scaffolding: configuration, dependency
wiring, error conventions, logging, and migrations.

## 2. Goals

- Go module initialized per the layout in `backend/AGENTS.md` §2.
- Configuration loaded from environment variables; `.env.example` documents every variable.
- Application startup in `cmd/main.go` wires dependencies with explicit constructors.
- Domain model `News` aligned with the frontend `Article` type.
- Repository layer over PostgreSQL using `pgx/v5` (`pgxpool`).
- Schema migrations managed with **goose** (owner's decision).
- Service layer with business logic, function-context (`fc`) constants, `log/slog` logging.
- HTTP transport with Gin: thin handlers, explicit DTOs, centralized error-to-HTTP mapping.
- Admin CRUD endpoints (see §7).
- Consistent JSON error responses per `backend/AGENTS.md` §5.
- Dockerfile + docker-compose for local development (app + PostgreSQL).
- Tests per `backend/AGENTS.md` §14: service unit tests, repository integration tests,
  handler tests.

## 3. Non-Goals (this iteration)

- Admin authentication/authorization (explicitly deferred by the owner; routes stay
  unprotected for now and auth middleware will be added as a separate task).
- S3 image upload (the model keeps a plain `cover_image` string field).
- AI integration.
- Public (reader-facing) endpoints.
- Comments system (frontend shows a preformatted comments string; no backend support yet).
- Uber FX (explicit constructors while the app is small, per `backend/AGENTS.md` §12).
- Kubernetes assets.

## 4. Anti-Goals

- No speculative interfaces or abstractions beyond what the layers need.
- No exposing drafts through any public route (there are no public routes yet).
- No soft delete "just in case".
- No ORM.

## 5. Constraints and Conventions

All conventions from `backend/AGENTS.md` apply. The ones that shape this design:

- Clean Architecture: domain does not depend on Gin, pgx, or any SDK.
- Services depend on repository interfaces defined in the service layer (consumer-side).
- Every service/repository function starts with `const fc = "<name>_<func>"` and wraps
  errors with `%w`; repositories never log; services log failures with `slog`.
- Centralized application errors in `internal/errs/errs.go` with sentinel errors.
- Handlers stay thin; error classification → HTTP status mapping happens in one place.
- Function parameter formatting rule (>4 params → one per line).
- Migration files are plain SQL, goose format (`-- +goose Up` / `-- +goose Down`).

## 6. Domain Model

`internal/models/news.go`:

| Field | Type | Notes |
|---|---|---|
| ID | int64 | internal primary key |
| Slug | string | unique, URL-safe; auto-generated from title, overridable |
| Category | string | one of the fixed rubrics (see below) |
| Title | string | required, max length 200 (constant) |
| Excerpt | string | short summary, max length 500 (constant) |
| Content | string | full article body |
| Language | string | ISO code, default `uk` |
| Status | string | `draft` or `published` |
| Views | int64 | counter, default 0 |
| Featured | bool | renders as large card on the site |
| CoverImage | string | URL/reference; plain string until S3 lands |
| PublishedAt | *time.Time | set on first transition to `published` |
| CreatedAt, UpdatedAt | time.Time | maintained by the database |

Categories (must match the frontend union type exactly):
`Новини`, `Аналітика`, `Інсайди`, `Важливо`, `Розслідування`.

`internal/models/errors.go` and `internal/errs/errs.go` hold domain/application errors
per `backend/AGENTS.md` §5 (at minimum: `ErrNewsNotFound`, `ErrInvalidInput`, `ErrConflict`
for duplicate slugs).

## 7. Admin API

Base path: `/api/v1/admin/news`. All routes unprotected in this iteration (auth later).

| Method & Path | Purpose | Success | Errors |
|---|---|---|---|
| `POST /api/v1/admin/news` | Create article | 201 + article JSON | 400 validation, 409 duplicate slug |
| `GET /api/v1/admin/news?page=&limit=` | Paginated list, newest first; optional `status` filter | 200 + `{items, total, page, limit}` | 400 bad pagination |
| `GET /api/v1/admin/news/:id` | Get one article | 200 + article JSON | 404 |
| `PUT /api/v1/admin/news/:id` | Full update (all editable fields in the DTO) | 200 + updated article | 400, 404, 409 |
| `DELETE /api/v1/admin/news/:id` | Hard delete | 204 | 404 |

Decisions made (builder may not change without asking):

- Update is `PUT` with the full editable DTO (idempotent, matches an editor form). *(agent's call)*
- Editable via create/update DTO: `slug` (optional on create — generated from title when
  absent), `category`, `title`, `excerpt`, `content`, `language`, `featured`,
  `cover_image`, `status`. System-owned (never accepted from the client): `id`, `views`,
  `published_at`, `created_at`, `updated_at`. *(agent's call)*
- Validation matrix: `title` (1–200 chars) and `category` (fixed rubric set) are always
  required; `content` is required when `status=published` (a draft may be title-only);
  `excerpt` (max 500) and `cover_image` are optional. *(agent's call)*
- Delete is **hard delete**. *(agent's call, owner approved the overall design)*
- Pagination: `page` starts at 1, `limit` default 20, max 100 (constants). *(agent's call)*
- List ordering: `created_at DESC, id DESC` — deterministic, and drafts without
  `published_at` don't break the order. *(agent's call)*
- `published_at` is strictly system-managed: set by the service when status first becomes
  `published`; reverting to `draft` keeps it; no client-supplied override in this
  iteration (YAGNI — backdating comes when the admin UI needs it). *(agent's call)*
- DTOs serialize raw values: `id` and `views` as JSON numbers, timestamps as RFC 3339.
  The frontend's preformatted strings are mock artifacts — its own types say
  "the backend will send a number later". *(answered by the frontend types)*
- Admin endpoints address articles by numeric `id`; `slug` is for public URLs later.

Error response shape (all non-2xx):

```json
{ "error": { "code": "NEWS_NOT_FOUND", "message": "News article not found" } }
```

## 8. Database

PostgreSQL 16. First migration `00001_create_news.sql` (goose):

- `news` table matching §6 with:
  - `id BIGSERIAL PRIMARY KEY`
  - `slug TEXT NOT NULL UNIQUE`
  - `category TEXT NOT NULL CHECK (category IN (...rubrics...))`
  - `status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published'))`
  - `language TEXT NOT NULL DEFAULT 'uk'`
  - `views BIGINT NOT NULL DEFAULT 0`, `featured BOOLEAN NOT NULL DEFAULT FALSE`
  - `published_at TIMESTAMPTZ NULL`, `created_at`/`updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- Index on `(status, published_at DESC)` for the future public feed.

Migrations are embedded into the binary (`embed.FS` exported by the `migrations` package)
and run at application startup via goose's Go API (`goose.Up`) — no external migration
tooling needed to boot the app. *(agent's call: simplest for local dev and single-instance
deploys; revisit if multi-instance deployment appears)*

Access through `pgx/v5` `pgxpool`, queries with parameterized SQL only.
Connection via `DATABASE_URL`. Pool sizing left to pgx defaults for now.

## 9. Slug Generation

Service generates a slug from the title on create when the client did not supply one:
Cyrillic transliteration (uk) → lowercase latin, non-alphanumerics → `-`, trimmed,
truncated at 80 chars. On unique-conflict, append `-2`, `-3`, ... until free.
If the client supplies a slug, validate format (`^[a-z0-9-]+$`) and let the database
unique constraint produce `ErrConflict`.

## 10. Configuration

Environment variables (all documented in `.env.example`):

- `HTTP_PORT` (default `8080`)
- `DATABASE_URL` (required, no default)
- `LOG_LEVEL` (default `info`)
- `ALLOWED_ORIGINS` (comma-separated CORS origins; `.env.example` ships
  `http://localhost:5173` for the Vite dev server)

CORS is in scope for this iteration: `backend/AGENTS.md` §11 mandates explicit CORS
configuration. A CORS middleware reads `ALLOWED_ORIGINS`; no wildcard in any
committed default. *(project requirement, settled by the agent)*

No secrets in the repo. `internal/config` validates required variables at startup and
fails fast with a clear message.

## 11. Package Layout (this iteration)

```text
cmd/main.go
internal/
  config/config.go
  errs/errs.go
  models/news.go
  service/news_service.go        (+ news_service_test.go)
  repository/postgres/news_repository.go  (+ integration test)
  transport/http/
    handler/news_handler.go      (+ handler test)
    middleware/                  (empty until auth/logging middleware is needed)
    dto/news_dto.go
    router.go
pkg/database/postgres/postgres.go
migrations/00001_create_news.sql
Dockerfile
docker-compose.yml
.env.example
```

`pkg/database/postgres` wraps pool construction (reusable); everything else lives in
`internal/`.

## 12. Testing

- Service unit tests: repository mocked behind the consumer-side interface; cover
  create (slug generation, validation, duplicate slug), get, list (pagination bounds),
  update (incl. status transition setting `published_at`), delete, not-found paths.
- Repository integration tests: against real PostgreSQL from docker-compose
  (skip when `DATABASE_URL` is unset, documented in the test).
- Handler tests: Gin in test mode with a stubbed service; cover status codes,
  error JSON shape, validation errors, pagination validation.
- `gofmt`, `go vet`, `go test ./...` must pass before the task is considered done.

## 13. Docker

- `docker-compose.yml`: `postgres:16` service with a named volume, healthcheck, and
  credentials from `.env`; the Go app runs locally via `go run` during development.
- Multi-stage `Dockerfile` for the app (non-root runtime user) — validated by building
  the image, not necessarily by running the app container in this iteration.

## 14. What's Left to the Builder

- Exact DTO field names in JSON (snake_case) and validation library choice
  (Gin's binding tags are acceptable).
- Transliteration table details for slug generation.
- Whether integration tests use docker-compose or testcontainers (plain
  docker-compose + skip-if-no-DB is the default stated above).
- Exact slog handler setup (JSON handler is the default).
