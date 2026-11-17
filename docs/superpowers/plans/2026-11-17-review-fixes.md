# Backend review fixes (reviewProject.md)

Source: `backend/reviewProject.md` (35 items). User decisions (2026-11-17):
- all 35 items in scope;
- item 5: slug is fixed at creation and never changes on update;
- item 18: moderators keep DELETE — no change;
- item 16: `has_more` may be added to list responses;
- item 23: the `status` field in error JSON stays — only docs are aligned;
- item 14: news_viewers cleanup is NOT approved (destructive) — deferred.

Tests are written by `test-author`; production code by the primary session.
Integration tests (PostgreSQL/Redis) stay skipped unless their env var is set.

---

## Task 1 — mechanical renames (items 32, 33, 34)

- Go field `ImageUrl` → `ImageURL` in `models.News`, `service.NewsInput`, `dto.NewsRequest`,
  `dto.NewsResponse`, `dto.PublicNewsResponse`, `dto.PublicNewsDetailResponse`. JSON tags unchanged (`image_url`).
- Package `pkg/storage/r2` moves to `internal/storage/r2` (package name `r2` unchanged).
- New package `pkg/imagetype` — the single table of supported cover formats:
  - `func Sniff(data []byte) string` — `http.DetectContentType` over the first 512 bytes;
  - `func MIMEForExt(ext string) (string, bool)` — case-insensitive; jpg/jpeg→image/jpeg, png, webp, gif;
  - `func ExtForMIME(mime string) (string, bool)` — image/jpeg→"jpg", image/png→"png", image/webp→"webp", image/gif→"gif".
  `r2`, `imgdownloader` and `service` use it instead of their private copies.

Tests: update every test for the field rename; new `pkg/imagetype/imagetype_test.go`.

## Task 2 — transport, auth, rate limits, sessions (items 1, 2, 9, 10, 11, 12, 17, 22, 27, 28, 29)

Routes (`internal/transport/http/routes`): constants `AdminNews = "/api/admin/news"`,
`AdminBase = "/api/admin"`, `Auth = "/api/auth"`, `PublicNews = "/api/news"`, `Health = "/health"`.
Handlers and middleware use them instead of duplicated literals.

Server-level middleware move into package `httptransport` (unexported): CORS, body-size limit,
plus two new ones. `middleware` package then imports `httptransport.WriteError` (no cycle).
- CORS `AllowHeaders`: `Origin, Content-Type, Accept` (no `Authorization`, item 22).
- Security headers (item 9), on every response: `X-Content-Type-Options: nosniff`,
  `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`,
  `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`;
  `Strict-Transport-Security: max-age=31536000; includeSubDomains` only when `HTTPConfig.SecureCookies`;
  `Cache-Control: no-store` for paths under `/api/admin` and `/api/auth`.
- Origin guard (item 10): for POST/PUT/PATCH/DELETE under `/api/admin` and `/api/auth`:
  allowed when `Origin` is absent (non-browser client), is listed in `HTTPConfig.AllowedOrigins`,
  or its host equals the request `Host`; otherwise `403` with the standard error body.

Errors (`internal/errs`, `internal/transport/http/errors.go`):
- `errResps` becomes an ordered slice (item 12); first match wins.
- New sentinel `errs.ErrUnauthenticated` → 401 "Authentication required".
  `errs.ErrUnauthorized` stays 401 "Invalid login or password" (failed login).
- `errs.ErrTooManyRequests` → 429 "Too many requests" (item 11).
- New `type RetryAfterError struct{ Wait time.Duration }`, `func NewRetryAfterError(wait time.Duration) *RetryAfterError`,
  `Unwrap() error` → `ErrTooManyRequests`. `WriteError` sets `Retry-After` (seconds, rounded up, min 1)
  when the error chain holds a `*RetryAfterError`.
- All middleware error responses go through `httptransport.WriteError` (item 28):
  `RequireAuth` → `ErrUnauthenticated` (or internal 500 for non-auth failures),
  `RequireRoles` → `ErrUnauthenticated` / `ErrForbidden`, `RateLimit` → `NewRetryAfterError(wait)`.

Rate limits (item 2):
- Delete `middleware/login_rate_limit.go` (in-memory, capped at 10 IPs).
- `service.NewScopedRateLimitService(store, logger, cfg, scope string) *RateLimitService` — keys
  `<scope>:<key>`; `NewRateLimitService` keeps scope `global`.
- `(*RateLimitService).AllowKey(ctx, key string) (time.Duration, error)` — key used verbatim
  (no IP masking); `Allow(ctx, clientIP)` = `AllowKey(ctx, clientKey(clientIP))`.
- POST `/api/auth/login` is guarded by `middleware.RateLimit(loginIPLimiter)` (scope `login:ip`).
- `AuthService` checks a per-login limit (scope `login:user`) before bcrypt;
  over the limit → error chain holding `*errs.RetryAfterError` (429 + Retry-After).
  Key is `strings.ToLower(strings.TrimSpace(login))`. A nil limiter disables the check.
