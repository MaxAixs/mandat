# Category Pages Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add header-linked Sport and International News pages that list only published articles from their selected rubric.

**Architecture:** Extend the existing public list filter with an optional validated category and apply it in PostgreSQL before pagination. Reuse the frontend public-news mapper and `NewsCard` in a new category page/hook, then register two routes and update the two header links.

**Tech Stack:** Go, Gin, PostgreSQL/pgx, React 19, TypeScript, Tailwind CSS, Node test runner.

**Spec:** `docs/superpowers/specs/2026-10-31-category-pages-design.md`

## Global Constraints

- Tests are written only by the `test-author` subagent; the primary session implements production code.
- Run backend unit tests with `env -u DATABASE_URL go test ./...`; do not run repository integration fixtures that truncate `news` without separate explicit permission.
- `category` is optional and accepts only existing `models.Category` values; unknown values return `400 Invalid input`.
- Preserve home/admin behavior and the existing API response shape.
- Do not add dependencies, edit unrelated code, commit, or push without an explicit command.
- Preserve the local untracked legacy `frontend/tests/admin-page.test.tsx`; do not include it in full-suite compilation because it targets the superseded text badge.

## Review Focus

- An invalid category must produce a client error, not an empty successful page (Task 1 service/handler tests).
- Category filtering must occur before SQL `LIMIT/OFFSET`, so a rubric with older articles is not lost behind unrelated newer items (Task 1 repository integration coverage; execution requires permission).
- Omitting `category` must leave the home feed and its main-event/featured ordering unchanged (Task 1 existing tests plus Task 2 API compatibility test).
- Switching rubric or requesting a page beyond the available results must not show stale articles from a previous request (Task 2 hook tests and request-ID guard).
- Empty, loading, and failed responses must be distinguishable and retry must reload the same category/page (Task 3 page tests).

---

### Task 1: Backend category filter

**Files:**
- Modify: `backend/internal/models/news.go`
- Modify: `backend/internal/service/news.go`
- Modify: `backend/internal/repository/postgres/news.go`
- Modify: `backend/internal/testsupport/stubrepo.go`
- Modify: `backend/internal/transport/http/handler/public_news.go`
- Test: `backend/internal/service/news_test.go`
- Test: `backend/internal/transport/http/handler/public_news_test.go`
- Test: `backend/internal/repository/postgres/news_integration_test.go`

**Interfaces:**
- Produces: `models.ListFilter{Category *models.Category, Status *models.Status, Page int, Limit int}`.
- Changes: `NewsService.ListPublished(ctx context.Context, page int, limit int, category string)`.
- Changes: `PublicNewsService.ListPublished(ctx context.Context, page int, limit int, category string)`.
- Accepts: `GET /api/news?page=1&limit=20&category=Спорт`; empty category preserves existing behavior.

- [ ] **Step 1: Dispatch `test-author` for failing backend tests**

Request service tests proving valid categories reach `ListFilter`, empty category leaves it nil, and an unknown category returns `ErrInvalidInput` without a repository call. Request handler tests proving `?category=Спорт` returns only published Sport items, the general list remains unfiltered, and an invalid category returns 400. Add repository integration coverage proving category filtering happens before pagination; do not execute that fixture without permission.

- [ ] **Step 2: Run backend tests and verify RED**

Run: `cd backend && env -u DATABASE_URL go test ./internal/service ./internal/transport/http/handler`
Expected: compilation or behavioral failure because the category parameter/filter is not implemented.

- [ ] **Step 3: Implement validated filtering**

Add `Category *Category` to `ListFilter`. In `ListPublished`, validate non-empty input with `models.Category(category).Valid()` and populate the filter. Pass `c.Query("category")` through the public handler. Mirror exact category matching in `StubRepo.ListPublished`.

- [ ] **Step 4: Apply the category in SQL before pagination**

Build `ListPublished` arguments dynamically: `status = $1`, optional `AND category = $N`, then existing ordering and `LIMIT/OFFSET`. Keep the unfiltered SQL ordering unchanged.

- [ ] **Step 5: Run backend unit verification**

Run: `cd backend && env -u DATABASE_URL go test ./... && go vet ./...`
Expected: PASS; PostgreSQL integration tests skip with `DATABASE_URL` unset.

### Task 2: Frontend category API and data hook

**Files:**
- Modify: `frontend/src/api/news.ts`
- Modify: `frontend/src/api/index.ts`
- Create: `frontend/src/hooks/use-category-news.ts`
- Test: `frontend/tests/public-news-api.test.ts`
- Test: `frontend/tests/category-news-api.test.ts`

