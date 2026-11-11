# Redis-backed Exchange Rates (3-day window + retries) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the in-memory exchange cache with Redis persistence (3-day TTL), fetch NBU for today with 3 backoff retries, walk back at most 3 days, and serve requests from Redis only.

**Architecture:** New `ExchangeStore` (service-owned interface, Redis implementation beside `SessionStore`) holds dated rate snapshots with a 72h TTL plus a `latest` pointer. `ExchangeService` keeps the NBU `Fetcher`, gains retry + 3-day walk-back in `Refresh`, and `GetRates` becomes a fast Redis-only read (never touches NBU, never blocks on retries). Wiring in `cmd/main.go` builds the store from the existing Redis client and moves the startup warmup to the background.

**Tech Stack:** Go, go-redis/v9 (already in go.mod), `encoding/json` payloads, existing `pkg/redis` client.

**Spec:** In-chat agreement 2026-11-11 (no separate design doc — small change, approved in chat). Pinned spec values: ask NBU for TODAY only — 1 attempt + 3 retries with pauses 5s, 10s, 15s (each next +5s), abort waits on `ctx.Done()`; on total failure walk back NBU single-attempt days yesterday, day-2, day-3, then stop; first complete day (EUR+USD+PLN all present) is saved to Redis; requests never fetch — missing/expired Redis data returns 503 (`ErrServiceUnavailable`); Redis entries live 3 days (TTL 72h, auto-expire, no cleanup job); data older than 3 calendar days is never served even if somehow present; refresh runs in background at startup (non-blocking) and every 24h.

**Change 2026-11-12 (user request, after plan approval): NBU walk-back REMOVED.** `Refresh` fetches today only (1+3 attempts); past NBU dates are never requested because Redis already holds the 3-day history and `GetRates` serves the nearest stored day. Obsolete walk-back tests replaced with `NoWalkBackToNBU` + `IncompleteTodayFallsBackToRedis` pins (Task 2).

## Global Constraints

- Backend only. `gofmt`, `go vet ./...`, `go test ./...` pass; every service/repository function starts with `const fc = "..."` and wraps errors with `%w`; logging only in service via `log/slog`; handlers untouched; no `errResps` changes (all failures stay `ErrServiceUnavailable` → 503).
- Test files (`*_test.go`) are written ONLY by the `test-author` subagent — the implementer never edits them; each test step means "dispatch test-author with the exact test names and assertions".
- **NO COMMITS without the user's explicit command.** The standing project rule outranks this plan: every `Commit` step below means "stage the files, verify the diff, then STOP and ask the user" — never run `git commit` on your own initiative. (This exact failure happened before; the plan's commit steps are prepare-and-ask.)
- Never `push`. Never touch `frontend/`. Never run destructive DB/Redis commands (`FLUSHDB`, `DEL` outside tests) without explicit permission.
- NBU wire format unchanged: `?json&date=DD.MM.YYYY`, per-unit rate = `Amount / Units`; `BaseURL` stays `"https://bank.gov.ua/NBU_Exchange/exchange"`.
- New unexported test seams are allowed and expected (`now func() time.Time` exists; add `retryDelays`), set to spec defaults in the constructor; tests live in the same package so they may set them.

## Review Focus

- NBU down for weeks: daily background refresh burns 4×10s + 30s sleeps then stops, Redis TTL expires after 3 days, `GetRates` returns 503 — no unbounded growth, no stale-forever (test: fail-all fetcher + empty store → 503 in Task 2).
- Redis down (connection refused) on both read and write paths: reads → 503, writes → `Refresh` fails logged, background retries tomorrow; nothing panics (test: erroring store fake in Task 1/2).
- `latest` pointer present but dated entry expired/missing (TTL race): treated as absent → walk-back/503, never a nil dereference (test in Task 1).
- Stored data newer than service clock or clock skew across deploys: the 3-calendar-day window check uses the stored `Date` string, not save time, so future-dated data is served until it ages out rather than crashing (test in Task 2).
- Concurrent `GetRates` during a background `Refresh`: reads are lock-free Redis GETs, refresh is singleflight-guarded; no torn reads because each date key is written atomically with one `SET` (test: parallel `GetRates` in Task 2).

---

### Task 1: Redis ExchangeStore (repository)

**Files:**
- Create: `backend/internal/repository/redis/exchange.go`
- Test: `backend/internal/repository/redis/exchange_test.go` (via test-author, package `redis`)

