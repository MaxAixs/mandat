# Fuel Prices Ticker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show fuel prices (A-95, A-92, A-98, diesel, LPG) as rich cards in the home ticker next to currency rates.

**Architecture:** Mirror the exchange pipeline exactly: `FuelService` (service-owned `FuelStore` + `HTTPClient` contracts) fetches `https://20.com.ua/api/v1/prices` averages with the same 3-retry policy, persists snapshots to Redis with a 72h TTL, serves `GET /api/fuel/rates` from Redis only; frontend adds a fuel API client + hook and appends fuel cards after currency cards in the same `Ticker`. No `pkg` changes (reuse `httpclient.Client.GetJSON`); no walk-back (fuel API has no per-date endpoint — current snapshot only).

**Tech Stack:** Go (existing `pkg/httpclient`, go-redis), React 19 + TypeScript (`@/api`, `@/hooks`, `TickerRate`).

**Spec:** In-chat agreement 2026-11-11 (no separate design doc — small change, approved in chat). Pinned spec values, verified live against the API: source `https://20.com.ua/api/v1/prices`, use `averages` object only (ignore `operators` and `changes`); display mapping — `A-95 = averages.a95`, `A-92 = averages.a92`, `A-98 = averages.a95_plus` (**user decision 2026-11-11**: a95_plus value shown labeled as А-98, nothing dropped), `Дизель = averages.diesel`, `Автогаз = averages.lpg`; order A-95, A-92, A-98, Дизель, Автогаз; backend response `{"base":"UAH","unit":"liter","date":"<meta.updated_at>","items":[{"code":"A95","rate":88.74},...]}` with code set exactly `{A95, A92, A98, DIESEL, LPG}`; item with null/missing average → that refresh fails like an incomplete NBU day (whole snapshot rejected, Redis keeps previous); refresh once per day in background + non-blocking startup warmup; `GetRates` Redis-only with the same 3-calendar-day window rule; empty/expired Redis → 503 (`ErrServiceUnavailable`, already in `errResps`); frontend cards show name + code + price with unit `₴/л`, order currencies first then fuel.

## Global Constraints

- Backend only touches `backend/`; frontend only touches `frontend/`; never mix both in one commit.
- Backend: `gofmt`, `go vet ./...`, `go test ./...` pass; every service/repository function starts with `const fc = "..."` and wraps errors with `%w`; logging only in service via `log/slog`; handlers thin, errors via `errResps` table only (no handler/errResps changes needed — 503 already mapped).
- Frontend: strict TS, imports via `@/` alias, Tailwind `mandat-*` classes only, `npm run lint` and `npm run build` pass; never edit `src/components/ui/*` manually.
- Test files (`*_test.go`, `*.test.ts`) are written ONLY by the `test-author` subagent — the implementer never edits them; each test step means "dispatch test-author with the exact test names and assertions".
- **NO COMMITS without the user's explicit command.** Every `Commit` step below means: stage the listed files, show `git status`, then STOP and ask the user. Never `push`.
- Read current code, not old plans: `internal/service/exchange.go` (post-refactor: NBU fetch lives IN the service as `fetchDay`, service owns `HTTPClient` interface, `*httpclient.Client` implements it, no `internal/exchange` package anymore) is the pattern to mirror — `internal/exchange/` does not exist.
- Reuse, don't duplicate: `models.ExchangeRate{Code, Rate, Date string}` is reused for fuel snapshots (documented as generic); `pkg/httpclient` is NOT modified.

## Review Focus

- `averages.a95_plus` (or any average) is `null`/missing in a fetch → whole snapshot rejected, Redis keeps previous day, no partial fuel table (test in Task 2).
- Fuel API returns HTTP 200 with an error-shaped or schema-changed body (missing `averages`) → decode/validation fails, treated as fetch failure with retries, never a zero-price card (test in Task 1).
- Redis `fuel:latest` pointer without dated entry (TTL race) → absent sentinel, 503, never a nil dereference (test in Task 1).
- Stored fuel snapshot older than 3 calendar days (NBU-style window) → 503 even if Redis TTL hasn't expired it (test in Task 2).
- Ticker with rates but no fuel (fuel backend down, 503) → currency cards still render, no layout break, empty fuel list renders nothing (test in Task 4).

---

### Task 1: Fuel Redis store (backend)

**Files:**
- Create: `backend/internal/repository/redis/fuel.go`
- Test: `backend/internal/repository/redis/fuel_test.go` (via test-author, package `redis`)
- Modify: `backend/internal/errs/errs.go` (append sentinel)

