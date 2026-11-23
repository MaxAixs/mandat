# Telegram Subscribers — Design

## Goal

The «Соціальні мережі» block of `/advertising` shows the real subscriber count of
https://t.me/mandat_ua instead of the hardcoded `42 728`. The backend refreshes it once
an hour.

## Decisions (agreed with the user)

- Source: the public channel page `https://t.me/mandat_ua` (no bot, no token). The count
  is read from `<div class="tgme_page_extra">3 846 subscribers</div>`.
- Refresh: every hour in the background, like the exchange and fuel rates. Requests never
  fetch Telegram; they read the stored value.

## Backend

- Config section `telegram`: `channel_url` (default `https://t.me/mandat_ua`, must be an
  `https://t.me/<name>` URL), `refresh_interval` (default `1h`, min `1m`). The fetch reuses
  the shared outbound HTTP client and its `rates.http_timeout`.
- `pkg/httpclient.Client.GetText(ctx, url, maxBytes)` — GET, 2xx only, body capped at
  `maxBytes` (the page is ~10 KB; cap 512 KB).
- `models.ChannelStats{Subscribers int, UpdatedAt time.Time}`.
- Service `TelegramService` (owns `TelegramStore`, `HTTPClient`-style interface):
  - `Refresh` — fetch with the shared retry policy (`FetchWithRetries`), parse, store;
    a page without a count or a count ≤ 0 is an error and keeps the previous value;
  - `StartAutoRefresh(ctx, interval)`;
  - `Stats(ctx)` — the stored value; before the first successful refresh the store returns
    `errs.ErrTelegramStatsNotFound` and the service answers `errs.ErrServiceUnavailable` (503),
    exactly like the fuel rates.
  - Parsing: the number before `subscriber`/`subscribers` inside `tgme_page_extra`;
    regular and non-breaking spaces between digit groups are dropped.
- Redis store `telegram:channel` → JSON `{subscribers, updated_at}`, no TTL (the last known
  value survives restarts and failed refreshes).
- Public `GET /api/telegram/channel` → `{"subscribers": 3846, "updated_at": "<RFC 3339>"}`.
- `cmd/main.go`: first refresh in the startup background job, then hourly auto-refresh.

## Frontend

- `src/api/telegram.ts` `getTelegramChannel()`; `TelegramChannelStats` type.
- `/advertising` loads it and shows the count formatted for `uk-UA` (`3 846`); while
  loading or on error the Telegram tile shows `—`. `TELEGRAM_CHANNEL.subscribers` is removed.

## Out of scope

Other social networks' counts, showing the count anywhere else, the Bot API.
