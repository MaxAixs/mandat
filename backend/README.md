# Mandat backend

Go API server for the Mandat news platform: public site backend plus the
administrative API for managing news articles.

## Stack

Go, Gin, PostgreSQL (pgx), goose migrations, slog, Docker Compose.

## Local development

```bash
cp config.example.yaml config.yaml   # local app settings; true for the dev DB
cp .env.example .env                 # dev DB password, git-ignored
docker compose up -d db              # PostgreSQL on :5432
set -a; . ./.env; set +a             # export DB_PASSWORD
go run ./cmd                         # API on :8080
```

`config.yaml` is git-ignored. The database password never lives in a yaml: the
app reads `DB_PASSWORD` from the environment, and Compose reads it from `.env`.
Migrations are embedded in the binary and applied automatically on startup.

## Configuration

Settings live in `config.yaml` (see `config.example.yaml`); one secret only:

| Variable | Required | Description |
|---|---|---|
| `DB_PASSWORD` | yes | PostgreSQL password (never put it in the yaml) |

## Admin API

Base path `/api/v1/admin/news`. Authentication is not implemented yet.

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/admin/news` | Create an article |
| `GET` | `/api/v1/admin/news?page=&limit=&status=` | Paginated list (newest first) |
| `GET` | `/api/v1/admin/news/:id` | Get one article |
| `PUT` | `/api/v1/admin/news/:id` | Update an article |
| `DELETE` | `/api/v1/admin/news/:id` | Delete an article |

`page` starts at 1, `limit` defaults to 20 and is capped at 100.
`status` accepts `draft` or `published`.

## Public API

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/news?page=&limit=` | Paginated published articles for the public site |

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
{ "error": { "code": "NEWS_NOT_FOUND", "message": "News article not found" } }
```

## Tests

```bash
go test ./...                                        # unit tests only
DATABASE_URL=postgres://mandat:mandat@localhost:5432/mandat?sslmode=disable go test ./...
```

Repository integration tests need a real PostgreSQL and skip without
`DATABASE_URL`.

## Docker

The image bakes `config.example.yaml` (host `localhost`) at `/app/config.yaml`;
mount another config to point it at a different host. The only secret is
`DB_PASSWORD`, passed through the environment; Compose reads it from `.env`.

Local stack — PostgreSQL plus the API on `:8080`:

```bash
cp .env.example .env   # first run only
docker compose up -d --build
```

Compose mounts `config.docker.yaml` (host `db`) over the baked config.

Standalone container (the database must be reachable from the container):

```bash
docker build -t mandat-backend .
docker run --rm -p 8080:8080 --env-file .env \
  -v ./config.docker.yaml:/app/config.yaml:ro mandat-backend
```

Mount `config.yaml` instead when the database runs on the host and you use
host networking.
