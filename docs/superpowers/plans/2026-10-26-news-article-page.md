# News Article Page Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Let readers open any published home-page news card at `/news/:slug` and read its full text within the existing site layout.

**Architecture:** Add a publication-checked detail operation to the existing backend service and expose a dedicated public DTO through `GET /api/news/:slug`. The frontend maps that response into an `ArticleDetail`, loads it in a route-scoped hook, and renders a new page between the existing header and footer. Cards and the logo use React Router links.

**Tech Stack:** Go, Gin, existing repository/service patterns, React 19, TypeScript, React Router 7, Tailwind CSS, lucide-react, Node tests.

**Spec:** `docs/superpowers/specs/2026-10-26-news-article-page-design.md`

## Global Constraints

- Tests are written only by the `test-author` subagent before the corresponding implementation; the primary agent never edits test files.
- Add no dependencies and preserve unrelated working-tree changes.
- Public detail responses contain public card fields plus `text`, never admin-only fields.
- Drafts and missing slugs both return `404 News article not found`.
- Render article text as plain text with preserved paragraphs; never inject HTML.
- Do not increment views and do not commit or push.

## Review Focus

- A draft requested by a guessed slug must not leak through the public endpoint; pin this in Task 1 handler tests.
- Slugs requiring URL escaping must reach the backend as one encoded path segment; pin this in Task 2 API tests.
- Multiline text must remain unchanged through API mapping and render as text rather than HTML; pin mapping in Task 2 and rendering in Task 3.
- A direct article visit must show loading and useful failure/retry states rather than a blank page; implement and inspect these states in Task 3.
- SPA navigation from a scrolled home page must begin at the top of the article; implement scroll reset in Task 3 and verify the code path during final review.

---

### Task 1: Published Article Detail API

**Files:**
- Modify: `backend/internal/service/news.go`
- Modify: `backend/internal/transport/http/dto/response.go`
- Modify: `backend/internal/transport/http/handler/public_news.go`
- Test: `backend/internal/service/public_news_test.go`
- Test: `backend/internal/transport/http/handler/public_news_test.go`

**Interfaces:**
- Produces: `NewsService.GetPublishedBySlug(ctx context.Context, slug string) (*models.News, error)`.
- Produces: `GET /api/news/:slug` with `slug`, `category`, `title`, `description`, `image_url`, `featured`, `views`, `published_at`, and `text`.
- Produces: `NewsToPublicDetailResponse(news *models.News) *PublicNewsDetailResponse`.

- [x] **Step 1: Dispatch `test-author` to add failing service tests** named `TestGetPublishedBySlug_ReturnsPublishedArticle`, `TestGetPublishedBySlug_RejectsDraft`, and `TestGetPublishedBySlug_WrapsRepositoryError`; assert the loaded slug, `errors.Is(err, errs.ErrNewsNotFound)`, and preservation of a sentinel repository cause respectively.
- [x] **Step 2: Dispatch `test-author` to add failing HTTP tests** named `TestPublicDetail_ReturnsPublishedArticle`, `TestPublicDetail_DraftAndMissingReturn404`, and `TestPublicDetail_RepoFailureReturnsGeneric500`; assert 200/full text/public fields only, identical 404 payloads for draft and unknown slugs, and a generic 500 without repository details.
- [x] **Step 3: Run the focused tests and verify RED.** Run `go test ./internal/service ./internal/transport/http/handler`; expected failures are missing service/handler/DTO behavior, not unrelated existing failures.
- [x] **Step 4: Implement the service and public DTO.** Reuse `repo.GetBySlug`; after a successful lookup, map every non-published status to `errs.ErrNewsNotFound`. Add a dedicated detail response containing the public card fields and `text`.
- [x] **Step 5: Register the detail route and thin handler.** Bind `news.GET("/:slug", h.GetBySlug)` after the list route, call `GetPublishedBySlug`, map through the public detail DTO, and delegate errors to `WriteError`.
- [x] **Step 6: Re-run focused tests and verify GREEN.** Run `go test ./internal/service ./internal/transport/http/handler`.

### Task 2: Frontend Article Data Loading

