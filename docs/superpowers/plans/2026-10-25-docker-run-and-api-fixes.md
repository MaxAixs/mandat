# Docker Run and API Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the backend image runnable as-is, fix the DELETE status contract, add a compose `app` service, and verify the admin panel against the Dockerized API.

**Architecture:** Keep the Go layering intact. The image bakes `config.example.yaml` at `/app/config.yaml`; a `WORKDIR /app` makes it the config the binary reads. Deployment-specific values (the database host) come from the environment, so one image serves both local Compose (`db`) and host runs (`localhost`). Compose gains an `app` service built from the same Dockerfile.

**Tech Stack:** Go 1.27, Gin, pgx/goose, Viper, PostgreSQL 16, Docker Compose, React admin panel (unchanged).

**Spec:** User-approved scope in this session: fix `WORKDIR` and the DELETE status contract, add `DB_HOST` env override, add the compose `app` service, then verify CRUD and the admin panel against the container.


> **Update 2026-10-25:** the `DB_HOST` environment override was removed at the user's request. The host now lives in `config.docker.yaml` (mounted by Compose) and `DB_PASSWORD` comes from `.env` (see `.env.example`). The `DB_HOST` parts of Tasks 2 and 4 are superseded.

## Global Constraints

- Do not modify frontend code; the panel already targets `http://localhost:8080/api/admin/news`.
- Do not commit or push; the user commits explicitly.
- No secrets in `config.yaml`, `docker-compose.yml`, or the image; only `DB_PASSWORD`/`DB_HOST` through the environment.
- Service/repository code keeps the `fc` constant and `%w` wrapping conventions; this plan does not touch them.
- Run `gofmt -w`, `go vet ./...`, and `go test ./...` before claiming completion.
- The development database volume (`pgdata`) must survive; no `-v` pruning.

## Review Focus

- A successful DELETE returns `200` with an empty body; the panel's `request()` returns `undefined` for empty bodies — verify the delete path stays valid (Task 1, Task 5).
- `DB_HOST` must override the file but an empty/unset variable must not blank it; both cases tested in Task 2.
- Compose `app` must not start before `db` is healthy, or the 15s startup timeout fails; `depends_on: service_healthy` in Task 4.
- The baked config must stay secret-free and the container must stay non-root after adding `WORKDIR`; checked in Task 3.
- The admin panel origin `http://localhost:3000` must remain allowed by CORS after the move to Compose; checked in Task 5.
- Port `8080` is currently held by a local `go run` process; it must be stopped before Compose starts the `app` service (Task 5).

---

### Task 1: DELETE returns 200 OK

**Files:**
- Modify: `backend/internal/transport/http/handler/news_handler.go` (the `Delete` method)
- Test: `backend/internal/transport/http/handler/news_handler_test.go:231` (already exists, currently red)

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces: `DELETE /api/admin/news/:slug` responds `200` with an empty body on success; `404` unchanged.

- [ ] **Step 1: Confirm the existing test fails**

Run: `cd backend && go test ./internal/transport/http/handler -run TestDelete_Returns200 -v`
Expected: FAIL — `expected: 200`, `actual: 204` (the handler was switched to 204 earlier in this session; the user wants 200).

- [ ] **Step 2: Change the success status in `Delete`**

Replace the `c.Status(http.StatusOK)` call at the end of `Delete` with `c.Status(http.StatusNoContent)`; nothing else changes.

- [ ] **Step 3: Verify the handler package passes**

Run: `cd backend && go test ./internal/transport/http/handler -v`
Expected: PASS, including `TestDelete_NotFoundReturns404`.

- [ ] **Step 4: Verify the whole suite**

Run: `cd backend && go test ./...`
Expected: PASS for every package.

### Task 2: `DB_HOST` environment override

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`
- Modify: `backend/README.md` (configuration table)

**Interfaces:**
- Consumes: `config.Load(path string, getenv func(string) string)`.
- Produces: `DB_HOST` overrides `db.host` from the YAML; unset/empty leaves the file value. `DB_PASSWORD` behaviour is unchanged.

- [ ] **Step 1: Write the failing tests**

Add to `config_test.go`, following the existing `writeConfig`/`envMap` helpers:

```go
func TestLoad_HostFromEnv(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, fullYAML), envMap(map[string]string{
		"DB_PASSWORD": "secret",
		"DB_HOST":     "db",
	}))

	require.NoError(t, err)
	assert.Equal(t, "db", cfg.DB.Host)
}

