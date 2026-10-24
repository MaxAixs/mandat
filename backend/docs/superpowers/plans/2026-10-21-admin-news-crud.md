# Backend Bootstrap + Admin News CRUD — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bootstrap the Go backend and ship an admin CRUD API for news articles (create, list, get, update, hard-delete) over PostgreSQL.

**Architecture:** Clean Architecture per `backend/AGENTS.md`: domain models + sentinel errors at the core, service layer with business logic behind a consumer-side repository interface, pgx-based Postgres repository, thin Gin handlers with DTOs and centralized error mapping. Wiring via explicit constructors in `cmd/main.go`; goose migrations embedded in the binary and applied at startup.

**Tech Stack:** Go (match the locally installed toolchain), Gin, gin-contrib/cors, jackc/pgx/v5 (pgxpool + stdlib for goose), pressly/goose/v3, PostgreSQL 16, Docker Compose, testify for assertions.

**Spec:** `backend/docs/superpowers/specs/2026-10-21-admin-news-crud-design.md` — read it together with this plan.

## Global Constraints

From `backend/AGENTS.md` and the spec; every task implicitly includes these:

- Module path: `github.com/maxaixs/mandat/backend`. All work happens in `/home/aix/GolandProjects/mandat/backend`.
- Every service/repository function starts with `const fc = "<name>_<Func>"` and wraps errors via `fmt.Errorf("%s: ...: %w", fc, err)`.
- Repositories never log. Only services log (structured `log/slog`, include `fc`).
- Functions with more than 4 parameters: one parameter per line.
- Comments in English, 1–3 lines.
- Named constants for limits/statuses/timeouts; no magic numbers for business rules.
- Handlers stay thin: bind → call service → map result/error. No business logic in handlers.
- Client error responses always `{"error": {"code": "...", "message": "..."}}`; never leak internals.
- SQL: parameterized queries only.
- **Do NOT commit anything** — project git rules require an explicit owner command. If the owner commands commits, messages must start with `[+]` (new), `[*]` (change), or `[-]` (delete).
- New dependencies must be justified in one line when introduced (gin, gin-contrib/cors, pgx/v5, goose/v3, testify are pre-approved by the spec/plan).
- Verify with `gofmt -l .` (empty), `go vet ./...`, `go test ./...` at the end of each task.

## Review Focus

The spec implies these failure modes; each is pinned by a test in the owning task:

1. **UPDATE/DELETE of a nonexistent id must not silently succeed** — 0 rows affected → `errs.ErrNewsNotFound`. (Task 2, integration tests; Task 4, handler 404 tests.)
2. **A title that sanitizes to an empty slug** (emoji/punctuation-only) → `errs.ErrInvalidInput`, never an empty or `-`-only slug. (Task 3, service test.)
3. **Pagination abuse**: `page=0`, negative, non-numeric, `limit` above max → 400 or clamped to 100, never a huge DB scan. (Task 3 service tests; Task 4 handler tests.)
4. **`published_at` semantics on status flips**: set once on first publish; reverting to draft keeps it; republishing does not move it. (Task 3, service tests.)
5. **Hostile strings** (SQL metacharacters, quotes) in title/slug round-trip verbatim — parameterized queries, no sanitization of stored content. (Task 2, integration test.)

---

### Task 1: Module scaffold, config, domain model, errors

