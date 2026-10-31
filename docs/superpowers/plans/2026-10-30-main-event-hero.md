# Main Event Hero Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let editors select one published article whose title, description, cover, and link populate the home-page Hero.

**Architecture:** Add `main_event` alongside `featured` through the domain, service, PostgreSQL repository, and DTOs. A partial unique index and the existing transactional replacement pattern enforce one published main event. The existing home-news request returns that article first; the frontend maps it to `Article | null` and passes it to Hero, with static content as fallback.

**Tech Stack:** Go, PostgreSQL/goose, Gin, React 19, TypeScript, Tailwind CSS, Node test runner.

**Spec:** `docs/superpowers/specs/2026-10-30-main-event-hero-design.md`

## Global Constraints

- Tests are written only by the `test-author` subagent; the primary agent implements production code and fixes.
- Do not edit existing tests directly. Request test changes from `test-author`.
- Do not run integration tests that execute `TRUNCATE news` without explicit permission. Use `env -u DATABASE_URL go test ./...` for the non-destructive unit run.
- Do not commit or push unless explicitly commanded.
- Preserve unrelated uncommitted user work and existing `featured` behavior.
- No new dependencies.

## Review Focus

- Concurrent published updates selecting different main events must leave exactly one published `main_event` row; pin with a repository integration test (permission required to run).
- A failed create/update after clearing the previous selection must roll back; pin with a repository integration test (permission required to run).
- An old selected article must still appear in the first 20 public items; pin with repository ordering coverage.
- A selected article with an empty cover must retain the bundled Hero background; pin with a Hero rendering test.
- No selected article or a failed public-news load must preserve the static Hero without a link; pin with API mapping and Hero fallback tests.

---

### Task 1: Backend main-event flag

**Files:**
- Create: `backend/migrations/00007_news_main_event.sql`
- Modify: `backend/internal/models/news.go`
- Modify: `backend/internal/service/news.go`
- Modify: `backend/internal/testsupport/stubrepo.go`
- Modify: `backend/internal/repository/postgres/news.go`
- Modify: `backend/internal/transport/http/dto/request.go`
- Modify: `backend/internal/transport/http/dto/response.go`
- Test: `backend/internal/service/news_test.go`
- Test: `backend/internal/transport/http/handler/public_news_test.go`
- Test: `backend/internal/transport/http/handler/news_test.go`
- Test: `backend/internal/repository/postgres/news_integration_test.go`

**Interfaces:**
- Produces: `models.News.MainEvent bool`, `service.NewsInput.MainEvent bool`.
- JSON: admin/public requests and responses use `main_event` as a boolean.
- Public ordering: `main_event DESC, featured DESC, published_at DESC NULLS LAST, id DESC`.

- [ ] **Step 1: Dispatch `test-author` for failing backend tests**

Request service coverage for create/update persistence and published-selection replacement, HTTP coverage for `main_event` request/response mapping, and repository integration coverage for round-trip, replacement, concurrent selection, rollback, and first-page ordering. Keep existing featured tests unchanged.

- [ ] **Step 2: Run the new unit tests and verify RED**

Run: `cd backend && env -u DATABASE_URL go test ./internal/service ./internal/transport/http/handler`
Expected: FAIL because `MainEvent` and `main_event` are not implemented. Do not run the integration package against a database.

- [ ] **Step 3: Add the migration and propagate the field**

Add `main_event BOOLEAN NOT NULL DEFAULT FALSE` and a unique partial index on `status` where `status = 'published' AND main_event = TRUE`; down drops the index and column. Add the Go domain/input/DTO fields and mappings, including public list and detail responses.

- [ ] **Step 4: Implement atomic selection and public ordering**

Extend repository INSERT/UPDATE/SELECT/scan lists. Generalize the existing lock helper so a published write with either flag takes one table lock and clears only the flags being selected. Mirror replacement and main-event-first ordering in `StubRepo` without changing featured semantics.

- [ ] **Step 5: Run backend unit tests and verify GREEN**

