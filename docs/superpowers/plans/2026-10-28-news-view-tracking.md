# News View Tracking Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show an up-to-date public article view count while allowing one accepted view per anonymous visitor per article per hour.

**Architecture:** A separate public `POST` endpoint atomically updates a per-article/per-viewer cooldown row and increments the existing `news.views` counter only when appropriate. The frontend records the view after loading the article and updates the count without making article rendering depend on tracking success.

**Tech Stack:** Go 1.27, Gin, pgx/v5, PostgreSQL, goose, React 19, TypeScript, node:test.

**Spec:** `docs/superpowers/specs/2026-10-28-admin-auth-and-view-tracking-design.md`

## Global Constraints

- Follow `backend/AGENTS.md` and `frontend/AGENTS.md`; preserve Clean Architecture and existing public DTOs.
- Only `test-author` writes or edits tests; implement production code only after observing RED.
- Do not add dependencies or store IP addresses, user agents, or other browser fingerprints.
- The cooldown is exactly one hour and is enforced atomically by PostgreSQL, never only in React or service memory.
- Do not execute destructive SQL or the existing `TRUNCATE news` integration helper without specific approval. Prefer rollback-scoped integration fixtures.
- Do not commit or push unless explicitly commanded.

## Review Focus

- Two simultaneous first views by one visitor: exactly one increment, not two — Task 1 concurrent repository test.
- Missing/invalid viewer cookie: issue a fresh UUID and still show the article; do not turn it into a public `400` — Task 2 handler tests.
- Draft or missing article: return the existing `404` and create no viewer row or counter change — Task 1 repository tests and Task 2 HTTP tests.
- Tracking network/database failure: preserve the loaded article and its previous count rather than replacing the page with an error — Task 3 API/hook tests.
- React StrictMode duplicate effects: server cooldown prevents a second increment even if the frontend requests twice — Tasks 1 and 3 tests.

---

### Task 1: Atomic view-cooldown repository and service

**Files:**
- Create: `backend/migrations/00005_news_viewers.sql`
- Create: `backend/internal/repository/postgres/view_repository.go`
- Create: `backend/internal/service/views.go`
- Test: `backend/internal/repository/postgres/view_repository_integration_test.go`, `backend/internal/service/views_test.go`

**Interfaces:**
- Produces:

```go
func NewViewRepo(db ViewDB) *ViewRepo
func (r *ViewRepo) RecordView(ctx context.Context, slug string, viewerID uuid.UUID, cooldown time.Duration) (int64, error)
func NewViewService(repo ViewRepository, logger *slog.Logger) *ViewService
func (s *ViewService) RecordView(ctx context.Context, slug string, viewerID uuid.UUID) (int64, error)
```

`ViewDB` requires `QueryRow(context.Context, string, ...any) pgx.Row`, allowing both `pgxpool.Pool` and a rollback-scoped `pgx.Tx`.

- [ ] **Step 1: test-author writes failing service and repository tests**

Service tests assert that the repository receives the supplied slug/viewer and exactly `time.Hour`, and that not-found/infrastructure errors retain their classification.

Repository tests cover: first view returns `1`; same viewer within one hour remains `1`; a backdated `last_viewed_at` older than one hour permits `2`; another viewer has an independent cooldown; missing/draft slugs return `errs.ErrNewsNotFound` and do not create rows.

Add a concurrent test with two independent transactions/connections starting together for the same viewer and article; assert final views is exactly one greater. Any committed fixture cleanup requires explicit user approval; otherwise keep this test skipped and cover the sequential cases with rollback fixtures.

- [ ] **Step 2: run RED**

Run: `cd backend && go test ./internal/service ./internal/repository/postgres -run 'Test.*View'`
Expected: FAIL because view service/repository are absent.

- [ ] **Step 3: implement migration and one-statement atomic write**

Migration Up creates `news_viewers(news_id, viewer_id, last_viewed_at)` with composite primary key, `news_id` FK `ON DELETE CASCADE`, and time index. Down drops only this new table and is not executed during implementation.

Implement one parameterized SQL statement with CTEs:

1. `target` selects the article ID only for the matching published slug.
2. `accepted` inserts `(news_id, viewer_id, now())`, or on conflict updates `last_viewed_at` only when the previous value is at least the parameterized cooldown old; return accepted `news_id`.
3. `incremented` updates `news.views = views + 1` only for accepted rows and returns views.
4. Select the incremented value or the unchanged current value, with `WHERE EXISTS (SELECT 1 FROM target)` so missing/draft articles produce `pgx.ErrNoRows`.

Pass `cooldown.String()` as an interval parameter. PostgreSQL row locking in `INSERT ... ON CONFLICT DO UPDATE` serializes same-viewer races; do not add an in-process lock.

`ViewService.RecordView` owns the `const viewCooldown = time.Hour`, validates non-empty slug and non-nil UUID, logs infrastructure failures with `fc`, and returns the current total.

- [ ] **Step 4: run GREEN**

