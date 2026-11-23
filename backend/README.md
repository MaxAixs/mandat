# Mandat backend

Go API server for the Mandat news platform: public site backend plus the
administrative API for managing news articles.

## Stack

Go, Gin, PostgreSQL (pgx), Redis (sessions), goose migrations, slog, Docker Compose.

## Local development

Docker Compose is the supported way to run the application:

```bash
cp .env.example .env   # first run only: dev DB password, git-ignored
docker compose up -d --build
```

The API listens on `:8080`, PostgreSQL on `127.0.0.1:5432`, Redis on `127.0.0.1:6379`
(database ports are bound to loopback only).
Migrations are embedded in the binary and applied automatically on startup.

`go test ./...` can be run from the host; unit tests need nothing, repository
integration tests need PostgreSQL on `localhost:5432` (see Tests).

## Configuration

There is exactly one config file, `config.yaml`, and it is tracked in git. It
contains no secrets and uses the Compose service names (`db`, `redis:6379`),
because the service is expected to run inside Compose. Changing those hosts
means rebuilding the image.

Secrets come from `.env` (git-ignored) through the environment:

| Variable | Required | Description |
|---|---|---|
| `DB_PASSWORD` | yes | PostgreSQL password (never put it in the yaml); Compose reads it from `.env` |
| `REDIS_PASSWORD` | no | Only for deployments running the app directly against a password-protected Redis; the local Compose stack has none and does not forward it |

## Authentication

Staff accounts are provisioned only through SQL. Store a bcrypt hash (cost 12 or
higher), never a plaintext password:

```sql
INSERT INTO admin_users (login, password_hash, role)
VALUES ('editor', '<BCRYPT_HASH>', 'moderator');
```

Allowed roles are `admin` and `moderator`; both currently manage news.
Authentication uses an opaque random token in the `mandat_admin_session` cookie.
Only its SHA-256 hash is stored, in Redis (`admin:session:<hash>` keys with a TTL
matching the session expiry); staff accounts stay in PostgreSQL, so role changes
apply without re-login. Sessions last 12 hours and logout revokes them
immediately; a per-user set (`admin:user:<id>:sessions`) lets `logout-all` revoke
every session of the account. Restarting Redis logs everyone out; the legacy
`admin_sessions` PostgreSQL table is no longer read or written. The cookie is
`HttpOnly` and `SameSite=Lax`; set `http.secure_cookies: true` (or
`HTTP_SECURE_COOKIES=true`) behind production HTTPS — startup fails when a
non-local origin is allowed without secure cookies. State-changing requests under
`/api/admin` and `/api/auth` sent from a browser must carry an `Origin` that is
listed in `http.allowed_origins` (env `HTTP_ALLOWED_ORIGINS`) or matches the API
host; otherwise they get `403`.

Login attempts are limited in Redis per client IP (`ratelimit:login:ip:<client>`)
and per account (`ratelimit:login:user:<login>`), by default 5 per 5 minutes each
(`login_rate_limit` section). Over the limit login answers `429` with `Retry-After`.

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/auth/login` | Login with `{ "login", "password" }` and set the session cookie |
| `POST` | `/api/auth/logout` | Revoke the current session and clear the cookie (`204`) |
| `POST` | `/api/auth/logout-all` | Revoke every session of the current user and clear the cookie (`204`) |
| `GET` | `/api/auth/me` | Return the current user's `id`, `login`, and `role` |

## Admin API

Base path `/api/admin/news`. Every endpoint requires a valid admin or
moderator session cookie. Cross-origin frontend requests must include credentials.

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/admin/news` | Create an article |
| `GET` | `/api/admin/news?page=&limit=&status=&category=` | Paginated list (newest first) |
| `GET` | `/api/admin/news/:slug` | Get one article |
| `PUT` | `/api/admin/news/:slug` | Update an article (the slug never changes) |
| `DELETE` | `/api/admin/news/:slug` | Delete an article (`200 OK`, empty body) |

`page` starts at 1, `limit` defaults to 20 and is capped at 100. List responses
are `{ "items", "page", "limit", "has_more" }`; `has_more` is true when a next page exists.
Cover uploads are checked by content before anything is written (the extension must
match the image bytes, max 10 MB) and stored as `covers/<slug>-<random>.<ext>`.
`status` accepts `draft` or `published`. `category` accepts `Новини`,
`Міжнародні новини`, `Спорт`, `Інтерв'ю`, `Економіка`, `Політика`, or
`Регіональні новини`; an unknown value returns `400 Invalid input`.

Ad format cards of the `/advertising` page (same roles as news):

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/admin/ad-formats` | All cards in display order, `{ "items" }` |
| `POST` | `/api/admin/ad-formats` | Create a card at the end (multipart: `title`, `description`, optional file `image`) |
| `PUT` | `/api/admin/ad-formats/:id` | Update text; a new `image` replaces the old one, `remove_image=true` clears it |
| `DELETE` | `/api/admin/ad-formats/:id` | Delete a card and its image (`200 OK`, empty body) |
| `POST` | `/api/admin/ad-formats/:id/move` | Swap with the neighbour: `{ "direction": "up" \| "down" }`; no neighbour is a no-op |

`title` is 1–80 characters, `description` at most 240. Images follow the cover rules
and are stored as `ad-formats/<id>-<random>.<ext>`; replaced and deleted images are removed.

Contacts of the `/contacts` page (same roles as news):

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/admin/contacts` | All contacts in display order, `{ "items" }` |
| `POST` | `/api/admin/contacts` | Create a contact at the end: `{ "type", "value", "label" }` |
| `PUT` | `/api/admin/contacts/:id` | Update type, value and label (same body) |
| `DELETE` | `/api/admin/contacts/:id` | Delete a contact (`200 OK`, empty body) |
| `POST` | `/api/admin/contacts/:id/move` | Swap with the neighbour: `{ "direction": "up" \| "down" }`; no neighbour is a no-op |

