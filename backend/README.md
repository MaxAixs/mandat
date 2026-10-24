# Mandat backend

Go API server for the Mandat news platform: public site backend plus the
administrative API for managing news articles.

## Stack

Go, Gin, PostgreSQL (pgx), goose migrations, slog, Docker Compose.

## Local development

```bash
cp config.example.yaml config.yaml   # local app settings; true for the dev DB
docker compose up -d db              # PostgreSQL on :5432
DB_PASSWORD=mandat go run ./cmd      # API on :8080
```

`config.yaml` is git-ignored — it never holds secrets. The database password
always comes from the `DB_PASSWORD` environment variable (priority: env over yaml).
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

```bash
docker build -t mandat-backend .
docker run --rm -p 8080:8080 -e DB_PASSWORD=... mandat-backend
```

The image ships `config.example.yaml` as its built-in default config
(it contains no secrets). Mount your own config to override it:

```bash
docker run --rm -p 8080:8080 -e DB_PASSWORD=... \
  -v ./config.yaml:/app/config.yaml mandat-backend
```