Run: `cd backend && go test ./internal/service ./internal/repository/postgres -run 'Test.*View'`
Expected: service PASS; integration PASS on an approved safe database or SKIP when no `DATABASE_URL` is set.

### Task 2: Public view endpoint and anonymous cookie

**Files:**
- Create: `backend/internal/transport/http/dto/views.go`
- Create: `backend/internal/transport/http/handler/views.go`
- Create: `backend/internal/transport/http/middleware/viewer.go`
- Modify: `backend/internal/transport/http/handler/public_news.go`
- Modify: `backend/internal/transport/http/middleware/cors.go`
- Modify: `backend/cmd/main.go`
- Modify: `backend/README.md`
- Test: `backend/internal/transport/http/handler/views_test.go`

**Interfaces:**
- Consumes: Task 1 `ViewService.RecordView`.
- Produces: `POST /api/news/:slug/view` returning `200` with `{ "views": number }`.
- Produces: `middleware.EnsureViewer(secureCookie bool) gin.HandlerFunc` and `middleware.ViewerID(c *gin.Context) uuid.UUID` shared by article-detail GET and view POST.
- Cookie: `mandat_viewer`, UUID value, one-year expiry, `HttpOnly`, `SameSite=Lax`, `Path=/api/news`, configurable `Secure`.

- [ ] **Step 1: test-author writes failing handler tests**

Cover article GET without a cookie, valid existing UUID, malformed UUID, view POST without a cookie, service `404`, and service `500`. Assert GET issues the viewer cookie before tracking, a following POST reuses that exact UUID, a new valid cookie is issued only when needed, the raw response contains only the count, and `Secure` follows configuration. Simulate one GET followed by two requests carrying the same cookie and assert both use one UUID. Assert both routes remain public and never require the admin session cookie.

- [ ] **Step 2: run RED**

Run: `cd backend && go test ./internal/transport/http/handler -run 'Test.*View'`
Expected: FAIL because the handler/route does not exist.

- [ ] **Step 3: implement handler and wiring**

Use an interface for the view service so handler tests remain database-free. `EnsureViewer` parses an existing cookie with `uuid.Parse`; absent or invalid cookies produce `uuid.New()`, store the UUID in Gin context, and set a replacement cookie. Apply it to public article-detail GET and view POST. `ViewHandler` reads `ViewerID`, calls the service, and returns the count.

Wire the viewer middleware into the existing public detail route and wire `ViewRepo`, `ViewService`, and `ViewHandler` in `cmd/main.go`. Ensure credentialed CORS is enabled for the existing explicit origins (already delivered by the auth plan when executed first). Document the endpoint, one-hour semantics, cookie privacy, and failure response in README.

- [ ] **Step 4: run GREEN**

Run: `cd backend && go test ./internal/transport/http/handler -run 'Test.*View'`
Expected: PASS.

### Task 3: Public article view recording in React

**Files:**
- Modify: `frontend/src/api/news.ts`
- Modify: `frontend/src/hooks/use-public-article.ts`
- Modify: `frontend/README.md`
- Test: `frontend/tests/public-news-api.test.ts`, `frontend/tests/article-page.test.tsx`

**Interfaces:**
- Produces: `recordPublicNewsView(slug: string): Promise<number>` calling the encoded `POST /api/news/:slug/view` with `credentials: 'include'`.
- Consumes: the existing `usePublicArticle(slug)` state and `ArticleDetail.views` display; no new visual component is required.

- [ ] **Step 1: test-author writes failing API and article-state tests**

API tests assert the exact encoded URL, `POST`, credentials, numeric response mapping, backend error handling, and network error handling. Also assert public article-detail GET uses `credentials: 'include'` so the viewer cookie is available before tracking starts.

Add a testable hook/helper contract proving: loaded article renders before tracking completes; a successful tracking response replaces only `views`; stale responses for an old slug are ignored; a rejected tracking call retains the loaded article/count and does not set the article error; duplicate calls may occur without frontend-side counter math.

- [ ] **Step 2: run RED**

Compile/run the affected frontend node tests with the established temporary test tsconfig.
Expected: FAIL because `recordPublicNewsView` and hook integration are absent.

- [ ] **Step 3: implement tracking without blocking article content**

After `getPublicNewsArticle(slug)` resolves and the request is still current, set the article immediately and stop the main loading state. Start the view request separately; on success, update only the current article's `views` using the server-returned number. Catch tracking errors silently at the UI level while leaving article state and retry behavior intact. Increment the existing request ID on slug changes so stale responses cannot update a new article.

Do not increment locally, use localStorage, or add frontend cooldown timers.

- [ ] **Step 4: run GREEN and full verification**

Run: affected frontend tests, `cd frontend && npm run lint && npm run build`, and safe backend checks `cd backend && gofmt -w <changed-go-files> && go test ./... && go vet ./...`.
Expected: PASS. Keep `DATABASE_URL` unset unless the user approves the destructive legacy integration helper.

- [ ] **Step 5: request independent review**

Dispatch `reviewer` for the complete view-tracking diff and read its `.superpowers/reviews/*.txt` report. Fix Critical/Important findings; route test changes back to `test-author`.
