# News Search Frontend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Interactive header search with a live suggestion dropdown that navigates to `/news/:slug`, on desktop and mobile.

**Architecture:** New `searchPublicNews` client hits `GET /api/news/search`; new `SearchBox` component (debounced input + listbox dropdown + keyboard nav) is embedded in `Header` for desktop and behind a magnifier toggle for mobile. No new dependencies.

**Tech Stack:** React 19, TypeScript, react-router v7, Tailwind `mandat-*` theme, node:test for tests.

**Spec:** `/home/aix/GolandProjects/mandat/docs/superpowers/specs/2026-11-03-news-search-design.md`

**Backend dependency:** needs the backend plan's `GET /api/news/search?q=&limit=` returning `{items, page, limit}` of public cards. Frontend Task 1 can be developed against the specced contract before the backend lands.

## Global Constraints

- TypeScript only, no `any`; function components; `@/` imports; styling via Tailwind `mandat-*` classes + `cn()`; never hardcode hex colors.
- Do NOT edit `src/components/ui/*` manually or `vite.config.ts` / `tailwind.config.js` without need.
- `npm run lint` and `npm run build` green before done.
- Tests live in `tests/`: API tests compile via per-test tsconfigs in `tests/testdata/tsconfig.<name>.json` (outDir `/tmp/opencode/mandat-<name>-tests`) and run with `node --test`; tsx suites additionally need `NODE_PATH=/home/aix/GolandProjects/mandat/frontend/node_modules` prefixed, otherwise node cannot resolve react/jsx-runtime from the /tmp outDir; component tests use `renderToStaticMarkup` (no DOM runner available — assert initial static markup only; interactive behavior is verified manually in the dev server).
- Tests (`tests/*`) are written only by the `test-author` subagent; primary never edits them.
- NEVER commit or push without the user's explicit command (repo rule overrides the usual per-task commit step: end tasks with a status report instead).

## Review Focus

- Typing, clearing, then retyping quickly never shows stale suggestions from the earlier query, because in-flight requests are aborted.
- Pressing Enter with an empty suggestion list does nothing instead of navigating somewhere wrong.
- Opening the mobile search row does not shift or overlap the 72px header bar content.
- A failed search request hides the dropdown without any error flash or console spam.
- Selecting a suggestion clears the field so reopening search starts blank.

## File Structure

- Modify `frontend/src/api/news.ts`: add `searchPublicNews`.
- Create `frontend/src/components/SearchBox.tsx`: the interactive search box + dropdown.
- Modify `frontend/src/sections/Header.tsx`: desktop field + mobile magnifier toggle + expandable row.
- Modify `frontend/README.md`: search behavior section.
- Tests (test-author): `frontend/tests/search-api.test.ts` + `frontend/tests/testdata/tsconfig.search-api.json`; `frontend/tests/search-box.test.tsx` + `frontend/tests/testdata/tsconfig.search-box.json`.

## Task 1: API client

**Files:**
- Modify: `frontend/src/api/news.ts`
- Test: `frontend/tests/search-api.test.ts`, `frontend/tests/testdata/tsconfig.search-api.json` (test-author)

**Interfaces:**
- Consumes: existing `request()` helper and `PublicNewsListResponse` in `src/api/news.ts`.
- Produces: `export function searchPublicNews(query: string, limit?: number): Promise<PublicNewsListResponse>` — `GET {PUBLIC_NEWS_API_URL}/search?q=<encoded>&limit=<n>`, default limit 7.

- [ ] **Step 1: test-author writes the failing client test** — `searchPublicNews builds the search URL with encoded q and default limit 7` (stub `fetch`, assert URL ends with `/search?q=%D0%BA%D0%B8%D1%97%D0%B2&limit=7` and `credentials: 'include'`), `searchPublicNews passes an explicit limit`, `searchPublicNews surfaces the server error message` (400 → rejects with `Invalid input`).
- [ ] **Step 2: Run to verify it fails** — `npx tsc -p tests/testdata/tsconfig.search-api.json && node --test /tmp/opencode/mandat-search-api-tests`, expected FAIL (function undefined).
- [ ] **Step 3: Primary implements** — in `src/api/news.ts` add `export function searchPublicNews(query: string, limit = 7): Promise<PublicNewsListResponse>` building `URLSearchParams({q: query, limit: limit.toString()})` and delegating to `request<PublicNewsListResponse>(PUBLIC_NEWS_API_URL, \`/search?${...}\`)`.
- [ ] **Step 4: Run to verify green** — same compile + `node --test` command, expected PASS.