**Interfaces:**
- Consumes: narrow client interface — REUSE the existing unexported `redisCmdable` from `exchange.go`'s sibling file in the same package (do not redefine it; same `Set`/`Get` shape, `*goredis.Client` satisfies it).
- Produces: `func NewFuelStore(client redisCmdable) *FuelStore`; `func (s *FuelStore) Save(ctx context.Context, date time.Time, rates []*models.ExchangeRate) error`; `func (s *FuelStore) Latest(ctx context.Context) ([]*models.ExchangeRate, time.Time, error)`; sentinel `ErrFuelRatesNotFound = errors.New("fuel rates not found")` in `internal/errs/errs.go` (no `errResps` entry — service maps it to 503).
- Keys: `fuel:rates:YYYY-MM-DD` (JSON `{"date":"DD.MM.YYYY","items":[{"code","rate"}]}`, mirror the exchange record shape) + `fuel:latest` holding `YYYY-MM-DD`; BOTH `SET` with TTL `fuelRatesTTL = 72 * time.Hour`. `Latest`: `latest` miss / dated-key miss / corrupt JSON → `ErrFuelRatesNotFound`-wrapped; never partial results. Mirror `exchange.go` in the same package (fc consts, `goredis.Nil` mapping, `%w`).

- [ ] **Step 1: Dispatch test-author to write the failing test**

```go
func TestFuelStore_SaveThenLatest(t *testing.T) // in-memory fake; Save 5 fuel rates for a day → Latest returns same 5 in order + same calendar day; assert both keys carry TTL 72h
func TestFuelStore_LatestAbsent(t *testing.T) // empty fake → errors.Is(err, errs.ErrFuelRatesNotFound)
func TestFuelStore_LatestPointerWithoutEntry(t *testing.T) // pointer without dated key → ErrFuelRatesNotFound, no panic
func TestFuelStore_SaveError(t *testing.T) // failing Set → non-nil error, not ErrFuelRatesNotFound
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/repository/redis/ -run TestFuelStore -v` (from `backend/`)
Expected: FAIL — `NewFuelStore` undefined.

- [ ] **Step 3: Implement `FuelStore` in `backend/internal/repository/redis/fuel.go` + sentinel in `errs.go`**

Reuse `redisCmdable` from `exchange.go` (same package — no redefinition, no import cycle).

- [ ] **Step 4: Run tests to verify they pass**

Run: `gofmt -l internal/repository/redis internal/errs; go vet ./internal/repository/redis/ ./internal/errs/ && go test ./internal/repository/redis/ -v`
Expected: PASS.

- [ ] **Step 5: Prepare commit (DO NOT COMMIT — ask the user)**

```bash
git add backend/internal/repository/redis/fuel.go backend/internal/repository/redis/fuel_test.go backend/internal/errs/errs.go
git status --short
```
Then STOP and ask the user for the commit command.

### Task 2: FuelService + endpoint + wiring (backend)

**Files:**
- Create: `backend/internal/service/fuel.go`
- Test: `backend/internal/service/fuel_test.go` (via test-author, package `service`)
- Modify: `backend/internal/transport/http/dto/response.go` (append fuel DTO + mapper, reuse the 2-decimal rounding pattern)
- Create: `backend/internal/transport/http/handler/fuel.go`
- Test: `backend/internal/transport/http/handler/fuel_test.go` (via test-author, package `handler` like the exchange handler test)
- Modify: `backend/cmd/main.go` (build fuel store/service/handler, background startup warmup + `StartAutoRefresh`-style 24h ticker; mirror the exchange wiring lines)

**Interfaces:**
- Consumes: service-owned contracts (defined in `fuel.go`, mirroring exchange):
```go
type FuelStore interface {
    Save(ctx context.Context, date time.Time, rates []*models.ExchangeRate) error
    Latest(ctx context.Context) ([]*models.ExchangeRate, time.Time, error)
}
```
HTTP via the existing `HTTPClient` interface in the same `service` package (`GetJSON(ctx, url, out)` — reuse it, do not redefine; `*httpclient.Client` satisfies it). `main.go` passes the SAME shared client instance to both exchange and fuel constructors.
- Produces: `func NewFuelService(httpClient HTTPClient, store FuelStore, logger *slog.Logger) *FuelService` (same `now`/`retryDelays` seams as exchange: `now func() time.Time`, `retryDelays [3]time.Duration` default `{5s,10s,15s}`); `func (s *FuelService) GetRates` (Redis-only, 3-calendar-day window, expired/absent → `ErrServiceUnavailable`); `func (s *FuelService) Refresh` (fetch `https://20.com.ua/api/v1/prices` with today-retries, map averages → 5 codes in pinned order, any null/missing average → day rejected; save wins; total failure → `ErrServiceUnavailable`); `StartAutoRefresh` (same ticker shape); handler `NewFuelHandler(svc FuelService)` with `Register` mounting `GET /api/fuel/rates` → `200 {"base":"UAH","unit":"liter","date":"<meta.updated_at>","items":[{"code":"A95","rate":88.74},...]}` (rates rounded to 2 decimals in the DTO mapper).