`type` is one of `email`, `telegram`, `instagram`, `threads`, `facebook`, `phone`, `website`.
The value is validated per type; social types accept `name`, `@name` or a profile link on
their own host and store the bare handle. Responses add the server-built `display` text and
`href` link; `label` (at most 40 characters) replaces the type name on the page.

## Public API

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/news?page=&limit=&category=` | Paginated published articles for the public site (same `category` values as the admin API) |
| `GET` | `/api/news/search?q=&limit=` | Live title suggestions over published articles, newest first (default limit 7, max 20; empty `q` returns an empty list) |
| `GET` | `/api/ad-formats` | Ad format cards of the advertising page in display order, `{ "items" }` |
| `GET` | `/api/contacts` | Contacts of the contacts page in display order, `{ "items" }` with `display` and `href` |
| `GET` | `/api/telegram/channel` | Subscriber count of the Telegram channel, `{ "subscribers", "updated_at" }`; refreshed hourly from the public t.me page, `503` until the first successful refresh |
| `GET` | `/api/news/:slug` | Full published article; also establishes the anonymous viewer cookie |
| `POST` | `/api/news/:slug/view` | Record a view and return `{ "views": number }` |

View recording accepts at most one view per anonymous visitor per article per
three hours. The `mandat_viewer` cookie contains only a random UUID, lasts one year, and
is `HttpOnly` and `SameSite=Lax`; no IP address or browser fingerprint is stored.
The article-detail GET sets this cookie before the frontend records a view, so
simultaneous tracking requests share the same visitor ID. Cooldown enforcement
and counter increments are atomic in PostgreSQL.

Only articles with status `published` are returned, newest publication first.
The response contains public article cards without the full text or administrative
fields. `page` starts at 1, `limit` defaults to 20 and is capped at 100; the list
includes `has_more`.

## Health checks

| Method | Path | Description |
|---|---|---|
| `GET` | `/health/live` | Process liveness; does not check dependencies |
| `GET` | `/health/ready` | Readiness including a PostgreSQL ping with a 2-second timeout |

Both return `200 OK` with `{ "status": "ok" }` on success. Readiness returns
`503 Service Unavailable` when PostgreSQL is unreachable.

## Rate limiting

Every API request counts toward a global per-client limit kept in Redis
(`ratelimit:global:<client>` keys, fixed window, atomic Lua `INCR`+`PEXPIRE`).
The client is its IPv4 address or its IPv6 `/64` prefix. Over the limit the API
answers `429 Too Many Requests` with a `Retry-After` header (seconds) and
`{ "error": { "status": 429, "message": "Too many requests" } }`.
`OPTIONS` preflights and `/health/*` probes are never limited. When Redis is
unreachable requests are allowed and the failure is logged (fail open).

```yaml
rate_limit:
  enabled: true      # false switches the limit off
  requests: 300      # per client per window
  window: 1m         # at least 1s
```

**Client IP behind Cloudflare.** The real IP is read from `http.client_ip_headers`
(default `CF-Connecting-IP`; an empty list honours no header) only when the TCP
peer is in `http.trusted_proxies`.
The default empty list trusts nobody, so the peer address is used and spoofed
`CF-Connecting-IP` / `X-Forwarded-For` headers are ignored. The shipped `config.yaml` lists the
Cloudflare ranges (https://www.cloudflare.com/ips/); add the address of any reverse
proxy between Cloudflare and the API, and refresh the list if Cloudflare changes it —
otherwise traffic from unlisted edges shares the limit of the edge IP.
The same client IP drives the per-IP login attempt limit.

Every response carries `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
`Referrer-Policy` and a deny-all `Content-Security-Policy`; HSTS is added when
`secure_cookies` is on, and `/api/admin` / `/api/auth` responses are `Cache-Control: no-store`.

Errors are returned as:

```json
{ "error": { "status": 404, "message": "News article not found" } }
```

## Tests

```bash
go test ./...                                        # unit tests only
```

Repository integration tests need a real PostgreSQL and skip without
`DATABASE_URL`. They run migrations and **truncate `news`**, so never point
`DATABASE_URL` at a database with articles you want to keep. Use a separate
database:

```bash
docker compose up -d db                                # the db service must be running
docker compose exec db createdb -U mandat mandat_test  # first time only
DATABASE_URL=postgres://mandat:mandat@localhost:5432/mandat_test?sslmode=disable go test ./...
```

## Docker

The image bakes `config.yaml` (Compose service names `db` and `redis:6379`) at
`/app/config.yaml`. The only required secret is `DB_PASSWORD`, passed through
the environment; Compose reads it from `.env`.

Local stack — PostgreSQL, Redis, and the API on `:8080`:

```bash
cp .env.example .env   # first run only
docker compose up -d --build
```

The image resolves `db` and `redis` by name, so it is meant to run on the
Compose network; `docker run` on the default bridge cannot reach those services.