- Config `login_rate_limit` (`service.RateLimitConfig`): defaults enabled, 5 requests, 5m window.

Auth service:
- `NewAuthService(repo, sessions, limiter LoginAttemptLimiter, logger, sessionTTL)` where
  `type LoginAttemptLimiter interface { AllowKey(ctx context.Context, key string) (time.Duration, error) }`.
- Item 1: the dummy hash is a real bcrypt hash: `dummyPasswordHash = sync.OnceValue(func() []byte)`
  generated with `bcrypt.DefaultCost`; unknown logins run a full bcrypt compare.
- Item 17: `SessionStore.DeleteUserSessions(ctx, userID uuid.UUID) error`;
  `(*AuthService).LogoutAll(ctx, userID uuid.UUID) error`.
  Redis keeps a per-user set `admin:user:<uuid>:sessions` of token-hash hex values.
- Handler: `POST /api/auth/logout-all` (authenticated) → 204, clears the session cookie.
- `AuthHandler.Me` uses `middleware.CurrentUser` (item 27).

## Task 3 — SSRF in the image downloader (item 3)

- `type resolver interface { LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) }`.
- `NewDownloader(cfg)` uses `net.DefaultResolver`; unexported `newDownloader(cfg Config, r resolver) *Downloader` for tests.
- The dialer resolves once, rejects the host if any address is not public (unless `AllowPrivate`),
  and dials the resolved IP (`netip.AddrPortFrom`) — never the hostname again (DNS rebinding).
- `isPublicAddr(netip.Addr) bool` also rejects: 0.0.0.0/8, 100.64.0.0/10, 192.0.0.0/24, 192.0.2.0/24,
  198.18.0.0/15, 198.51.100.0/24, 203.0.113.0/24, 240.0.0.0/4, ::/128, 64:ff9b::/96, 64:ff9b:1::/48,
  100::/64, 2001::/32, 2001:db8::/32, 2002::/16, fc00::/7, fe80::/10; IPv4-mapped IPv6 is unmapped first.

## Task 4 — configuration and runtime (items 7, 8, 19, 20, 21)

- `http.write_timeout` default 30s; config fails when `download.timeout >= http.write_timeout`.
- Env overrides: `HTTP_SECURE_COOKIES` (bool; invalid → error), `HTTP_ALLOWED_ORIGINS` (comma-separated).
- Config fails when `secure_cookies` is false and an allowed origin is not local
  (`localhost`, `127.0.0.1`, `[::1]`); every origin must be an http(s) URL with a host.
- `rates.refresh_interval` (default 24h, min 1m) and `rates.http_timeout` (default 10s, > 0).
- `login_rate_limit` section (see Task 2), validated like `rate_limit`.
- `main.go`: background goroutines tracked by a `sync.WaitGroup`, awaited before Redis/pool close.
- Compose: db/redis ports bound to `127.0.0.1`, `restart: unless-stopped`; Dockerfile `HEALTHCHECK`.

## Task 5 — news service (items 4, 5, 6, 13, 16, 24, 25, 26, 35)

- Item 5: `Update` keeps `existing.Slug`.
- Item 4: cover keys are unique per upload: `covers/<slug>-<8 lowercase hex>.<ext>`.
  On a failed DB update only the new key is deleted; the old cover is deleted only after success.
- Item 6: cover bytes are read (≤ 10 MB), sniffed and matched against the extension BEFORE any write
  (Create and Update). Mismatch / non-image → `ValidationError` (field `cover`) → 400, nothing stored.
  If the upload fails after the row is inserted, Create deletes the row and returns an error
  (`ErrServiceUnavailable` in the chain → 503). If saving the URL fails, the object and the row are removed.
- Item 13: expected errors (`ErrNewsNotFound`, `ErrInvalidInput`, `ErrPayloadTooLarge`, `ErrConflict`)
  are not logged as errors; unexpected ones are logged with `fc`.
- Item 16: `models.ListFilter{Status, Category, Limit, Offset}` (Page removed); the service asks the repo
  for `limit+1` rows at `offset=(page-1)*limit`, trims, and sets `ListResult.HasMore`.
  `dto.ListResponse` / `dto.PublicNewsListResponse` gain `has_more`.
- Items 24–26, 35: `collectNews` helper in the repo, `normalizePage` helper, `Update` split into helpers,
  `NewNewsService` comment fixed.

## Task 6 — search index (item 15)

Migration `00012_news_title_trgm.sql`: `CREATE EXTENSION IF NOT EXISTS pg_trgm`;
`CREATE INDEX IF NOT EXISTS idx_news_title_trgm ON news USING gin (title gin_trgm_ops)`.

## Task 7 — style and docs (items 23, 30, 31)

Parameter formatting and function ordering per backend/AGENTS.md §3; AGENTS.md §5 error example
shows `status`; §10 documents the new cover key format.