- [ ] **Step 1: Dispatch test-author to write the failing tests**

Service (`fuel_test.go`):
```go
func TestGetFuelRates_ServesRedisWithoutFetching(t *testing.T) // preloaded store, fail-all HTTP fake → 5 rates A95/A92/A98/DIESEL/LPG in order, fetcher.calls == 0
func TestGetFuelRates_RetryThenSuccess(t *testing.T) // 2 failed HTTP calls then full averages → success, calls == 3, store has today
func TestGetFuelRates_NullAverageRejectsDay(t *testing.T) // averages with a95:null → Refresh fails ErrServiceUnavailable, nothing saved; GetRates serves preloaded previous day
func TestGetFuelRates_EmptyStoreReturns503(t *testing.T) // empty store + fail-all HTTP → ErrServiceUnavailable
func TestGetFuelRates_StaleSnapshotReturns503(t *testing.T) // 4-day-old stored day → ErrServiceUnavailable
```
Handler (`handler/fuel_test.go`, same harness as exchange handler test):
```go
func TestFuelHandler_GetRates(t *testing.T) // fake service returns 5 rates → 200, base UAH, unit liter, items in pinned order with 2-decimal rates
func TestFuelHandler_ServiceUnavailable(t *testing.T) // fake service ErrServiceUnavailable → 503
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run TestGetFuelRates -v` and `go test ./internal/transport/http/handler/ -run TestFuelHandler -v` (from `backend/`)
Expected: FAIL — `NewFuelService` / `NewFuelHandler` undefined.

- [ ] **Step 3: Implement `FuelService` (+ `fetchFuelDay` + `selectFuel`-style mapping with the pinned A98=a95_plus mapping and pinned order), DTO + mapper, `FuelHandler`, `main.go` wiring**

NBU-wire details to mirror: `models.ExchangeRate` reuse (`Code` = one of `A95 A92 A98 DIESEL LPG`, `Rate` = average, `Date` = `meta.updated_at` formatted `DD.MM.YYYY`); `fc` consts; `%w`; slog-only logging; Warn on absent snapshot, Error otherwise (same log-level rule as exchange). `main.go`: build `fuelStore` from the existing `redisClient`, share ONE `*httpclient.Client` instance between exchange and fuel (build once, pass to both constructors), background startup warmup (non-blocking), 24h `StartAutoRefresh`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `gofmt -l ./internal ./cmd; go vet ./... && go test ./...` (from `backend/`)
Expected: PASS, no gofmt output.

- [ ] **Step 5: Prepare commit (DO NOT COMMIT — ask the user)**

```bash
git add backend/internal/service/fuel.go backend/internal/service/fuel_test.go backend/internal/transport/http/dto/response.go backend/internal/transport/http/handler/fuel.go backend/internal/transport/http/handler/fuel_test.go backend/cmd/main.go
git status --short
```
Then STOP and ask the user for the commit command.

### Task 3: Fuel API client + hook (frontend)

**Files:**
- Create: `frontend/src/api/fuel.ts`
- Create: `frontend/src/hooks/use-fuel-prices.ts`
- Test: `frontend/tests/fuel-api.test.ts` (via test-author, `node:test` + existing alias/hook-driver harness)
- Modify: `frontend/src/api/mock.ts` (append `mockFuelRates: TickerRate[]`-shaped dev fallback — import type from `@/sections/Ticker`)

**Interfaces:**
- Consumes: same `request`-style fetch as `src/api/news.ts` (`http://localhost:8080`, `credentials: include`, identical error messages).
- Produces: `export interface FuelRateItem { code: string; rate: number }`, `export interface FuelRatesResponse { base: string; unit: string; date: string; items: FuelRateItem[] }`, `export async function fetchFuelRates(): Promise<FuelRatesResponse>` (GET `http://localhost:8080/api/fuel/rates`), `export function getFuelErrorMessage` (same shape as exchange); hook `useFuelPrices(): { rates: FuelRateItem[]; date: string; loading: boolean; error: string | null; retry: () => void }` (raw items, response order, `[]` on error — mirrors `useExchangeRates` exactly).