Run: `cd backend && env -u DATABASE_URL go test ./... && go vet ./...`
Expected: PASS; PostgreSQL integration tests report SKIP because `DATABASE_URL` is unset.

### Task 2: Admin editor switch and API types

**Files:**
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/api/news.ts`
- Modify: `frontend/src/components/admin/NewsEditorDialog.tsx`
- Test: `frontend/tests/news-api.test.ts`
- Test: `frontend/tests/admin-news-editor.test.tsx` (create)

**Interfaces:**
- Consumes: admin API `main_event` from Task 1.
- Produces: required `NewsFormValues.main_event: boolean`, default `false`; `AdminNews` inherits it.

- [ ] **Step 1: Dispatch `test-author` for failing admin tests**

Request payload serialization coverage and server-rendered editor coverage showing the «Головна подія дня» switch, its checked state while editing, and its unchecked default for a new article.

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `cd frontend && npx tsc -b && node --test tests/news-api.test.js tests/admin-news-editor.test.js`
Expected: FAIL because the form/API field and switch do not exist.

- [ ] **Step 3: Implement the admin field**

Add `main_event` to editable/public TypeScript contracts, initialize it to `false`, populate it from `article.main_event`, and render a second existing `Switch` with a calendar/star icon consistent with the current editor. Keep «На головній» unchanged.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run the Step 2 command again. Expected: PASS.

### Task 3: Dynamic Hero content and layout

**Files:**
- Modify: `frontend/src/api/index.ts`
- Modify: `frontend/src/hooks/use-public-news.ts`
- Modify: `frontend/src/pages/Home.tsx`
- Modify: `frontend/src/sections/Hero.tsx`
- Test: `frontend/tests/public-news-api.test.ts`
- Test: `frontend/tests/home-layout.test.tsx`

**Interfaces:**
- Consumes: public `main_event` and admin-selected article from Tasks 1–2.
- Produces: `HomeNewsData.mainEvent: Article | null`; `Hero({ mainEvent }: { mainEvent?: Article | null })`.

- [ ] **Step 1: Dispatch `test-author` for failing Hero/API tests**

Request coverage proving `getHomeNews` selects only `main_event`, returns `null` when absent, and preserves existing feeds. Hero tests must cover selected title/description/cover, `/news/:slug` link, fallback content and background, no fallback link, missing-cover fallback, and desktop bottom-right audience classes.

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `cd frontend && npx tsc -b && node --test tests/public-news-api.test.js tests/home-layout.test.js`
Expected: FAIL because `mainEvent` and dynamic Hero rendering are not implemented.

- [ ] **Step 3: Map and render the selected article**

Find the raw public item with `main_event === true`, map it through `toArticle`, and default the hook to `null`. Pass it from Home to Hero. Use selected `title`, `excerpt`, and `imageUrl || heroBg`; wrap only a selected heading in `Link` to `/news/${mainEvent.id}`. Preserve the current static fallback and gradients. Position the audience block bottom-right at `lg` while retaining mobile flow.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run the Step 2 command again. Expected: PASS.

- [ ] **Step 5: Run all frontend checks**

Run: `cd frontend && npx tsc -b && node --test tests/*.test.js && npm run lint && npm run build`
Expected: all tests, lint, type-checking, and production build PASS.

### Task 4: Final review and verification

**Files:**
- Review all task files; fix only findings related to this feature.

- [ ] **Step 1: Dispatch `reviewer`**

Ask for a correctness, concurrency, API compatibility, accessibility, and regression review. The reviewer writes `.superpowers/reviews/2026-10-30-main-event-hero.txt` and does not change code.

- [ ] **Step 2: Address findings and rerun checks**

Primary agent fixes production findings; any test changes go back to `test-author`. Repeat backend/frontend verification and request follow-up review for substantive fixes.

- [ ] **Step 3: Report evidence and limitations**

Report exact commands and results. State that destructive PostgreSQL integration tests were not run unless the user separately authorized their `TRUNCATE news` fixtures. Do not commit.
