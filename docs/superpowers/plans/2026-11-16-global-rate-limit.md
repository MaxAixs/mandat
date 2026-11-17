# Global Per-IP Rate Limit (Redis) Implementation Plan

> **For agentic workers:** primary session = Claude Code (writes production code); tests = opencode `test-author`
> (Muse Spark), review = opencode `reviewer` (Muse Spark). Steps use checkbox (`- [ ]`) syntax.

**Goal:** Limit every API client to N requests per window per IP, counters in Redis, real client IP taken
from Cloudflare's `CF-Connecting-IP` only when the request comes from a trusted proxy.

**Spec (agreed in chat 2026-11-16):** scope = one global per-IP limit on the whole API (login limiter is out
of scope); storage = Redis (shared across instances, survives restarts); production sits behind Cloudflare.

**Pinned values:**
- Default limit 300 requests / 1 minute per client (configurable `rate_limit.requests`, `rate_limit.window`).
  300 not 120: mobile carriers in Ukraine use CGNAT, many readers share one IPv4.
- Fixed window starting at the client's first request; one atomic Redis Lua call per request
  (`INCR`, `PEXPIRE` on the first hit, return count + `PTTL`). Key: `ratelimit:global:<client>`.
- Client key: IPv4 as is; IPv6 truncated to its `/64` prefix (one subscriber owns a whole /64);
  an unparsable value is used verbatim.
- Over the limit → `429`, body in the existing wire format
  `{"error":{"status":429,"message":"Too many requests"}}`, header `Retry-After: <seconds>` (ceil, min 1).
- Redis failure → **fail open** (request allowed), error logged in the service with `fc`.
- Not limited: `OPTIONS` (CORS preflight) and `/health/*` (probes).
- Client IP: `http.trusted_proxies` (CIDR/IP list, default empty = trust nobody → `RemoteAddr`),
  `http.client_ip_headers` (default `["CF-Connecting-IP"]`). Fixes today's spoofable `X-Forwarded-For`,
  which also protects the existing login limiter.
- `rate_limit.enabled` (default `true`) allows switching it off without a deploy of code.

## Global Constraints

- Backend only; never touch `frontend/`. `gofmt`, `go vet ./...`, `go test ./...` pass.
- `const fc` + `%w` wrapping in service/repository; logging only in the service.
- `*_test.go` only by `test-author`. No commits/pushes without the user's command.
- No destructive Redis/DB commands. Unit tests use fakes, no live Redis.

## Task 1 — Redis store

`internal/repository/redis/rate_limit.go`

```go
type evalCmdable interface {
    Eval(ctx context.Context, script string, keys []string, args ...any) *goredis.Cmd
}
type RateLimitStore struct{ client evalCmdable }
func NewRateLimitStore(client *goredis.Client) *RateLimitStore
// Hit counts one request for key inside window; returns the count and the time left in the window.
func (s *RateLimitStore) Hit(ctx context.Context, key string, window time.Duration) (int64, time.Duration, error)
```
Tests (fake `evalCmdable`): key gets `ratelimit:` prefix; window passed in ms; count/TTL parsed from
`[]any{int64,int64}`; Redis error wrapped; malformed reply (wrong type/length) → error; non-positive window → `ErrInvalidInput`.

## Task 2 — Service

`internal/service/rate_limit.go`

```go
type RateLimitStore interface {
    Hit(ctx context.Context, key string, window time.Duration) (int64, time.Duration, error)
}
type RateLimitConfig struct {
    Enabled  bool          `mapstructure:"enabled"`
    Requests int64         `mapstructure:"requests"`
    Window   time.Duration `mapstructure:"window"`
}
func NewRateLimitService(store RateLimitStore, logger *slog.Logger, cfg RateLimitConfig) *RateLimitService
// Allow returns errs.ErrTooManyRequests (wrapped) and the wait time when clientIP is over the limit.
func (s *RateLimitService) Allow(ctx context.Context, clientIP string) (time.Duration, error)
```
Tests: under/at limit → nil; limit+1 → `ErrTooManyRequests` + TTL as retry-after; store error → nil (fail open);
disabled → store never called; IPv6 `2001:db8:1:2:aaaa::1` and `2001:db8:1:2:bbbb::2` hit the same key
`global:2001:db8:1:2::/64`; IPv4 key `global:203.0.113.7`; garbage `"not-an-ip"` used verbatim.

## Task 3 — Middleware

`internal/transport/http/middleware/rate_limit.go`

```go
type RateLimiter interface {
    Allow(ctx context.Context, clientIP string) (time.Duration, error)
}
func RateLimit(limiter RateLimiter) gin.HandlerFunc
```
Tests: allowed → next handler runs; `ErrTooManyRequests` → 429 + JSON body + `Retry-After` (1.2s → `2`, 0 → `1`);
other error → request passes (fail open); `OPTIONS` and `/health/live` never call the limiter;
limiter receives `c.ClientIP()`.

## Task 4 — Server, client IP, config, wiring

- `HTTPConfig` += `TrustedProxies []string`, `ClientIPHeaders []string`.
- `httptransport.New(cfg, middlewares []gin.HandlerFunc, registrars ...Registrar) (*Server, error)`:
  `SetTrustedProxies(cfg.TrustedProxies)` (error returned), `RemoteIPHeaders = cfg.ClientIPHeaders` when set,
  global middlewares after CORS (so a 429 still carries CORS headers) — order:
  Recovery → LimitBodySize → CORS → middlewares → routes.
- `config`: `RateLimit service.RateLimitConfig` (`rate_limit`), defaults (enabled, 300, 1m,
  `http.client_ip_headers=[CF-Connecting-IP]`), validation: `requests > 0`, `window >= 1s`, every
  trusted proxy is a valid IP or CIDR.
- `cmd/main.go`: build store → service → `middleware.RateLimit` → pass to `New`.
- `config.yaml`, README: document Cloudflare setup (list Cloudflare ranges in `trusted_proxies` in prod).
Tests: existing `New(...)` call sites updated to the new signature; trusted proxy honours `CF-Connecting-IP`,
untrusted remote ignores it (spoofing); invalid CIDR → error; config defaults/validation.

## Review Focus

- Spoofing: with empty `trusted_proxies`, `CF-Connecting-IP`/`X-Forwarded-For` must not change the key.
- Redis down: every request still served, errors logged, no panic.
- Key without TTL can never appear (INCR+PEXPIRE atomic in Lua).
- Health probes and preflights never throttled.
