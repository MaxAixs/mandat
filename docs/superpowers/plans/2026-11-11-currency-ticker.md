# Currency Ticker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the news headlines in the home page running ticker with NBU currency rates (EUR, USD, PLN), refreshed once a day with fallback to the nearest available day.

**Architecture:** Backend owns NBU fetching, daily refresh, caching and fallback in a new `ExchangeService` (mirrors the `imgdownloader` pattern: service owns the `Fetcher` interface, a small `internal/exchange` package implements the NBU HTTP client); a thin public handler serves `GET /api/exchange/rates`. Frontend adds an exchange API client + hook and wires `Home.tsx` to feed formatted rate strings into the existing `Ticker`.

**Tech Stack:** Go (Gin, `log/slog`, `net/http`), React 19 + TypeScript + Vite + Tailwind.

**Spec:** In-chat agreement 2026-11-11 (no separate design doc — small change, approved in chat). Pinned spec values: ticker shows ONLY rates, no news; format `UAH -> EUR = 50.48` (code order EUR, USD, PLN); NBU base URL `https://bank.gov.ua/NBU_Exchange/exchange?json`, date param format `DD.MM.YYYY` (verified live 2026-11-11: `?date=10.11.2026` works, `?date=20251110` returns "Wrong date format"); refresh once per day; fallback walks back day-by-day to the nearest day with all three codes; frontend rate text uses exactly 2 decimals with a dot (`toFixed(2)`).

## Global Constraints

- Backend: `gofmt`, `go vet ./...`, `go test ./...` pass; every service/repository function starts with `const fc = "..."` and wraps errors with `%w`; logging only in service layer via `log/slog`; handlers stay thin, errors map via the `errResps` table only.
- Test files (`*_test.go`, `*.test.ts`) are written ONLY by the `test-author` subagent — the implementer never edits them directly; each test step below means "dispatch test-author with the exact test name and assertions".
- Frontend: strict TS, imports via `@/` alias, Tailwind `mandat-*` classes only (no hardcoded hex), `npm run lint` and `npm run build` pass.
- Commits: `[+]` prefix, no `frontend:`/`backend:` scope; no commit ever mixes `frontend/` and `backend/`; never commit `dist/`, `node_modules/`; commit/push only on explicit user command — plan steps say "commit" as the tracking action, the human confirms.
- NBU per-unit rate = `Amount / Units`; a date's data is usable only if USD, EUR and PLN are ALL present.

## Review Focus

- NBU unreachable for longer than the fallback window → backend returns 503 (`ErrServiceUnavailable`, already in `errResps`), frontend renders no ticker and the page does not break (test in Task 3 + Task 5).
- NBU entries with `Units` 10/100/1000 → rate must be divided to per-unit value (test in Task 1).
- NBU response missing one of EUR/USD/PLN for a date → that date is skipped, walk-back continues (test in Task 2).
- Cold start with NBU unreachable → service starts with empty cache, handler returns 503 instead of crashing (test in Task 2/3).
- Rate text formatting → exactly `UAH -> {CODE} = {NNN.NN}`, codes in EUR, USD, PLN order (test in Task 4).

---

### Task 1: Exchange model + NBU client (backend)

**Files:**
- Create: `backend/internal/models/exchange.go`
- Create: `../../../backend/internal/exchange/exchange.go`
- Test: `../../../backend/internal/exchange/exchange_test.go` (via test-author)