func TestLoad_EmptyHostEnvKeepsFileValue(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, fullYAML), envMap(map[string]string{
		"DB_PASSWORD": "secret",
		"DB_HOST":     "",
	}))

	require.NoError(t, err)
	assert.Equal(t, "db-host", cfg.DB.Host)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd backend && go test ./internal/config -run 'TestLoad_(HostFromEnv|EmptyHostEnvKeepsFileValue)' -v`
Expected: `TestLoad_HostFromEnv` FAIL (`db-host` != `db`); `TestLoad_EmptyHostEnvKeepsFileValue` PASS.

- [ ] **Step 3: Implement the override**

In `config.go` add `const hostEnvVar = "DB_HOST"` next to `passwordEnvVar`, and in `Load`, after `Unmarshal` and before `validate`:

```go
if host := getenv(hostEnvVar); host != "" {
	cfg.DB.Host = host
}
```

- [ ] **Step 4: Run the config tests**

Run: `cd backend && go test ./internal/config -v`
Expected: PASS, including the pre-existing password tests.

- [ ] **Step 5: Document the variable**

Add a `DB_HOST` row to the README configuration table: optional, defaults to `db.host`, overrides it, used by Docker Compose (`db`).

### Task 3: Dockerfile runs the baked config

**Files:**
- Modify: `backend/Dockerfile` (runtime stage)

**Interfaces:**
- Consumes: `config.example.yaml` copied to `/app/config.yaml`.
- Produces: image `mandat-backend` whose default working directory is `/app`, so `/app/server` finds `/app/config.yaml`; runs as `mandat` (uid 10001).

- [ ] **Step 1: Add `WORKDIR /app`**

In the runtime stage, add `WORKDIR /app` after `COPY --from=builder /out/server /app/server` (before `USER mandat`). Keep the existing `COPY config.example.yaml /app/config.yaml` and `ENTRYPOINT ["/app/server"]`.

- [ ] **Step 2: Build and verify the working directory and config**

Run:

```bash
cd backend
docker build -t mandat-backend:check .
docker run --rm --entrypoint sh mandat-backend:check -c 'pwd; test -r config.yaml && echo CONFIG_OK; id -u'
```

Expected: `/app`, `CONFIG_OK`, `10001`.

- [ ] **Step 3: Verify startup without mounts**

Run the image on the existing dev network with the environment pointing at the dev DB:

```bash
docker run --rm --network backend_default -e DB_PASSWORD=mandat -e DB_HOST=db \
  -p 18080:8080 mandat-backend:check
```

Expected: no `load config` error; the server logs a goose version and listens. Stop with Ctrl+C. If the `backend_default` endpoint is still unreachable (known local breakage), run this check after Task 4 through Compose instead.

### Task 4: Compose `app` service

**Files:**
- Modify: `backend/docker-compose.yml`
- Modify: `backend/README.md` (Docker section)

**Interfaces:**
- Consumes: image behaviour from Task 3 and `DB_HOST` from Task 2.
- Produces: `docker compose up -d --build` starts `db` and `app`; the API is reachable on `http://localhost:8080`, panel origin allowed.

- [ ] **Step 1: Add the service**

Append to `docker-compose.yml`:

```yaml
  app:
    build: .
    environment:
      DB_PASSWORD: mandat
      DB_HOST: db
    ports:
      - "8080:8080"
    depends_on:
      db:
        condition: service_healthy
```

Keep the dev-only credentials comment at the top of the file.

- [ ] **Step 2: Validate the Compose model**

Run: `cd backend && docker compose config`
Expected: both `db` and `app` are printed; no interpolation or schema errors.

- [ ] **Step 3: Start the stack**

Run: `cd backend && docker compose up -d --build`
Expected: `db` becomes healthy, `app` starts, and `docker compose ps` shows both running.

- [ ] **Step 4: Smoke-test the containerized API**

Run:

```bash
curl -s -o /dev/null -w '%{http_code}\n' 'http://localhost:8080/api/admin/news?page=1&limit=5'
curl -s -i -X OPTIONS 'http://localhost:8080/api/admin/news' \
  -H 'Origin: http://localhost:3000' -H 'Access-Control-Request-Method: POST' | head -8
```

Expected: `200`, and `204` with `Access-Control-Allow-Origin: http://localhost:3000`.

- [ ] **Step 5: Update README Docker usage**

Document `docker compose up -d --build` as the primary path, the `DB_HOST`/`DB_PASSWORD` variables, and the corrected `docker run` example (no config mount needed; mount only to override). Record `DELETE` as `200 OK` with an empty body.

### Task 5: End-to-end check with the admin panel

**Files:**
- No source changes; verification only.

**Interfaces:**
- Consumes: Compose stack from Task 4, running Vite dev server on `http://localhost:3000`, headless Chrome.
- Produces: evidence that the panel loads and mutates data through the Dockerized API.

- [ ] **Step 1: Free port 8080**

Find the local process with `ss -ltnp | grep :8080` (expected `go run`/build-cache binary) and stop it with `kill <pid>`. Confirm the port is free. Do not stop the Vite server on `:3000` or the database container.

- [ ] **Step 2: Recreate the stack with the free port**

Run: `cd backend && docker compose up -d --build` then `docker compose ps`.
Expected: `app` is running and bound to `8080`.

- [ ] **Step 3: Verify CRUD through the container with the panel's payload shape**

Create an article with `{"category":"Важливо","title":"Docker panel check",...,"status":"published"}`; expect `201`. Repeat list `200`, get-by-slug `200`, update `200`, delete `200`. Confirm a repeat `GET` returns `404` and the list is empty. Use the slug from each response when it changes on update.

- [ ] **Step 4: Verify the panel renders the container data**

Seed one published article, then run:

```bash
google-chrome --headless=new --disable-gpu --no-sandbox \
  --user-data-dir=/tmp/opencode/chrome-profile --virtual-time-budget=8000 \
  --dump-dom http://localhost:3000/admin > /tmp/opencode/admin-dom.html
grep -c 'Docker panel check' /tmp/opencode/admin-dom.html
```

Expected: at least `1`; no connection-error text in the DOM.

- [ ] **Step 5: Clean up test data**

Delete the seeded article through the API and confirm the list is empty. Leave the stack running only if the user asks; otherwise `docker compose down` (without `-v`).

- [ ] **Step 6: Final checks**

Run: `cd backend && gofmt -l . && go vet ./... && go test ./...`
Expected: no `gofmt` output, `vet` clean, tests PASS.
