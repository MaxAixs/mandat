# News search with live suggestions — design

Date: 2026-11-03. Status: pending approval.

## Intent

A reader has already seen an article, wants to come back to it, and looks
for it through the search field in the header. Suggestions appear while
typing; a click opens the article directly. Works on desktop and on mobile.

## Behavior (agreed with the user)

- Suggestions appear starting from the first typed character; the list
  narrows with every next one.
- Match against titles only, among published articles only, newest first.
- A click on a suggestion opens the article directly (`/news/:slug`); there
  is no separate results page.
- Enter opens the first suggestion in the list.
- No matches or a request error — show nothing, the list simply hides.
- Search is available on mobile too (the field is currently hidden below `md`).

## Non-goals and anti-goals

- No searching article bodies, descriptions, or categories. No fuzzy search,
  query history, or search analytics.
- No separate `/search` results page.
- Failure even if it technically works: showing drafts; hitting the server
  on every keystroke with no debounce; breaking the current header layout.

## Constraints

- Backend: clean architecture and `backend/AGENTS.md` conventions — `fc`
  constants, sentinel errors via `internal/errs`, logging in the service
  layer only, parameterized queries via pgx, `gofmt` + `go vet`.
- Frontend: `frontend/AGENTS.md` conventions — `mandat-*` classes, `cn()`,
  `@/` imports, no new dependencies, green `npm run lint` and
  `npm run build`.
- Shared repo, but separate commits: one for `backend/`, one for
  `frontend/` (prefixes `[+]`/`[*]`/`[-]`, no scope). Commit only on an
  explicit command.
- Repository integration tests wipe the `news` table, so they may run only
  against a dedicated test database and only with the user's explicit
  permission for each destructive operation.

## Backend (decisions)

- New public endpoint `GET /api/news/search?q=<text>&limit=<n>`.
  The static `/search` route coexists with `/:slug` (in Gin, static
  segments win over params); no auth and no viewer cookie needed.
- `q` is trimmed; an empty `q` returns `200` with an empty list, not 400
  (friendlier for live input). `q` is capped at 20 characters, longer —
  `400 Invalid input`. `limit` defaults to 7: above 20 is clamped to 20,
  below 1 or non-numeric is `400` (same convention as `ListPublished`).
  The response envelope is the same `{items, page, limit}` as the feed
  (`PublicNewsListResponse`).
- Chain: `PublicNewsHandler.Search` → `NewsService.SearchPublished`
  (validation, trim, limit cap, logging) → `NewsRepo.SearchPublished`
  (plain SQL, no business logic).
- SQL: `title ILIKE '%' || $1 || '%'` with `published` status; `%`, `_`,
  `\` in the query are escaped (`ESCAPE '\'`). Case and Ukrainian letters
  as plain `ILIKE` does out of the box, no apostrophe normalization in v1.
  Searching from the first character is a hard requirement; no `pg_trgm`
  index — a future optimization if the table grows. Ordering
  `published_at DESC NULLS LAST, id DESC`, `LIMIT $2`.
  Items are the same `PublicNewsResponse` cards as the feed.
- An article with slug `search` would be shadowed by the search route —
  accepted edge case, no slug blacklist. A regression test proves
  `/api/news/search` hits search, not the detail handler.
- Search itself never counts views. Following a suggestion is an ordinary
  `/news/:slug` open (same page, same viewer cookie and view counting
  as a direct open).
- Backend README: add a row to the Public API table.

## Frontend (decisions)

- New component `src/components/SearchBox.tsx`, embedded in `Header`.
- Controlled input, 250 ms debounce after the last keystroke, in-flight
  request cancellation via `AbortController`. Request only when the trimmed
  text is non-empty; otherwise the list hides.
- On desktop — the existing field in the header; below `md` — a magnifier
  icon in the mobile header that expands a search row under the header
  (agreed with the user).
- `listbox` dropdown: title plus small category/date. Click —
  `navigate('/news/:slug')`, clear the field, close the list.
  Keyboard: up/down arrows select, Enter opens the active (or first)
  suggestion, Esc and outside click close.
- `src/api/news.ts`: new `searchPublicNews(query, limit)` function.
- Frontend README: briefly describe the search behavior.

## Builder's calls (mine, check while reading)

- Default `limit` 7, cap 20, `q` cap 20 chars, debounce 250 ms.
- A failed request hides the list with no error UI (background behavior).
- No match highlighting in v1 (optional).
- Exact dropdown and mobile-row styling within the `mandat-*` theme.

## Tests (written by test-author, primary does not touch them)

- Backend: service unit tests (empty `q`, limit cap, invalid limit),
  handler tests (statuses, response shape, empty `q`), a repository
  integration test against real PostgreSQL (test database only,
  only with permission).
- Frontend: component tests (suggestions appear, Enter opens the first one,
  Esc/outside click close, navigation to `/news/:slug`).
- Before done: `go test ./...`, `go vet ./...`,
  `npm run lint`, `npm run build`.
