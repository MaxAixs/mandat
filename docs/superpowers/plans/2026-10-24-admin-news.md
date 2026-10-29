# Admin News Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Ukrainian-language black-and-red news administration screen at `/admin`, backed by the existing `/api/admin/news` CRUD endpoints.

**Architecture:** Keep the Go API unchanged. Add a typed frontend API module, a focused news editor dialog, and an admin page that owns loading, filtering, pagination, and mutation state.

**Tech Stack:** React 19, strict TypeScript, Vite, Tailwind, existing shadcn/ui, lucide-react, Node built-in tests.

**Spec:** User request: one news-management section at `/admin`; follow existing backend handlers; all admin copy in Ukrainian; visual discretion delegated to the implementer.

## Global Constraints

- Do not modify backend code or existing public-site behavior.
- Do not add dependencies or edit shadcn/ui files.
- Use `@/` imports in application code, existing `mandat-*` colors, and Ukrainian copy.
- API fields and statuses must match the Go DTOs: `draft`, `published`, and the five existing categories.
- Preserve unrelated user changes; do not commit or push.

## Review Focus

- Failed requests must show a useful Ukrainian error and never replace real data with mock content; test API error parsing in Task 1.
- Encoded slugs and omitted status filters must produce correct URLs; test URL construction in Task 1.
- Publishing without text must be blocked before the request; assert this in the form validation step in Task 2.
- A short final page must disable “next” because the backend has no total count; wire and manually check this in Task 3.
- Responsive tables and dialogs must remain usable on narrow screens; check responsive classes and production build in Task 3.

---

### Task 1: Typed News API

**Files:**
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/api/index.ts`
- Create: `frontend/src/api/news.ts`
- Test: `frontend/tests/news-api.test.ts`

**Interfaces:**
- Produces: `AdminNews`, `NewsPayload`, `NewsStatus`, `NewsListResponse`.
- Produces: `listNews(page, limit, status?)`, `createNews(payload)`, `updateNews(slug, payload)`, `deleteNews(slug)`, and `getNewsErrorMessage(error)`.

- [ ] **Step 1: Write failing Node tests** for query construction, slug encoding, request methods/payloads, JSON error extraction, and fallback errors. Use a stubbed `fetch`; compile the test and source with `tsc` into `/tmp/opencode/mandat-news-tests` and run `node --test`.
- [ ] **Step 2: Run tests and verify failure** because `src/api/news.ts` does not exist.
- [ ] **Step 3: Add API types and implementation.** Default the base URL to `http://localhost:8080`, allow `VITE_API_BASE_URL`, call `/api/admin/news`, encode path/query values, parse `{ error: { message } }`, and treat non-2xx responses as errors.
- [ ] **Step 4: Re-run the API tests and verify they pass.**

### Task 2: News Editor Dialog

**Files:**
- Create: `frontend/src/components/admin/NewsEditorDialog.tsx`

**Interfaces:**
- Consumes: `AdminNews`, `NewsPayload`, `getNewsErrorMessage` from Task 1.
- Produces: controlled `NewsEditorDialog` props for open state, optional edited article, busy state, cancel, and `onSubmit(payload)`.

- [ ] **Step 1: Define the failing validation check** with cases: blank title/category rejected, draft without text accepted, published news without text rejected, and language limited to 10 characters. Keep this as an exported pure `validateNewsPayload` helper tested in `frontend/tests/news-api.test.ts` if shared, otherwise colocate a pure helper in a new `news-validation.ts` and test it there.
- [ ] **Step 2: Run the validation tests and verify failure.**
- [ ] **Step 3: Implement the controlled dialog** with title, category, description, text, language, status, featured switch, and image URL. Initialize from the selected article, preserve server-owned fields, disable submission while busy, and show field/general errors in Ukrainian.
- [ ] **Step 4: Run validation tests and verify they pass.**

### Task 3: Admin Page and Route

**Files:**
- Create: `frontend/src/pages/Admin.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/.env.example` (create if absent)

**Interfaces:**
- Consumes: API functions and editor dialog from Tasks 1–2.

- [ ] **Step 1: Implement the page** with a compact sidebar/header, “Керування новинами” heading, current-page counts, status filter, local title search, refresh, news table, loading/empty/error states, edit/delete actions, and previous/next pagination with limit 20.
- [ ] **Step 2: Wire CRUD state.** Creating/updates refresh the list and close the dialog; deletion requires an alert-dialog confirmation; mutations expose busy states and Ukrainian success/error feedback.
- [ ] **Step 3: Register `/admin`** without changing `/`; document `VITE_API_BASE_URL` in `.env.example`.
- [ ] **Step 4: Verify** with API/validation tests, `npm run lint`, and `npm run build`; all must complete without errors. Do not commit.