**Files:**
- Create: `go.mod` (via `go mod init github.com/maxaixs/mandat/backend`)
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`
- Create: `internal/errs/errs.go`
- Create: `internal/models/news.go`
- Test: `internal/models/news_test.go`
- Create: `.env.example`
- Modify: `../.gitignore` (root) — add `.env` if absent

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `config.Load(getenv func(string) string) (*config.Config, error)`; `Config{HTTPPort int, DatabaseURL string, LogLevel string, AllowedOrigins []string}`
  - `errs.ErrNewsNotFound`, `errs.ErrInvalidInput`, `errs.ErrConflict` (sentinel `errors.New`)
  - `models.News{ID int64; Slug string; Category Category; Title string; Excerpt string; Content string; Language string; Status Status; Views int64; Featured bool; CoverImage string; PublishedAt *time.Time; CreatedAt, UpdatedAt time.Time}`
  - `models.Status` (`StatusDraft = "draft"`, `StatusPublished = "published"`), `models.Category` with constants `CategoryNews = "Новини"`, `CategoryAnalytics = "Аналітика"`, `CategoryInsides = "Інсайди"`, `CategoryImportant = "Важливо"`, `CategoryInvestigation = "Розслідування"`, method `(Category) Valid() bool`
  - `models.ListFilter{Status *Status; Page int; Limit int}`
  - Constants: `models.TitleMaxLen = 200`, `models.ExcerptMaxLen = 500`, `models.SlugMaxLen = 80`, `models.DefaultLanguage = "uk"`

- [ ] **Step 1: Init module**

Run `go mod init github.com/maxaixs/mandat/backend` in `backend/`. Record the local `go version` — the Dockerfile in Task 5 must use the same major.minor.

- [ ] **Step 2: Write failing config tests**

`config_test.go` (testify): `Load` with only `DATABASE_URL` set → defaults (`HTTPPort=8080`, `LogLevel="info"`, `AllowedOrigins` from `ALLOWED_ORIGINS` split on comma, trimmed); missing `DATABASE_URL` → error mentioning the variable name; `HTTP_PORT=abc` → error; `ALLOWED_ORIGINS="http://a, http://b"` → `["http://a","http://b"]`. Test signature pins the `getenv` injection: `config.Load(func(key string) string { ... })`.

- [ ] **Step 3: Run tests, verify they fail** (`go test ./internal/config/`, undefined `config.Load`).

- [ ] **Step 4: Implement `internal/config/config.go`**

`Load` reads via the injected `getenv`, applies defaults, validates required `DATABASE_URL`, parses `HTTP_PORT` as int. Keep it dependency-free (stdlib only).

- [ ] **Step 5: Write failing models tests, then implement**

`news_test.go`: `Category("Новини").Valid() == true`, `Category("Спорт").Valid() == false`. Implement `internal/models/news.go` with the exact struct/types/constants from Interfaces; `internal/errs/errs.go` with the three sentinels.

- [ ] **Step 6: `.env.example` + `.gitignore`**

`.env.example`: `HTTP_PORT=8080`, `DATABASE_URL=postgres://mandat:mandat@localhost:5432/mandat?sslmode=disable`, `LOG_LEVEL=info`, `ALLOWED_ORIGINS=http://localhost:5173`. Ensure root `.gitignore` ignores `.env`.

- [ ] **Step 7: Verify** — `go test ./...` passes, `gofmt -l .` empty, `go vet ./...` clean.

---

### Task 2: Compose, migration, DB pool, news repository

**Files:**
- Create: `docker-compose.yml`
- Create: `migrations/00001_create_news.sql`
- Create: `migrations/migrations.go`
- Create: `pkg/database/postgres/postgres.go`
- Create: `internal/repository/postgres/news_repository.go`
- Test: `internal/repository/postgres/news_repository_integration_test.go`

**Interfaces:**
- Consumes: Task 1 models/errs.
- Produces:
  - `postgres.NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error)` (pings before returning)
  - `postgres.Migrate(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) error` (goose over `stdlib.OpenDBFromPool`)
  - `migrations.FS embed.FS`
  - `newsrepo.NewNewsRepo(pool *pgxpool.Pool) *newsrepo.NewsRepo` (`newsrepo` = import alias
    for `internal/repository/postgres`; package clause is `package postgres`) with methods
    (each starting with `const fc = "newsRepo_<Func>"`, no logging):
    - `Create(ctx context.Context, news *models.News) error` — INSERT, fills `ID`, `CreatedAt`, `UpdatedAt` via RETURNING
    - `GetByID(ctx context.Context, id int64) (*models.News, error)` — `pgx.ErrNoRows` → `errs.ErrNewsNotFound`
    - `List(ctx context.Context, filter models.ListFilter) ([]*models.News, int64, error)` — items + total count
    - `Update(ctx context.Context, news *models.News) error` — full row update by `news.ID`, refreshes `UpdatedAt`
    - `Delete(ctx context.Context, id int64) error`
  - Unique-violation (SQLSTATE `23505`) → `errs.ErrConflict`; `Update`/`Delete` with 0 rows affected → `errs.ErrNewsNotFound`. Later tasks rely on exactly this mapping.