## Task 2: SearchBox component and Header integration

**Files:**
- Create: `frontend/src/components/SearchBox.tsx`
- Modify: `frontend/src/sections/Header.tsx`
- Test: `frontend/tests/search-box.test.tsx`, `frontend/tests/testdata/tsconfig.search-box.json` (test-author)

**Interfaces:**
- Consumes: `searchPublicNews` (Task 1), `useNavigate` from `react-router`, `Search`/`X` from `lucide-react`, `cn` from `@/lib/utils`.
- Produces: `export default function SearchBox({ autoFocus, onNavigate }: { autoFocus?: boolean; onNavigate?: () => void })` — controlled input with placeholder `Пошук новин…`; dropdown `role="listbox"` of `role="option"` items (title + small `category · date`); `Header` keeps its 72px grid, shows `SearchBox` inline from `md` up and a magnifier button below `md` that toggles a search row under the bar.

- [ ] **Step 1: test-author writes the static component test** — `SearchBox renders a search input with placeholder Пошук новин… and no listbox when empty` (render `SearchBox` via `renderToStaticMarkup` inside `MemoryRouter`; assert `type="search"` input present, `role="listbox"` absent). Interactivity (debounce, arrows, Enter, outside click) is NOT unit-testable with the available runner and is verified manually in Step 5.
- [ ] **Step 2: Run to verify it fails** — `npx tsc -p tests/testdata/tsconfig.search-box.json && node --test /tmp/opencode/mandat-search-box-tests`, expected FAIL (module missing).
- [ ] **Step 3: Primary implements `SearchBox`** — state `value/items/open/activeIndex`; `useEffect` debounce 250 ms: trimmed empty → clear items + close, no request; else `AbortController` (cancel previous via ref), `searchPublicNews(debounced, 7)`, stale responses ignored via request id, errors → hide list silently; keyboard on input: `ArrowDown/ArrowUp` move `activeIndex` (open list), `Enter` → navigate active ?? first item, `Escape` → close; option click → navigate; container `ref` + `pointerdown` outside → close; on navigate: `navigate('/news/' + slug)`, clear value, close, call `onNavigate?.()`. Markup: wrapper `relative`, input keeps the existing header classes (`h-10 px-4 text-[13px] bg-mandat-bg border border-mandat-line focus:border-mandat-red-bright`), dropdown `absolute` panel with `bg-mandat-surface border border-mandat-line`, `role="listbox"`, `aria-expanded`, options `role="option"` + `aria-selected`.
- [ ] **Step 4: Primary integrates into `Header`** — replace the decorative input + `#news` anchor in the `md+` block with `<SearchBox />` (same width behavior: `hidden xl:block w-[220px]` preserved via wrapper classes); below `md`, add a magnifier `button` (toggles `mobileOpen` state, icon swaps `Search`/`X`) rendering a full-width row under the 72px bar with `<SearchBox autoFocus onNavigate={close} />`.
- [ ] **Step 5: Manual verification in dev server** — `npm run dev`, open `http://localhost:3000/`: type one letter → suggestions narrow per keystroke; click → article opens and field clears; Enter → first suggestion opens; Enter with an empty list → nothing happens; Esc/outside click → closes; fast type-clear-retype → no stale suggestions appear; disconnect backend (stop compose `app`) → typing shows nothing, no crash; mobile viewport → magnifier toggles the row without shifting the 72px bar. (This step is the pinned verification for the Review Focus lines above: no DOM runner exists for interactive unit tests.)
- [ ] **Step 6: Run checks** — `npm run lint` and `npm run build` clean; rerun both frontend test suites green.

## Task 3: Docs and final verification

**Files:**
- Modify: `frontend/README.md`.

- [ ] **Step 1: Primary updates README** — replace the search-related lines if needed: header search shows live title suggestions from the first letter (backend `GET /api/news/search`), click opens the article, Enter opens the first suggestion, empty/error hides the list, mobile via the magnifier toggle.
- [ ] **Step 2: Run full checks** — `npm run lint`, `npm run build`, both `tsc` + `node --test` suites green.
- [ ] **Step 3: Report done** — summarize changes per file, do NOT commit (repo rule).
