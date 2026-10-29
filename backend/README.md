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

The API listens on `:8080`, PostgreSQL on `:5432`, Redis on `:6379`.
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
immediately. Restarting Redis logs everyone out; the legacy `admin_sessions`
PostgreSQL table is no longer read or written. The cookie is `HttpOnly` and
`SameSite=Lax`; set `http.secure_cookies: true` behind production HTTPS. Login is
limited to ten attempts per client IP per five minutes.

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/auth/login` | Login with `{ "login", "password" }` and set the session cookie |
| `POST` | `/api/auth/logout` | Revoke the current session and clear the cookie (`204`) |
| `GET` | `/api/auth/me` | Return the current user's `id`, `login`, and `role` |

## Admin API

Base path `/api/admin/news`. Every endpoint requires a valid admin or
moderator session cookie. Cross-origin frontend requests must include credentials.

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/admin/news` | Create an article |
| `GET` | `/api/admin/news?page=&limit=&status=` | Paginated list (newest first) |
| `GET` | `/api/admin/news/:slug` | Get one article |
| `PUT` | `/api/admin/news/:slug` | Update an article |
| `DELETE` | `/api/admin/news/:slug` | Delete an article (`200 OK`, empty body) |

`page` starts at 1, `limit` defaults to 20 and is capped at 100.
`status` accepts `draft` or `published`.

## Public API

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/news?page=&limit=` | Paginated published articles for the public site |
| `GET` | `/api/news/:slug` | Full published article; also establishes the anonymous viewer cookie |
| `POST` | `/api/news/:slug/view` | Record a view and return `{ "views": number }` |

View recording accepts at most one view per anonymous visitor per article per
hour. The `mandat_viewer` cookie contains only a random UUID, lasts one year, and
is `HttpOnly` and `SameSite=Lax`; no IP address or browser fingerprint is stored.
The article-detail GET sets this cookie before the frontend records a view, so
simultaneous tracking requests share the same visitor ID. Cooldown enforcement
and counter increments are atomic in PostgreSQL.

Only articles with status `published` are returned, newest publication first.
The response contains public article cards without the full text or administrative
fields. `page` starts at 1, `limit` defaults to 20 and is capped at 100.

## Health checks

| Method | Path | Description |
|---|---|---|
| `GET` | `/health/live` | Process liveness; does not check dependencies |
| `GET` | `/health/ready` | Readiness including a PostgreSQL ping with a 2-second timeout |

Both return `200 OK` with `{ "status": "ok" }` on success. Readiness returns
`503 Service Unavailable` when PostgreSQL is unreachable.

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