- [ ] **Step 1: `docker-compose.yml`**

Service `db`: `postgres:16`, env `POSTGRES_USER=mandat`, `POSTGRES_PASSWORD=mandat`, `POSTGRES_DB=mandat`, port `5432:5432`, named volume `pgdata`, healthcheck `pg_isready -U mandat`. (Dev-only credentials; they never leave the local machine.)

- [ ] **Step 2: Migration `00001_create_news.sql`**

goose format (`-- +goose Up` / `-- +goose Down`). Table `news` per spec §8: `id BIGSERIAL PRIMARY KEY`, `slug TEXT NOT NULL UNIQUE`, `category TEXT NOT NULL CHECK (category IN ('Новини','Аналітика','Інсайди','Важливо','Розслідування'))`, `title TEXT NOT NULL`, `excerpt TEXT NOT NULL DEFAULT ''`, `content TEXT NOT NULL DEFAULT ''`, `language TEXT NOT NULL DEFAULT 'uk'`, `status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published'))`, `views BIGINT NOT NULL DEFAULT 0`, `featured BOOLEAN NOT NULL DEFAULT FALSE`, `cover_image TEXT NOT NULL DEFAULT ''`, `published_at TIMESTAMPTZ NULL`, `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`, `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`; index `idx_news_status_published_at ON news (status, published_at DESC)`. Down: drop index + table. Add `migrations/migrations.go`: `package migrations`, `//go:embed *.sql`, `var FS embed.FS`.

- [ ] **Step 3: `pkg/database/postgres/postgres.go`**