- [ ] **Step 1: Dispatch test-author to write the failing test**

```ts
test('fetchFuelRates maps items') // stub fetch → {base:'UAH',unit:'liter',date:'11.11.2026',items:[{code:'A95',rate:88.74}]}; assert deep-equal items + base/unit/date
test('fetchFuelRates throws connection error when backend is down') // fetch rejects → exact news.ts connection message
test('useFuelPrices returns raw rates in response order') // hook driver, 5 stubbed items A95/A92/A98/DIESEL/LPG → assert rates deep-equal in order
test('useFuelPrices returns empty rates when rates fail') // stub rejection → rates === [] and non-empty error
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx tsx --test tests/fuel-api.test.ts` (from `frontend/`)
Expected: FAIL — module `../src/api/fuel` does not exist.

- [ ] **Step 3: Implement `src/api/fuel.ts`, `src/hooks/use-fuel-prices.ts`, extend `mock.ts`**

Mirror `exchange.ts` / `use-exchange-rates.ts` line for line (only names/URLs differ).

- [ ] **Step 4: Run tests, lint and typecheck**

Run: `npx tsx --test tests/fuel-api.test.ts`, then `npm run lint`, then `npx tsc -b` (from `frontend/`)
Expected: test PASS; lint and typecheck clean.

- [ ] **Step 5: Prepare commit (DO NOT COMMIT — ask the user)**

```bash
git add frontend/src/api/fuel.ts frontend/src/hooks/use-fuel-prices.ts frontend/src/api/mock.ts frontend/tests/fuel-api.test.ts
git status --short
```
Then STOP and ask the user for the commit command.

### Task 4: Fuel cards in the ticker (frontend)

**Files:**
- Modify: `frontend/src/sections/Ticker.tsx` (add optional `unit?: string` to `TickerRate`; render unit after the value, default `₴`)
- Modify: `frontend/src/pages/Home.tsx` (append fuel cards after currency cards: `[...rateCards, ...fuelCards]`)
- Test: `frontend/tests/exchange-ticker.test.ts` (via test-author — EXTEND, do not rewrite: new tests for unit rendering)

**Interfaces:**
- Consumes: `useFuelPrices()` from Task 3; existing `TickerRate`.
- Produces: `TickerRate{code,name,flag,value,unit?}` where fuel cards are `{code:'A95',name:'А-95',flag:'⛽',value:'88.74',unit:'₴/л'}` (names А-95/А-92/А-98/Дизель/Автогаз, flags ⛽ for A95/A92/A98, 🛢️ for diesel, 🚗 for LPG — my call, uniform with currency style); currency cards keep unit `₴` (default, unchanged rendering); fuel fetch failure → `[]` → currency cards still render (Ticker never receives an empty-combined crash: `[...rates]` of possibly-empty arrays still maps fine; fully-empty → existing null guard).

- [ ] **Step 1: Dispatch test-author to extend the failing test**

```ts
test('ticker renders fuel unit after value') // renderTicker([{code:'A95',name:'А-95',flag:'⛽',value:'88.74',unit:'₴/л'}]) → markup contains '88.74' followed by '₴/л'
test('ticker defaults unit to hryvnia sign') // renderTicker([{code:'EUR',name:'Євро',flag:'🇪🇺',value:'51.08'}]) → markup contains '₴' and not '₴/л'
test('ticker renders empty for empty rates') // unchanged, keep passing
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx tsx --test tests/exchange-ticker.test.ts` (from `frontend/`)
Expected: FAIL — `unit` prop unknown / not rendered.

- [ ] **Step 3: Implement `unit?` in `Ticker.tsx` + fuel meta + merge in `Home.tsx`**

`Home.tsx`: `fuelMeta` record mirroring `rateMeta` (code → {name, flag}); `toFuelCards(rates)` mirroring `toTickerRates` with `unit: '₴/л'` and `value: rate.toFixed(2)`; `<Ticker items={[...toTickerRates(rates), ...toFuelCards(fuelRates)]} />`. No styling changes beyond the unit span (reuse existing `mandat-*` classes).

- [ ] **Step 4: Run full frontend verification**

Run: `npx tsx --test tests/exchange-ticker.test.ts tests/fuel-api.test.ts`, then `npm run lint`, then `npm run build` (from `frontend/`)
Expected: tests PASS; lint and build clean.

- [ ] **Step 5: Prepare commit (DO NOT COMMIT — ask the user)**

```bash
git add frontend/src/sections/Ticker.tsx frontend/src/pages/Home.tsx frontend/tests/exchange-ticker.test.ts
git status --short
```
Then STOP and ask the user for the commit command.