**Files:**
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/api/news.ts`
- Modify: `frontend/src/api/index.ts`
- Test: `frontend/tests/public-news-api.test.ts`

**Interfaces:**
- Consumes: Task 1's `GET /api/news/:slug` response.
- Produces: `PublicNewsDetail extends PublicNews` with `text: string`.
- Produces: `ArticleDetail extends Article` with `text: string`.
- Produces: `getPublicNews(slug: string): Promise<PublicNewsDetail>`.
- Produces: `getPublicNewsArticle(slug: string): Promise<ArticleDetail>`.

- [x] **Step 1: Dispatch `test-author` to add failing Node tests** named `getPublicNews uses the detail endpoint and encodes the slug`, `getPublicNews reports request errors`, and `getPublicNewsArticle maps a complete article`; assert the exact URL, backend/network messages, every `Article` field, compact views, Ukrainian publication-date formatting, and byte-for-byte unchanged multiline `text`.
- [x] **Step 2: Compile and run the frontend API tests to verify RED.** From `frontend/`, run `rm -rf /tmp/opencode/mandat-public-news-tests && npx tsc tests/public-news-api.test.ts src/api/news.ts src/api/index.ts src/types/index.ts --outDir /tmp/opencode/mandat-public-news-tests --module commonjs --target ES2022 --moduleResolution node --esModuleInterop --skipLibCheck --types node && node --test /tmp/opencode/mandat-public-news-tests/tests/public-news-api.test.js`; expected failure is the missing detail API/mapper.
- [x] **Step 3: Implement the typed detail request.** Call the existing request helper with `PUBLIC_NEWS_API_URL` and `/${encodeURIComponent(slug)}`; do not duplicate fetch/error handling.
- [x] **Step 4: Implement the detail mapper.** Reuse the existing `toArticle` conversion and add `text` without trimming, splitting, or interpreting it.
- [x] **Step 5: Re-run the frontend API tests and verify GREEN.**

### Task 3: Article Route, Page, and Navigation

**Files:**
- Create: `frontend/src/hooks/use-public-article.ts`
- Create: `frontend/src/pages/Article.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/components/NewsCard.tsx`
- Modify: `frontend/src/sections/Header.tsx`
- Test: `frontend/tests/article-page.test.tsx`

**Interfaces:**
- Consumes: `getPublicNewsArticle(slug)` and `ArticleDetail` from Task 2.
- Produces: `usePublicArticle(slug: string | undefined)` returning `{ article, loading, error, retry }` with the same stale-request protection pattern as `usePublicNews`.
- Produces: route `/news/:slug` rendering `Article`.
- Produces: exported presentational `ArticleContent({ article }: { article: ArticleDetail })` for SSR behavior tests.

- [x] **Step 0: Dispatch `test-author` to add failing SSR Node tests** proving the card's encoded article link, header logo `/`, retained header/footer on the article route, rendered image/title/date/views, preserved plain-text paragraphs, and HTML escaping. Compile TSX to `/tmp/opencode` and verify RED before implementation.

- [x] **Step 1: Implement the route-scoped loading hook.** For an undefined/empty slug return `article: null`, `loading: false`, and `error: 'Новину не знайдено.'`; otherwise clear stale data on retry/slug changes, use `getNewsErrorMessage`, and ignore out-of-order responses.
- [x] **Step 2: Implement the article page layout.** Keep `Header` and `Footer`; render a wide responsive cover (with the existing pattern fallback), then title, publication date, Eye icon/view count, and full text in a readable content column. Use `whitespace-pre-line` for paragraphs and never `dangerouslySetInnerHTML`.
- [x] **Step 3: Add page states and navigation behavior.** Show a spinner while loading, an alert plus retry button on failure, disable retry while loading, and scroll to the top whenever the slug changes. Use Ukrainian UI copy consistent with the home page.
- [x] **Step 4: Wire routes and links.** Register `/news/:slug`; make each complete `NewsCard` a keyboard-accessible React Router link to its encoded slug; change only the header logo target to `/`, preserving its appearance and all other header content.
- [x] **Step 5: Run focused and full verification.** Re-run frontend API tests, then `npm run lint` and `npm run build` in `frontend/`; run `go test ./...` and `go vet ./...` in `backend/`. All commands must exit 0.
- [x] **Step 6: Request independent review.** Dispatch `reviewer` for the complete feature, read its `.superpowers/reviews/*.txt` report, fix actionable findings through the owning roles, and repeat relevant verification.