`NewPool` (pgxpool + `Ping(ctx)`), `Migrate` (goose v3: `goose.SetBaseFS(fsys)`, dialect `postgres`, `goose.UpContext(ctx, stdlib.OpenDBFromPool(pool), ".")`). Add deps: `go get github.com/jackc/pgx/v5 github.com/pressly/goose/v3` (pgx = DB driver per spec; goose = owner's chosen migration tool).

- [ ] **Step 4: Write failing repository integration tests**

Guard: `databaseURL := os.Getenv("DATABASE_URL"); if databaseURL == "" { t.Skip(...) }`. `TestMain` (or helper) creates pool, runs `Migrate`, truncates `news` before each test. Tests:
- `TestNewsRepo_CreateAndGetByID` — round-trip all fields incl. Cyrillic title.
- `TestNewsRepo_Create_DuplicateSlug` — second insert with same slug → `errors.Is(err, errs.ErrConflict)`.
- `TestNewsRepo_Create_HostileStrings` — title `'); DROP TABLE news;--` and slug with quotes round-trip verbatim (Review Focus #5).
- `TestNewsRepo_GetByID_NotFound` → `errors.Is(err, errs.ErrNewsNotFound)`.
- `TestNewsRepo_List_PaginationAndFilter` — seed 5 (3 published, 2 draft), page 1 limit 2 → 2 items + total 5, ordered `created_at DESC, id DESC`; `Status: &published` → 3 items.
- `TestNewsRepo_Update` — changes fields, `UpdatedAt` advances, `CreatedAt` unchanged.
- `TestNewsRepo_Update_NotFound` / `TestNewsRepo_Delete_NotFound` → `ErrNewsNotFound` (Review Focus #1).
- `TestNewsRepo_Delete` — gone afterwards (`GetByID` → `ErrNewsNotFound`).

- [ ] **Step 5: Run tests — fail** (repo doesn't exist). Without DB: `docker compose up -d db`, export `DATABASE_URL=postgres://mandat:mandat@localhost:5432/mandat?sslmode=disable`.

- [ ] **Step 6: Implement `internal/repository/postgres/news_repository.go`**

Package name `postgres` (directory `internal/repository/postgres`); importers alias it as
`newsrepo` to avoid colliding with `pkg/database/postgres`. Parameterized `$1..$N` queries. `List` runs `COUNT(*)` + SELECT with `LIMIT/OFFSET`, optional `WHERE status = $1`, `ORDER BY created_at DESC, id DESC`. Update sets `updated_at = now()`. Detect 23505 via `pgconn.PgError`.

- [ ] **Step 7: Run tests — pass.** `gofmt -l .` empty, `go vet ./...` clean.

---

### Task 3: News service with slug generation

**Files:**
- Create: `internal/service/news_service.go`
- Create: `internal/service/slug.go`
- Test: `internal/service/news_service_test.go` (includes the stub repo)
- Test: `internal/service/slug_test.go`

**Interfaces:**
- Consumes: Task 1 models/errs; Task 2's error-mapping contract (`ErrConflict` on duplicate slug, `ErrNewsNotFound`).
- Produces (Task 4 relies on these exact signatures):
  - `service.NewsInput{Slug, Category, Title, Excerpt, Content, Language, Status, Featured, CoverImage string/bool}` (strings for Category/Status; service validates)
  - `service.ListResult{Items []*models.News; Total int64; Page int; Limit int}`
  - `service.NewsRepository` interface: `Create(context.Context, *models.News) error`, `GetByID(context.Context, int64) (*models.News, error)`, `List(context.Context, models.ListFilter) ([]*models.News, int64, error)`, `Update(context.Context, *models.News) error`, `Delete(context.Context, int64) error`
  - `service.NewNewsService(repo NewsRepository, logger *slog.Logger) *NewsService`
  - Methods (each with `const fc = "newsService_<Func>"`): `Create(ctx, input NewsInput) (*models.News, error)`, `GetByID(ctx, id int64)`, `List(ctx, page, limit int, status string) (*ListResult, error)`, `Update(ctx, id int64, input NewsInput) (*models.News, error)`, `Delete(ctx, id int64) error`
  - Constants: `service.DefaultPageLimit = 20`, `service.MaxPageLimit = 100`, `maxSlugAttempts = 10` (unexported)

- [ ] **Step 1: Write failing slug tests**

`slug_test.go` for `slugify(title string) string`: `"Уряд схвалив новий бюджет на 2026 рік"` → a lowercase latin dashed slug (assert exact expected string once the transliteration table is written — write the table first in the test's mind: uk Cyrillic → latin, e.g. `у→u`, `р→r`, `я→ia`/`ya` — pin the table in the implementation and assert 2–3 exact cases); spaces/punctuation collapse to single `-`; truncated at `models.SlugMaxLen`; `"🔥🔥🔥"` → `""`.

- [ ] **Step 2: Implement `slug.go`** — run tests pass.

- [ ] **Step 3: Write failing service tests**

In-memory stub repo (map-backed, programmable errors) in `news_service_test.go`. Cover:
- `Create` auto-generates slug from a Cyrillic title; empty `Language` → `models.DefaultLanguage`; empty `Status` → `StatusDraft`.
- `Create` with slug that sanitizes to empty (title `"🔥🔥🔥"`, no client slug) → `errors.Is(err, errs.ErrInvalidInput)` (Review Focus #2).
- `Create` with client slug failing `^[a-z0-9-]+$` → `ErrInvalidInput`; valid client slug used as-is.
- `Create` on stub-conflict retries with `-2`, `-3` suffixes (auto slug only); client-supplied slug conflict → `errs.ErrConflict` propagated without retry.
- Validation matrix (spec §7): missing title / title > 200 / unknown category / unknown status / excerpt > 500 → `ErrInvalidInput`; `Status=published` with empty content → `ErrInvalidInput`; draft without content → OK.
- `Create`/`Update` with `Status=published` when `PublishedAt == nil` → service sets it to now; already-set `PublishedAt` never overwritten (republish, or revert-to-draft keeps it — Review Focus #4).
- `Update` loads by id first: not found → `ErrNewsNotFound` propagated.
- `List`: `page < 1` or `limit < 1` → `ErrInvalidInput`; `limit > MaxPageLimit` clamped to 100 (Review Focus #3); empty status string → no filter; unknown status string → `ErrInvalidInput`.
- `Delete` propagates `ErrNewsNotFound`.
- Logging: on stubbed repo error, `slog` records an entry containing the `fc` value (use a test `slog.Handler`).

- [ ] **Step 4: Run tests — fail.**

- [ ] **Step 5: Implement `news_service.go`**

Validation helper per input; slug flow: client slug (validated) or `slugify(title)`; on `ErrConflict` with auto slug, append `-N` up to `maxSlugAttempts`. `Update` = load → overwrite editable fields → re-validate → repo.Update. Log failures via `logger.ErrorContext` with `fc`, per AGENTS.md §6 example.

- [ ] **Step 6: Run tests — pass.** `gofmt`, `go vet` clean.

---

### Task 4: HTTP transport (DTOs, handlers, error mapping, router, CORS)

**Files:**
- Create: `internal/transport/http/dto/news_dto.go`
- Create: `internal/transport/http/handler/news_handler.go`
- Create: `internal/transport/http/handler/errors.go`
- Create: `internal/transport/http/router.go`
- Test: `internal/transport/http/handler/news_handler_test.go`

**Interfaces:**
- Consumes: `*service.NewsService` (concrete — handler tests use a real service over a stub repo; no extra interface), Task 1–3 everything.
- Produces (Task 5 wires these):
  - `dto.NewsRequest{Slug string json:"slug"; Category string json:"category" binding:"required"; Title string json:"title" binding:"required"; Excerpt string json:"excerpt"; Content string json:"content"; Language string json:"language"; Status string json:"status"; Featured bool json:"featured"; CoverImage string json:"cover_image"}`
  - `dto.NewsResponse` — all model fields, snake_case JSON, `id`/`views` as numbers, timestamps RFC 3339 (default `time.Time` encoding); `dto.NewsToResponse(*models.News) *NewsResponse`
  - `dto.ListResponse{Items []*NewsResponse json:"items"; Total int64 json:"total"; Page int json:"page"; Limit int json:"limit"}`
  - `handler.NewNewsHandler(svc *service.NewsService) *NewsHandler` with `Create`, `List`, `GetByID`, `Update`, `Delete` gin handlers
  - `handler.WriteError(c *gin.Context, err error)` — `ErrNewsNotFound`→404 `NEWS_NOT_FOUND`, `ErrInvalidInput`→400 `INVALID_INPUT`, `ErrConflict`→409 `CONFLICT`, else 500 `INTERNAL_ERROR` with fixed generic messages
  - `httptransport.NewRouter(newsHandler *handler.NewsHandler, allowedOrigins []string) *gin.Engine` — package name `httptransport` (directory `internal/transport/http`)

- [ ] **Step 1: Write failing handler tests**

gin test mode + `httptest`, real `service.NewsService` over a stub repo (reuse the stub pattern from Task 3; a shared stub in `internal/service` test files is not importable — copy a minimal one into this test file). Route via `NewRouter(h, []string{"http://localhost:5173"})`. Cover:
- `POST /api/v1/admin/news` valid body → 201, response body has numeric `id`, `"status":"draft"`, RFC 3339 `created_at`.
- POST malformed JSON → 400 `INVALID_INPUT`; POST without title → 400; error body shape exactly `{"error":{"code":"...","message":"..."}}`.
- POST with stub repo returning `ErrConflict` → 409 `CONFLICT`.
- `GET /api/v1/admin/news` → 200 with `{items,total,page,limit}` envelope; `?page=abc` and `?page=0` → 400 (Review Focus #3); `?limit=1000000` → 200 with `limit:100` clamped.
- `GET /api/v1/admin/news/1` → 200; unknown id → 404 `NEWS_NOT_FOUND`; non-numeric id → 400.
- `PUT /api/v1/admin/news/1` full DTO → 200 with updated fields; unknown id → 404 (Review Focus #1).
- `DELETE /api/v1/admin/news/1` → 204; unknown id → 404.
- CORS: request with `Origin: http://localhost:5173` → `Access-Control-Allow-Origin` echoes it; preflight `OPTIONS` → 204; disallowed origin → no CORS header.

- [ ] **Step 2: Run tests — fail.**

- [ ] **Step 3: Implement DTOs, `errors.go`, handlers, router**

Handlers: bind JSON (`ShouldBindJSON` failure → `WriteError` with `ErrInvalidInput`), parse path id (`strconv.ParseInt`, failure → 400), call service, map via `WriteError` / `dto.NewsToResponse`. `List` parses `page`/`limit` query params (default 1 / `service.DefaultPageLimit`) and optional `status`. Router: `gin.New()` + `gin.Recovery()`, CORS via `gin-contrib/cors` with the exact `allowedOrigins`, routes under `/api/v1/admin/news`. Add deps: `go get github.com/gin-gonic/gin github.com/gin-contrib/cors` (Gin = spec-mandated framework; gin-contrib/cors = spec §11 explicit CORS requirement).

- [ ] **Step 4: Run tests — pass.** `gofmt`, `go vet` clean.

---

### Task 5: Application wiring, Dockerfile, docs

**Files:**
- Create: `cmd/main.go`
- Create: `Dockerfile`
- Create: `.dockerignore`
- Create: `README.md` (backend-level run instructions)

**Interfaces:**
- Consumes: everything (config, postgres pool+migrate, migrations.FS, repo, service, handler, router).
- Produces: a running binary; nothing imported by other tasks.

- [ ] **Step 1: `cmd/main.go`**

Flow: `cfg, err := config.Load(os.Getenv)` → fatal on error; `slog` JSON handler at `cfg.LogLevel`; `postgres.NewPool`; `postgres.Migrate(ctx, pool, migrations.FS)`; `NewNewsRepo` → `NewNewsService` → `NewNewsHandler` → `httptransport.NewRouter`. Serve via `http.Server` with timeout constants (`ReadHeaderTimeout 5s`, `ReadTimeout 10s`, `WriteTimeout 10s`, `IdleTimeout 60s`, `shutdownTimeout 10s`); graceful shutdown via `signal.NotifyContext(ctx, SIGINT, SIGTERM)`; `pool.Close()` on exit. No `fc` constants here (not service/repository layer).

- [ ] **Step 2: Verify build** — `go build ./...`.

- [ ] **Step 3: Dockerfile + .dockerignore**

Multi-stage: `golang:<local-major.minor>-alpine` builder (`go mod download`, `CGO_ENABLED=0 go build -o /app/server ./cmd`), runtime `alpine` with non-root user, `EXPOSE 8080`, `ENTRYPOINT ["/app/server"]`. `.dockerignore`: `.env`, `docs/`, `.git`. Validate with `docker build -t mandat-backend .`.

- [ ] **Step 4: `README.md`**

Short English doc: prerequisites, `cp .env.example .env`, `docker compose up -d db`, `go run ./cmd`, endpoint table, integration-test note (needs `DATABASE_URL`).

- [ ] **Step 5: Full verification + smoke test**

```bash
gofmt -l .            # expect: empty
go vet ./...          # expect: clean
go test ./...         # expect: PASS (integration tests run when DATABASE_URL set)
docker compose up -d db
DATABASE_URL=postgres://mandat:mandat@localhost:5432/mandat?sslmode=disable go run ./cmd &
# smoke: POST a Cyrillic-titled draft, GET list, PUT publish, GET by id, DELETE, GET list again
```

Expected: server boots (migrations applied), CRUD round-trip works, 404 after delete, shutdown on Ctrl-C without hanging.

---

## Post-implementation

Per `backend/AGENTS.md` §15: after execution — `verification-before-completion`, then `requesting-code-review` (dispatch the `glm-reviewer` subagent for the independent Go review). Nothing is committed without an explicit owner command.
