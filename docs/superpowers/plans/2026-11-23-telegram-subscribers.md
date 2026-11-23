# Telegram Subscribers Implementation Plan

> **For agentic workers:** execute task-by-task with TDD. Tests are written by the
> `test-author` opencode agent (`opencode run --agent test-author "<brief>"`), the
> primary session writes production code, `reviewer` reviews at the end.

**Goal:** Refresh the t.me/mandat_ua subscriber count hourly on the backend and show it on `/advertising`.

**Spec:** `docs/superpowers/specs/2026-11-23-telegram-subscribers-design.md`

**Tech Stack:** Go, Gin, go-redis, testify · React 19, TypeScript, node:test.

## Global Constraints

- Same layering as exchange/fuel: service owns the store and HTTP interfaces; handler only reads.
- Every service/repo function starts with `const fc = "<name>_<Func>"`, wraps with `%w`.
- Logging only in services; a failed hourly refresh is logged and keeps the old value.
- No per-sentinel error mapping tests.
- Primary never edits `*_test.go`, `*.test.ts(x)`, `testdata/`.
- No commits/pushes without the user's command; backend and frontend are separate commits.

## Review Focus

- Parsing never accepts a missing/zero count; NBSP and spaces between digit groups are handled.
- Body size is capped; non-2xx is an error.
- A failed refresh never overwrites the stored value.
- Config rejects a non-`https://t.me/` channel URL and intervals under a minute.

## Tasks

### Task 1: Backend
- [ ] Stubs with final signatures (config, httpclient.GetText, model, store, service, handler, route).
- [ ] test-author: config defaults/validation; `GetText` (2xx body, non-2xx error, size cap);
      service parsing table (plain spaces, NBSP, singular, missing block, zero), Refresh stores /
      keeps old value on failure, Stats not-found; Redis store round trip (miniredis, as the
      existing store tests); handler 200 shape and 503.
- [ ] Implement, wire in `cmd/main.go`, `config.yaml`, README; `gofmt`, `go vet`, `go test ./...`.

### Task 2: Frontend
- [ ] Stubs: `TelegramChannelStats`, `src/api/telegram.ts`.
- [ ] test-author: API test (URL, parse, error); `/advertising` Telegram tile shows the formatted
      count from the API and `—` before it; `TELEGRAM_CHANNEL.subscribers` removed.
- [ ] Implement; `npm run lint`, `npm run build`, suites; screenshot `/advertising`.

### Task 3: Review
- [ ] `reviewer` → `.superpowers/reviews/2026-11-23-telegram-subscribers.txt`; fix findings; restart backend.