**Interfaces:**
- Consumes: `*goredis.Client` (concrete, like `SessionStore`; constructor takes a narrow interface so tests use an in-memory fake):
```go
type redisCmdable interface {
    Set(ctx context.Context, key string, value any, expiration time.Duration) *goredis.StatusCmd
    Get(ctx context.Context, key string) *goredis.StringCmd
}
```
(`*goredis.Client` satisfies it; the fake builds cmds via `goredis.NewStatusResult` / `goredis.NewStringResult`.)
- Produces (service-owned contract, defined in Task 2 — this task defines the concrete type only):
```go
func NewExchangeStore(client redisCmdable) *ExchangeStore
func (s *ExchangeStore) Save(ctx context.Context, date time.Time, rates []*models.ExchangeRate) error
func (s *ExchangeStore) Latest(ctx context.Context) ([]*models.ExchangeRate, time.Time, error)
```
Keys: `exchange:rates:YYYY-MM-DD` (JSON `{date:"02.01.2006", items:[{code,rate}]}`) and `exchange:rates:latest` (the date key `YYYY-MM-DD`), both `SET` with TTL `exchangeRatesTTL = 72 * time.Hour` in one `Save`. `Latest`: `GET latest` → `goredis.Nil` means absent (return sentinel `errs.ErrExchangeRatesNotFound` — ADD it to `internal/errs/errs.go` in this task: `ErrExchangeRatesNotFound = errors.New("exchange rates not found")`; no `errResps` entry — the service maps it to 503 itself); then `GET` the dated key → `goredis.Nil`/corrupt JSON also means absent (same sentinel, never a partial result). Save serializes with `encoding/json`; any marshal/Redis error is wrapped `%w` with `fc`.

- [ ] **Step 1: Dispatch test-author to write the failing test**

```go
func TestExchangeStore_SaveThenLatest(t *testing.T) // in-memory redisCmdable fake; Save(day, EUR/USD/PLN) then Latest → same 3 rates in order + same calendar day; assert both keys carry TTL 72h (fake records expirations)
func TestExchangeStore_LatestAbsent(t *testing.T) // empty fake; Latest → errors.Is(err, errs.ErrExchangeRatesNotFound)
func TestExchangeStore_LatestPointerWithoutEntry(t *testing.T) // fake has latest pointer but no dated key; Latest → ErrExchangeRatesNotFound, no panic
func TestExchangeStore_SaveError(t *testing.T) // fake failing Set; Save → non-nil error (not the absent sentinel)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/repository/redis/ -run TestExchangeStore -v` (from `backend/`)
Expected: FAIL — `NewExchangeStore` undefined.

- [ ] **Step 3: Implement `ExchangeStore` in `backend/internal/repository/redis/exchange.go` + sentinel in `internal/errs/errs.go`**

Mirror `session.go` exactly (package `redis`, `fc` consts, `goredis.Nil` mapping, JSON record struct). Key helpers: `exchangeDayKey(date time.Time) string // "exchange:rates:" + date.Format("2006-01-02")`, `const exchangeLatestKey = "exchange:rates:latest"`, `const exchangeRatesTTL = 72 * time.Hour`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `gofmt -l internal/repository/redis internal/errs; go vet ./internal/repository/redis/ ./internal/errs/ && go test ./internal/repository/redis/ -v`
Expected: PASS.

- [ ] **Step 5: Prepare commit (DO NOT COMMIT — ask the user)**

```bash
git add backend/internal/repository/redis/exchange.go backend/internal/repository/redis/exchange_test.go backend/internal/errs/errs.go
git status --short
```
Then STOP and ask the user for the commit command.

### Task 2: Service rework — Redis reads, retries, 3-day walk-back

**Files:**
- Modify: `backend/internal/service/exchange.go` (rewrite refresh/read path; keep `selectRates`, `startOfDay`, `ExchangeFetcher`, `exchangeWantedCodes` untouched)
- Test: `backend/internal/service/exchange_test.go` (via test-author — REWRITE for the new contract; old in-memory tests no longer apply and are replaced)

**Interfaces:**
- Consumes: `ExchangeFetcher` (unchanged) + new service-owned contract:
```go
type ExchangeStore interface {
    Save(ctx context.Context, date time.Time, rates []*models.ExchangeRate) error
    Latest(ctx context.Context) ([]*models.ExchangeRate, time.Time, error)
}
```
(`*redis.ExchangeStore` from Task 1 satisfies it; tests use an in-memory fake with optional error injection.)
- Produces: `func NewExchangeService(fetcher ExchangeFetcher, store ExchangeStore, logger *slog.Logger) *ExchangeService` (store is a NEW required param — update the constructor); fields keep `now func() time.Time` (default `time.Now`) and add `retryDelays [3]time.Duration` defaulting to `{5s, 10s, 15s}` (tests override with milliseconds); drop in-memory `rates`/`ratesDate`/`mu` (Redis is the cache now); keep `refreshMu` singleflight around `Refresh`.
- Behavior (exact): `Refresh(ctx)` — no overall timeout (background-only; per-attempt HTTP timeout stays 10s in `main.go`): `fetchWithRetry(ctx, today)` (1 attempt + 3 retries, sleeps `retryDelays[i]` between, abort sleep on `ctx.Done()` → return ctx error wrapped; each failed attempt logged); on success with complete codes → `store.Save` (save error fails `Refresh`); on failure/incomplete → single attempts for yesterday, day-2, day-3 (no retries); first complete day saved wins; total failure → `ErrServiceUnavailable`. `GetRates(ctx)` — NEVER calls the fetcher: `store.Latest` → error/absent → `ErrServiceUnavailable`; present but `startOfDay(dataDate).Before(startOfDay(now − 2 days))` → `ErrServiceUnavailable` (3-calendar-day window); else serve as-is (order already EUR,USD,PLN from `selectRates` at save time).