**Interfaces:**
- Changes: `listPublicNews(page = 1, limit = 20, category?: Category): Promise<PublicNewsListResponse>`.
- Produces: `CategoryNewsData { articles: Article[]; page: number; limit: number }`.
- Produces: `getCategoryNews(category: Category, page = 1, limit = 20): Promise<CategoryNewsData>`.
- Produces: `useCategoryNews(category)` returning `{ articles, loading, error, page, limit, hasNextPage, retry, nextPage, previousPage }`.

- [ ] **Step 1: Dispatch `test-author` for failing API/hook tests**

Assert `listPublicNews(2, 20, 'Спорт')` requests page 2 with the exact encoded category and that existing two-argument calls omit `category`. Assert `getCategoryNews` maps API items through the existing `Article` shape and preserves page/limit. Cover empty results, error reset, stale-response rejection, previous-page lower bound, and `hasNextPage` when a full page is returned.

- [ ] **Step 2: Compile and run the focused tests to verify RED**

Use a test tsconfig under `frontend/tests/testdata/` supplied by `test-author`; run the emitted Node tests.
Expected: failure because category query/mapping/hook behavior is absent.

- [ ] **Step 3: Implement API mapping without changing home requests**

Add the optional third `category` parameter and set it only when provided. Export `getCategoryNews`, reusing the private `toArticle` mapper in `src/api/index.ts`; do not alter `getHomeNews`.

- [ ] **Step 4: Implement the category hook**

Use the existing `usePublicNews` request-ID pattern. Default to 20 items per page, clear stale data on errors, reset to page 1 when category changes, and expose bounded previous/next actions with `hasNextPage = articles.length === limit`.

- [ ] **Step 5: Run focused tests to verify GREEN**

Re-run Step 2.
Expected: all category API/hook tests PASS and existing public API tests remain green.

### Task 3: Category page and header routes

**Files:**
- Create: `frontend/src/pages/Category.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/constants/index.ts`
- Test: `frontend/tests/category-pages.test.tsx`
- Test: `frontend/tests/testdata/tsconfig.category-pages.json`

**Interfaces:**
- Consumes: `useCategoryNews` from Task 2 and existing `NewsCard`, `Header`, `Footer`.
- Produces: `CategoryPage({ category: Category })` with the rubric name as the page title.
- Routes: `/sport` → category `Спорт`; `/international` → category `Міжнародні новини`.

- [ ] **Step 1: Dispatch `test-author` for failing page tests**

Cover both route headings, exact header hrefs, article links, loading, retryable error, empty-rubric copy, page-number display, and disabled/enabled pagination controls. Keep real presentational rendering; mock only data hooks where SSR cannot perform effects.

- [ ] **Step 2: Run page tests and verify RED**

Compile with `tests/testdata/tsconfig.category-pages.json` and run the emitted `category-pages` Node test.
Expected: failure because the page/routes/links do not exist.

- [ ] **Step 3: Implement the category page**

Render `Header`, a main region with the category heading, then loading/error/empty states or a responsive `NewsCard` grid. Add «Назад», current-page text, and «Далі» controls consistent with Admin pagination; keep `Footer`. Use the existing theme tokens and no new dependencies.

- [ ] **Step 4: Register routes and update header links**

Map `/sport` and `/international` in `App.tsx`. Change only the corresponding `NAV_LINKS` hrefs; leave «Новини» at `/` and all other links unchanged.

- [ ] **Step 5: Run page tests and frontend checks**

Re-run Step 2, then run all maintained frontend tests (excluding only the local legacy admin-page test), `npm run lint`, and `npm run build`.
Expected: all maintained tests, lint, type-checking, and production build PASS.

### Task 4: Review and final verification

**Files:**
- Review only files changed by Tasks 1–3.

- [ ] **Step 1: Dispatch `reviewer`**

Request correctness, SQL pagination, API compatibility, stale-request, accessibility, and regression review. The reviewer writes `.superpowers/reviews/2026-10-31-category-pages.txt` and changes no files.

- [ ] **Step 2: Address findings and rerun checks**

Primary fixes production findings; test changes return to `test-author`. Repeat backend unit checks and maintained frontend checks. Do not run destructive PostgreSQL integration fixtures without explicit permission.

- [ ] **Step 3: Report results and limitations**

Report the routes, exact verification evidence, and that repository integration tests were not run unless separately authorized. Do not commit or push without a new explicit command.