**Interfaces:**
- Consumes: `net/http`, `internal/models` (inward only — no Gin, no service imports).
- Produces: `models.ExchangeRate{Code string; Rate float64; Date string}`; `exchange.Client` with `NewClient(baseURL string, httpClient *http.Client) *Client` and `(c *Client) FetchDate(ctx context.Context, date time.Time) ([]*models.ExchangeRate, error)` returning per-unit rates for ALL currencies in the response (filtering to EUR/USD/PLN is Task 2's job); `const BaseURL = "https://bank.gov.ua/NBU_Exchange/exchange?json"`, request timeout 10s via the injected `*http.Client`.

- [ ] **Step 1: Dispatch test-author to write the failing test**

```go
func TestFetchDate_ReturnsPerUnitRates(t *testing.T) // httptest server returns fixture with Units 1 (USD 45.322), Units 1 (EUR 50.4823), Units 10 (JPY 2.85); assert USD==45.322, EUR==50.4823, JPY==0.285; assert request path contains date=10.11.2026
func TestFetchDate_ServerError(t *testing.T) // httptest server returns 500; assert error is returned
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/exchange/ -run TestFetchDate -v`
Expected: FAIL — package `internal/exchange` does not exist.

- [ ] **Step 3: Implement `models.ExchangeRate` in `backend/internal/models/exchange.go` and `exchange.Client` in `../../../backend/internal/exchange/exchange.go`**

NBU item fields: `CurrencyCodeL`, `Units`, `Amount`, `StartDate`. Query = `?json` plus `&date=` + `date.Format("02.01.2006")` when date is non-zero... (decision: `FetchDate` always takes an explicit date; "today" is the caller's choice — keeps the client deterministic and testable). Skip items with `Units <= 0`. Wrap errors as `fmt.Errorf("nbuClient_FetchDate: fetch nbu rates: %w", err)` — no `fc` const here (not service/repository layer), no logging.

- [ ] **Step 4: Run tests to verify they pass**

Run: `gofmt -l internal/exchange internal/models && go vet ./internal/exchange/ ./internal/models/ && go test ./internal/exchange/ -v`
Expected: PASS, no gofmt output.

- [ ] **Step 5: Commit (backend only)**

```bash
git add backend/internal/models/exchange.go backend/internal/exchange/exchange.go backend/internal/exchange/exchange_test.go
git commit -m "[+] NBU exchange client with per-unit rates"
```

### Task 2: ExchangeService with cache, fallback, daily refresh (backend)

**Files:**
- Create: `backend/internal/service/exchange.go`
- Test: `backend/internal/service/exchange_test.go` (via test-author)

**Interfaces:**
- Consumes: `ExchangeFetcher` (defined in this file — service owns the contract):
```go
type ExchangeFetcher interface {
    FetchDate(ctx context.Context, date time.Time) ([]*models.ExchangeRate, error)
}
```
- Produces: `NewExchangeService(fetcher ExchangeFetcher, logger *slog.Logger) *ExchangeService`; `(s *ExchangeService) GetRates(ctx context.Context) ([]*models.ExchangeRate, error)` (EUR, USD, PLN in that order); `(s *ExchangeService) Refresh(ctx context.Context) error`; `(s *ExchangeService) StartAutoRefresh(ctx context.Context, interval time.Duration)`; constants `exchangeWantedCodes = [3]string{"EUR","USD","PLN"}`, `exchangeMaxFallbackDays = 7`.

- [ ] **Step 1: Dispatch test-author to write the failing test**

```go
func TestGetRates_FallsBackToNearestDay(t *testing.T) // fake fetcher fails for today, returns full set for yesterday; assert 3 rates EUR/USD/PLN in order with yesterday's values
func TestGetRates_AllDaysMissing(t *testing.T) // fake fetcher always fails; assert errors.Is(err, errs.ErrServiceUnavailable)
func TestGetRates_UsesCache(t *testing.T) // fake counts calls; two GetRates calls → exactly 1 fetch
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run TestGetRates -v`
Expected: FAIL — `NewExchangeService` undefined.

- [ ] **Step 3: Implement `ExchangeService` in `backend/internal/service/exchange.go`**

`const fc = "exchangeService_Refresh"` in `Refresh`, `const fc = "exchangeService_GetRates"` in `GetRates`. `Refresh`: try today then back to `today-6`, first date whose response contains all three wanted codes wins; store `{rates, date}` under `sync.RWMutex`. `GetRates`: return cache if warm, else `Refresh` once. Log failures with `s.logger.ErrorContext(ctx, ..., "fc", fc, "error", err)`; wrap returns `fmt.Errorf("%s: %w", fc, err)`. Missing-codes and fetch failures both continue the walk-back; total failure → `errs.ErrServiceUnavailable`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `gofmt -l internal/service && go vet ./internal/service/ && go test ./internal/service/ -run 'TestGetRates|TestExchange' -v`
Expected: PASS.

- [ ] **Step 5: Commit (backend only)**

```bash
git add backend/internal/service/exchange.go backend/internal/service/exchange_test.go
git commit -m "[+] exchange service with daily refresh and nearest-day fallback"
```

### Task 3: Exchange HTTP endpoint + wiring (backend)

**Files:**
- Modify: `backend/internal/transport/http/dto/response.go` (append types + mapper)
- Create: `backend/internal/transport/http/handler/exchange.go`
- Test: `backend/internal/transport/http/handler/exchange_test.go` (via test-author)
- Modify: `backend/cmd/main.go` (wire client → service → handler, start refresh goroutine)

**Interfaces:**
- Consumes: `service.ExchangeService.GetRates` via a narrow handler-local interface:
```go
type ExchangeService interface {
    GetRates(ctx context.Context) ([]*models.ExchangeRate, error)
}
```
- Produces: `GET /api/exchange/rates` → `200 {"base":"UAH","date":"12.11.2026","items":[{"code":"EUR","rate":50.48},...]}`; `NewExchangeHandler(svc ExchangeService) *ExchangeHandler` with `Register(r *gin.RouterGroup)` mounting `r.GET("/api/exchange/rates", h.GetRates)`; rates rounded to 2 decimals in the DTO mapper (`math.Round(x*100)/100`).

- [ ] **Step 1: Dispatch test-author to write the failing test**

```go
func TestExchangeHandler_GetRates(t *testing.T) // fake service returns 3 rates; assert 200, base=="UAH", items order EUR/USD/PLN, rate EUR==50.48
func TestExchangeHandler_ServiceUnavailable(t *testing.T) // fake service returns errs.ErrServiceUnavailable; assert 503 with {"error":...} envelope
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/transport/http/handler/ -run TestExchangeHandler -v`
Expected: FAIL — `NewExchangeHandler` undefined.

- [ ] **Step 3: Implement DTO types + mapper in `dto/response.go`, handler in `handler/exchange.go`, wiring in `cmd/main.go`**

Handler is thin: call `svc.GetRates`, on error `WriteError(c, err)` (no `errResps` change — `ErrServiceUnavailable` already maps to 503), else `c.JSON(200, ...)`. `main.go`: `nbuClient := exchange.NewClient(exchange.BaseURL, &http.Client{Timeout: 10*time.Second})`, `exchangeService := service.NewExchangeService(nbuClient, log)`, startup warmup `Refresh` with `startupCtx` (log failure, do NOT fail startup — handler serves 503 until first success), `go exchangeService.StartAutoRefresh(ctx, 24*time.Hour)`, pass `handler.NewExchangeHandler(exchangeService)` into `httptransport.New(...)`. No config-file changes (URL/timeout/codes are constants).

- [ ] **Step 4: Run tests to verify they pass**

Run: `gofmt -l backend && go vet ./... && go test ./...`
Expected: PASS (full backend suite, from `backend/` dir).

- [ ] **Step 5: Commit (backend only)**

```bash
git add backend/internal/transport/http/dto/response.go backend/internal/transport/http/handler/exchange.go backend/internal/transport/http/handler/exchange_test.go backend/cmd/main.go
git commit -m "[+] public exchange rates endpoint with NBU refresh"
```

### Task 4: Exchange API client + hook (frontend)

**Files:**
- Create: `frontend/src/api/exchange.ts`
- Create: `frontend/src/hooks/use-exchange-rates.ts`
- Test: `frontend/tests/exchange-api.test.ts` (via test-author, `node:test` pattern like `public-news-api.test.ts`)
- Modify: `frontend/src/api/mock.ts` (append `mockExchangeRates: string[]`)

**Interfaces:**
- Consumes: same `request`-style fetch against `http://localhost:8080` + credentials `include` (mirror `src/api/news.ts`).
- Produces: `ExchangeRateItem{code: string; rate: number}`, `ExchangeRatesResponse{base: string; date: string; items: ExchangeRateItem[]}`; `fetchExchangeRates(): Promise<ExchangeRatesResponse>`; `formatRateText(code: string, rate: number): string` → `` `UAH -> ${code} = ${rate.toFixed(2)}` ``; hook `useExchangeRates(): { items: string[]; date: string; loading: boolean; error: string | null; retry: () => void }` returning formatted strings in response order, `[]` on error (ticker hides itself, matching `Ticker`'s empty guard).

- [ ] **Step 1: Dispatch test-author to write the failing test**

```ts
test('fetchExchangeRates maps items and formatRateText pins 2 decimals') // stub global fetch → {base:'UAH',date:'12.11.2026',items:[{code:'EUR',rate:50.4823}]}; assert formatRateText('EUR', 50.4823) === 'UAH -> EUR = 50.48'
test('fetchExchangeRates throws connection error when backend is down') // fetch rejects; assert error message matches news.ts connection message
test('useExchangeRates returns formatted items in response order') // hook driver pattern from tests/category-news-hook.test.ts (real hook module, local React dispatcher); stubbed API → assert items === ['UAH -> EUR = 50.48', 'UAH -> USD = 45.32', 'UAH -> PLN = 11.47']
test('useExchangeRates returns empty items when rates fail') // stubbed API rejects; assert items === []
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx tsx --test tests/exchange-api.test.ts` (from `frontend/`)
Expected: FAIL — module `../src/api/exchange` does not exist.

- [ ] **Step 3: Implement `src/api/exchange.ts`, `src/hooks/use-exchange-rates.ts`, extend `mock.ts`**

Mirror `news.ts` error handling exactly (connection message `Не вдалося з’єднатися з сервером. Перевірте, чи запущено backend.`, error-envelope parsing). `mockExchangeRates = ['UAH -> EUR = 50.48', 'UAH -> USD = 45.32', 'UAH -> PLN = 11.47']` for dev without backend.

- [ ] **Step 4: Run tests, lint and typecheck to verify**

Run: `npx tsx --test tests/exchange-api.test.ts`, then `npm run lint`, then `npx tsc -b` (from `frontend/`)
Expected: test PASS; lint and typecheck clean.

- [ ] **Step 5: Commit (frontend only)**

```bash
git add frontend/src/api/exchange.ts frontend/src/hooks/use-exchange-rates.ts frontend/src/api/mock.ts frontend/tests/exchange-api.test.ts
git commit -m "[+] exchange rates API client and hook"
```

### Task 5: Ticker shows rates instead of news (frontend)

**Files:**
- Modify: `frontend/src/pages/Home.tsx` (use `useExchangeRates`, pass `items` to `Ticker`)
- Modify: `frontend/src/sections/Ticker.tsx` (rename prop `headlines` → `items`: it no longer carries headlines; no existing test renders `Ticker` directly — verified — so the rename is safe)
- Test: `frontend/tests/exchange-ticker.test.ts` (via test-author) — `renderToStaticMarkup(<Ticker items={mockExchangeRates} />)` contains each rate string; `<Ticker items={[]} />` renders `''`

**Interfaces:**
- Consumes: `useExchangeRates()` from Task 4; `mockExchangeRates` from `src/api/mock.ts`.
- Produces: `Ticker({ items }: { items: string[] })` (renamed prop); Home ticker fed by rates; on `error`/empty, `items={[]}` so `Ticker` returns null (no layout break). `getHomeNews`/`tickerItems` in `src/api/index.ts` stay untouched (news-limit tests keep passing).

- [ ] **Step 1: Dispatch test-author to write the failing test**

```ts
test('ticker renders each exchange rate string') // renderToStaticMarkup(<Ticker items={['UAH -> EUR = 50.48', 'UAH -> USD = 45.32']} />); assert markup includes both strings
test('ticker renders nothing for empty rates') // renderToStaticMarkup(<Ticker items={[]} />); assert markup === ''
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx tsx --test tests/exchange-ticker.test.ts` (from `frontend/`)
Expected: FAIL — `Ticker` destructures `headlines`, so `items` is ignored and `headlines.length` throws TypeError.

- [ ] **Step 3: Implement the prop rename in `frontend/src/sections/Ticker.tsx` and the wiring in `frontend/src/pages/Home.tsx`**

`Ticker({ items }: { items: string[] })`, empty guard and marquee logic unchanged. `Home.tsx`: replace `tickerItems` from `usePublicNews` with `const { items: rateItems } = useExchangeRates()` and `<Ticker items={rateItems} />`. Keep `usePublicNews` for articles/latestNews/mainEvent. No styling changes (existing `mandat-*` classes untouched).

- [ ] **Step 4: Run full frontend verification**

Run: `npx tsx --test tests/exchange-ticker.test.ts tests/exchange-api.test.ts`, then `npm run lint`, then `npm run build` (from `frontend/`)
Expected: tests PASS; lint and build clean.

- [ ] **Step 5: Commit (frontend only)**

```bash
git add frontend/src/pages/Home.tsx frontend/tests/exchange-ticker.test.ts frontend/src/sections/Ticker.tsx
git commit -m "[+] ticker shows NBU currency rates"
```
