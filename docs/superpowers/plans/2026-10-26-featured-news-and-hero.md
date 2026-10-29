# Featured News and Hero Height Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Guarantee one published featured article, place it first in the large home-grid position, and increase Hero height to match the article cover.

**Architecture:** PostgreSQL owns the atomic featured transfer and a partial unique index guarantees the invariant. Public list ordering puts featured first globally, while the frontend explicitly keeps latest-news and ticker chronological. The existing card and Hero components receive only layout changes.

**Tech Stack:** Go, Gin service/repository patterns, PostgreSQL, pgx, Goose, React 19, TypeScript, Tailwind CSS, Node SSR/API tests.

**Spec:** `docs/superpowers/specs/2026-10-26-featured-news-and-hero-design.md`

## Global Constraints

- Only published articles participate in featured uniqueness; drafts may retain featured intent.
- Existing service and HTTP request/response contracts remain unchanged.
- Featured transfer and its create/update are atomic; existing duplicate published-featured rows keep the newest publication.
- Latest-news and ticker remain chronological; only the main article grid receives featured-first ordering.
- Tests are written only by `test-author`; add no dependencies and preserve unrelated changes.
- No commits or pushes.

## Review Focus

- Concurrent published-featured writes must serialize rather than leave two featured rows or a spurious uniqueness failure; add an integration test in Task 1.
- A failed insert/update after demotion must roll back and preserve the previous featured article; add a duplicate-slug rollback test in Task 1.
- Existing duplicate rows must migrate deterministically, retaining the latest publication; add a migration integration test in Task 1.
- A featured article older than the first 20 publications must still lead page one; extend repository ordering tests in Task 1.
- Hero's minimum heights must not clip its existing content on small screens; pin `min-h` rather than fixed `h` classes in Task 2 SSR tests.

---

### Task 1: Atomic Published-Featured Invariant and Ordering

**Files:**
- Create: `backend/migrations/00002_single_published_featured.sql`
- Modify: `backend/internal/repository/postgres/news_repository.go`
- Modify: `backend/internal/testsupport/stubrepo.go` (test double only, owned by `test-author`)
- Test: `backend/internal/service/news_test.go`
- Test: `backend/internal/repository/postgres/news_repository_integration_test.go`

**Interfaces:**
- Preserves: `NewsRepository`, `NewsService.Create`, `NewsService.Update`, and all HTTP DTOs.
- Produces: partial unique index for rows satisfying `status = 'published' AND featured = TRUE`.
- Produces: public list ordering `featured DESC, published_at DESC NULLS LAST, id DESC`.

- [x] **Step 1: Dispatch `test-author` to write failing service tests** named `TestCreate_PublishedFeaturedReplacesPrevious`, `TestUpdate_PublishedFeaturedReplacesPrevious`, and `TestCreate_FeaturedDraftDoesNotReplacePublished`; assert exactly one published-featured item and that draft intent leaves the current published-featured item unchanged. Do not change the test double yet.
- [x] **Step 2: Dispatch `test-author` to write failing PostgreSQL integration tests** for create/update transfer, concurrent published-featured creates both succeeding with one final featured row, duplicate-slug rollback preserving the prior featured row, migration deduplication retaining latest publication, and featured-first page-one ordering with an old featured article beyond the 20 newest publications. Use named tests/subtests and existing `DATABASE_URL` skip behavior.
- [x] **Step 3: Run RED.** Run `go test ./internal/service ./internal/repository/postgres`; service tests must fail because previous featured state is not cleared, and integration tests must fail when a database is available or skip explicitly otherwise.
- [x] **Step 4: Dispatch `test-author` to update only the in-memory test double** so its Create/Update behavior mirrors the required published-featured replacement and draft distinction; rerun the service tests to confirm they pass against the desired contract.
- [x] **Step 5: Add the Goose migration.** In `00002_single_published_featured.sql`, deduplicate published-featured rows by `published_at DESC NULLS LAST, created_at DESC, id DESC`, then create a partial unique index; Down drops only that index.
- [x] **Step 6: Implement repository transfers atomically.** For published-featured create/update, begin a pgx transaction, take `LOCK TABLE news IN SHARE ROW EXCLUSIVE MODE`, clear featured only on published rows, perform the existing insert/update, and commit. Roll back on every error and preserve existing slug-conflict classification. Non-featured/draft paths remain direct operations.
- [x] **Step 7: Order public results globally.** Change only `ListPublished` ordering to featured first, then publication date and ID; do not change admin `List` ordering.
- [x] **Step 8: Run GREEN.** Run `gofmt` on changed Go files and `go test ./internal/service ./internal/repository/postgres`; record explicitly whether PostgreSQL integration tests ran or skipped.

### Task 2: Chronological Secondary Feeds, Featured Grid, and Hero Height

**Files:**
- Modify: `frontend/src/api/index.ts`
- Modify: `frontend/src/sections/Hero.tsx`
- Test: `frontend/tests/public-news-api.test.ts`
- Create: `frontend/tests/home-layout.test.tsx`

**Interfaces:**
- Consumes: Task 1's featured-first public response.
- Preserves: `HomeNewsData`, `Article`, `NewsCard`, and Hero content.
- Produces: Hero classes `min-h-[240px] sm:min-h-[360px] lg:min-h-[480px]`.

- [x] **Step 1: Dispatch `test-author` to add failing API and SSR tests.** The API fixture must return an older featured item first and a newer regular item second; assert articles retain featured-first order while latest-news and ticker are chronological. SSR tests assert featured cards retain `lg:col-span-2 lg:row-span-2`, regular cards lack them, and Hero contains all three exact `min-h` classes with existing image/text.
- [x] **Step 2: Compile and run both frontend suites to verify RED** using temporary configs under `/tmp/opencode`; expected failures are secondary-feed ordering and Hero height. Existing card-class assertions may pass as preservation checks.
- [x] **Step 3: Restore chronological secondary feeds.** In `getHomeNews`, sort a copied response array by valid `published_at` descending with missing/invalid dates last, and use it only for `latestNews` and `tickerItems`; map `articles` directly from the featured-first response without reordering.
- [x] **Step 4: Increase Hero responsively.** Add `min-h-[240px] sm:min-h-[360px] lg:min-h-[480px]` to the section, preserve current padding/content/gradients, and add flex alignment only if needed to center existing content without clipping.
- [x] **Step 5: Run focused GREEN, then full verification.** Run all frontend Node suites, `npm run lint`, `npm run build`, backend `go test ./...`, and `go vet ./...`; all available checks must exit 0.
- [x] **Step 6: Request independent review.** Dispatch `reviewer` over the explicit feature file set, read its report, fix Critical/Important findings through RED→GREEN, defer Minor findings, and rerun full verification.