- [ ] **Step 1: Dispatch test-author to (re)write the failing test**

```go
func TestGetRates_ServesRedisWithoutFetching(t *testing.T) // fetcher fail-all (assert calls==0), store preloaded with today full set → 3 rates EUR/USD/PLN in order, no error
func TestGetRates_RetryThenSuccess(t *testing.T) // fetcher fails twice then full set for today (retryDelays overridden to milliseconds) → success, fetcher.calls==3, store now holds today
func TestGetRates_WalkBackThreeDays(t *testing.T) // fail today/yesterday/day-2, full set day-3 → success with day-3 values; fetcher.calls == 1+3+3 (today attempts+retries, then 3 singles)
func TestGetRates_WalkBackStopsAfterThreeDays(t *testing.T) // fail-all fetcher (retryDelays ms) + empty store → ErrServiceUnavailable; fetcher.calls == 4+3 (no day-4 attempt)
func TestGetRates_StaleRedisReturns503(t *testing.T) // store preloaded with 4-day-old date, fetcher fail-all → ErrServiceUnavailable, rates empty
func TestGetRates_EmptyStoreReturns503(t *testing.T) // empty store, fetcher fail-all (retryDelays ms) → ErrServiceUnavailable
func TestGetRates_SaveFailureFailsRefresh(t *testing.T) // fetcher returns today full set, store.Save errors → Refresh error (call Refresh directly); GetRates after → ErrServiceUnavailable
func TestGetRates_ParallelReads(t *testing.T) // store preloaded; 16 parallel GetRates → all succeed, identical rates, fetcher.calls==0
func TestGetRates_FutureDatedRedisServed(t *testing.T) // store preloaded with tomorrow's date (clock skew), fetcher fail-all → served as-is, no error (window check only rejects old data)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run TestGetRates -v` (from `backend/`)
Expected: FAIL — `NewExchangeService` takes 2 args / store/retry seams missing (old tests replaced, new ones reference the new contract).

- [ ] **Step 3: Implement the reworked `ExchangeService` in `backend/internal/service/exchange.go`**

`fc` consts in `GetRates`, `Refresh`, `fetchWithRetry` (new unexported helper). Log each failed fetch/retry at error level with `fc`+`date`; log save failures. Keep `selectRates`/`startOfDay` as-is. Delete in-memory cache fields, `fresh()`, and the 8s overall timeout (background path no longer needs it; per-attempt 10s HTTP timeout remains).

- [ ] **Step 4: Run tests to verify they pass**

Run: `gofmt -l internal/service; go vet ./internal/service/ && go test ./internal/service/ -run TestGetRates -v`
Expected: PASS (all 9).

- [ ] **Step 5: Prepare commit (DO NOT COMMIT — ask the user)**

```bash
git add backend/internal/service/exchange.go backend/internal/service/exchange_test.go
git status --short
```
Then STOP and ask the user for the commit command.

### Task 3: Wiring — store construction, background startup refresh

**Files:**
- Modify: `backend/cmd/main.go` (wiring only)
- No new tests (covered by Task 1/2 suites + full suite); verify with `go test ./...`

**Interfaces:**
- Consumes: `redisrepo.NewExchangeStore(redisClient)` (Task 1), `service.NewExchangeService(nbuClient, exchangeStore, log)` (Task 2).
- Produces: unchanged `GET /api/exchange/rates` behavior; startup no longer blocks on NBU (startup warmup moves to a background goroutine so 3 retries × worst case never exceeds `startup_timeout: 15s`).

- [ ] **Step 1: Implement wiring in `backend/cmd/main.go`**

Replace the blocking warmup: `go func() { if err := exchangeService.Refresh(ctx); err != nil { log.ErrorContext(...) } }()` (same log message as today). Keep `go exchangeService.StartAutoRefresh(ctx, 24*time.Hour)` and the handler registration unchanged. No config changes (TTL/retry values are spec constants, not env).

- [ ] **Step 2: Run the full suite to verify**

Run: `gofmt -l ./cmd ./internal ./pkg; go vet ./... && go test ./...` (from `backend/`)
Expected: PASS, no gofmt output. Then live-check readiness (not a test, just the standard smoke): `go build ./...`.

- [ ] **Step 3: Prepare commit (DO NOT COMMIT — ask the user)**

```bash
git add backend/cmd/main.go
git status --short
```
Then STOP and ask the user for the commit command.
