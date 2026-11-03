# News Search Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `GET /api/news/search` returning published-article cards matching a title query.

**Architecture:** Service validates and caps the query; a new `NewsRepo.SearchPublished` runs one parameterized `ILIKE` query; `PublicNewsHandler.Search` exposes it with the existing `{items, page, limit}` envelope. No new packages, no migration.

**Tech Stack:** Go, Gin, pgx, testify, slog.

**Spec:** `/home/aix/GolandProjects/mandat/docs/superpowers/specs/2026-11-03-news-search-design.md`

## Global Constraints

- Clean Architecture: domain never imports Gin/pgx; handlers stay thin; business rules in service.
- Every service/repo function starts with `const fc = "<name>_<Func>"` and wraps errors with `%w` including `fc`.
- Logging only in the service layer, structured slog, never secrets.
- No raw DB errors to clients; classification via `internal/errs` sentinels + the `errResps` table.
- `gofmt`, `go vet ./...`, `go test ./...` green before done.
- Tests (`*_test.go`) are written only by the `test-author` subagent; primary never edits them.
- NEVER commit or push without the user's explicit command (repo rule overrides the usual per-task commit step: end tasks with a status report instead).

## Review Focus

- `q` with only spaces behaves like empty (returns empty list, not 400), because users clear the field while typing.
- `limit=0`, negative, and non-numeric each return 400 rather than silently defaulting, matching `ListPublished`.
- A title containing `%` or `_` matches literally, not as a wildcard.
- An article with slug `search` does not hijack `GET /api/news/search`.
- Drafts never appear in results even when their title matches.

## File Structure

- Modify `internal/service/news.go`: constants + `SearchPublished` method.
- Modify `internal/service/news.go` interface block: extend `NewsRepository` with `SearchPublished`.
- Modify `internal/repository/postgres/news.go`: `SearchPublished` query + LIKE-escape helper.
- Modify `internal/testsupport/stubrepo.go`: in-memory `SearchPublished` for the stub (NOT a `*_test.go` file, primary edits it).
- Modify `internal/transport/http/handler/public_news.go`: extend `PublicNewsService`, add `Search`, register route.
- Modify `backend/README.md`: Public API table row.
- Tests (test-author): `internal/service/search_test.go` (new), additions to handler/public tests, `internal/repository/postgres/news_search_integration_test.go` (new).

## Task 1: Service search

**Files:**
- Modify: `internal/service/news.go`
- Test: `internal/service/search_test.go` (test-author)

**Interfaces:**
- Consumes: `NewsRepository` (extended below), `models.ListFilter` (existing, unused here), `errs.ErrInvalidInput`.
- Produces: `func (s *NewsService) SearchPublished(ctx context.Context, query string, limit int) (*ListResult, error)`; constants `DefaultSearchLimit = 7`, `MaxSearchLimit = 20`, `MaxSearchQueryLen = 20`; interface method `SearchPublished(ctx context.Context, query string, limit int) ([]*models.News, error)`.

- [ ] **Step 1: test-author writes failing service tests** — `TestSearchPublished_EmptyQueryReturnsEmptyList` (assert `Items` non-nil empty, no error), `TestSearchPublished_TrimsQuery` (assert repo receives trimmed string), `TestSearchPublished_ClampsLimitAboveMax` (limit 99 → repo called with 20), `TestSearchPublished_RejectsBadLimit` (0 and -3 → `errors.Is(err, errs.ErrInvalidInput)`), `TestSearchPublished_RejectsQueryOver20Chars` (21-char query → `ErrInvalidInput`), `TestSearchPublished_OrdersAndShapesResult` (stub items returned as `ListResult` with `Page: 1`, echoed limit).
- [ ] **Step 2: Run to verify they fail** — `go test ./internal/service/ -run TestSearchPublished -v`, expected FAIL (method undefined).
- [ ] **Step 3: Primary implements** — add the three constants; add `SearchPublished` to the `NewsRepository` interface; implement `func (s *NewsService) SearchPublished(ctx context.Context, query string, limit int) (*ListResult, error)` in `internal/service/news.go` with `const fc = "newsService_SearchPublished"`: trim query, empty → `&ListResult{Items: []*models.News{}, Page: 1, Limit: resolvedLimit}` with resolvedLimit = limit if 1..20 else default/clamped; limit < 1 → `ErrInvalidInput`; limit > 20 → 20; trimmed rune length > 20 → `ErrInvalidInput`; else call repo, wrap error with `fc`, log failure via `s.logger.ErrorContext`, return `&ListResult{Items: items (non-nil), Page: 1, Limit: resolvedLimit}`.
- [ ] **Step 4: Primary adds stub support** — implement `SearchPublished` on `testsupport.StubRepo` in `internal/testsupport/stubrepo.go` with `const fc = "stubRepo_SearchPublished"`: case-insensitive `strings.Contains` on title, only `Status == models.StatusPublished`, order `PublishedAt` desc (nil last), then ID desc, apply limit.
- [ ] **Step 5: Run to verify green** — `go test ./internal/service/ -v` and `gofmt -l internal/ | head`, expected PASS and no output.

## Task 2: HTTP handler and route

**Files:**
- Modify: `internal/transport/http/handler/public_news.go`
- Test: `internal/transport/http/handler/public_search_test.go` (test-author, new file)

**Interfaces:**
- Consumes: `NewsService.SearchPublished` from Task 1; `queryInt` (existing); `dto.PublicNewsListResponse`, `dto.NewsToPublicResponse` (existing).
- Produces: `GET /api/news/search?q=&limit=` → `200 {items, page, limit}`.

- [ ] **Step 1: test-author writes failing handler tests** — `TestPublicSearch_ReturnsEnvelope` (seeded stub, `q` matching two published titles → 200, `items` length 2, `page: 1`, `limit: 7`, card fields `slug/title/category`), `TestPublicSearch_EmptyQueryReturnsEmptyItems` (`q=%20%20` → 200, `items: []`), `TestPublicSearch_InvalidLimit400` (`limit=abc` and `limit=0` → 400 `Invalid input`), `TestPublicSearch_LongQuery400` (101 chars → 400), `TestPublicSearch_ExcludesDrafts` (draft with matching title absent), `TestPublicSearch_SearchRouteBeatsSlug` (stub holds an article with slug `search`; `GET /api/news/search?q=x` returns the search envelope, not the article detail).
- [ ] **Step 2: Run to verify they fail** — `go test ./internal/transport/http/handler/ -run TestPublicSearch -v`, expected FAIL (route undefined).
- [ ] **Step 3: Primary implements** — extend `PublicNewsService` with `SearchPublished(ctx context.Context, query string, limit int) (*service.ListResult, error)`; add `func (h *PublicNewsHandler) Search(c *gin.Context)` (no `fc` constant — handlers don't use them): `limit, err := queryInt(c, "limit", service.DefaultSearchLimit)` → on error `WriteError`; call svc, on error `WriteError`; map items via `dto.NewsToPublicResponse`; respond `c.JSON(http.StatusOK, dto.PublicNewsListResponse{Items: items (non-nil), Page: result.Page, Limit: result.Limit})`. Register `news.GET("/search", h.Search)` before the `/:slug` line in `Register`.
- [ ] **Step 4: Run to verify green** — `go test ./internal/transport/http/handler/ -v`, expected PASS.

## Task 3: Repository query (integration)

**Files:**
- Modify: `internal/repository/postgres/news.go`
- Test: `internal/repository/postgres/news_search_integration_test.go` (test-author, new file)

**Interfaces:**
- Consumes: `models.News`, `newsColumns`, `scanNews`, `mapError` (all existing).
- Produces: `func (r *NewsRepo) SearchPublished(ctx context.Context, query string, limit int) ([]*models.News, error)`.

- [ ] **Step 1: test-author writes the failing integration test** — `TestNewsRepo_SearchPublished` with fixtures: two published articles (newer/older, Ukrainian titles sharing a word), one draft with a matching title, one published article whose title contains a literal `%`; assert newest-first order, draft excluded, `%` matched literally, `LIMIT` respected. Test truncates `news`: runs ONLY with `DATABASE_URL` pointing at a disposable test DB and ONLY after the user's explicit permission for that run.
- [ ] **Step 2: Run to verify it fails or skips** — `DATABASE_URL=... go test ./internal/repository/postgres/ -run TestNewsRepo_SearchPublished -v`, expected FAIL (method undefined) or SKIP without `DATABASE_URL`.
- [ ] **Step 3: Primary implements** — `func (r *NewsRepo) SearchPublished(ctx context.Context, query string, limit int) ([]*models.News, error)` with `const fc = "newsRepo_SearchPublished"`: escape `\` → `\\`, `%` → `\%`, `_` → `\_` via `strings.NewReplacer`; query `SELECT <newsColumns> FROM news WHERE status = $1 AND title ILIKE '%' || $2 || '%' ESCAPE '\' ORDER BY published_at DESC NULLS LAST, id DESC LIMIT $3` with `(models.StatusPublished, escaped, limit)`; scan rows, wrap errors with `fc` + `mapError`.
- [ ] **Step 4: Run to verify green (needs user permission first)** — request permission naming the target test DB, then `DATABASE_URL=... go test ./internal/repository/postgres/ -run TestNewsRepo_SearchPublished -v`, expected PASS.

## Task 4: Docs and full verification

**Files:**
- Modify: `backend/README.md` (Public API table: add `GET /api/news/search?q=&limit=` row).

- [ ] **Step 1: Primary updates README** — one table row: ``GET`` `/api/news/search?q=&limit=` — live title suggestions over published articles, newest first, default limit 7, max 20.
- [ ] **Step 2: Run full checks** — `gofmt -l .` (empty), `go vet ./...` (clean), `go test ./...` (PASS; repo integration tests need `DATABASE_URL`, otherwise they skip).
- [ ] **Step 3: Report done** — summarize changes per file, do NOT commit (repo rule).
