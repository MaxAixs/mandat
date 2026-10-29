# Admin Authentication Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Protect the admin UI and all admin news APIs with SQL-provisioned staff accounts, revocable 12-hour sessions, and administrator/moderator roles.

**Architecture:** Add staff/session models and PostgreSQL repositories, then an opaque-token auth service and Gin middleware. The React admin uses an auth provider, a protected route, login/logout UI, and credentialed cookies; no reusable token is stored in JavaScript.

**Tech Stack:** Go 1.27, Gin, pgx/v5, PostgreSQL, goose, `golang.org/x/crypto/bcrypt`, React 19, TypeScript, react-router v7, node:test.

**Spec:** `docs/superpowers/specs/2026-10-28-admin-auth-and-view-tracking-design.md`

## Global Constraints

- Follow `backend/AGENTS.md` and `frontend/AGENTS.md`; service/repository functions use local `fc` constants and preserve error causes.
- Only `test-author` writes or edits tests; the primary agent implements production code after observing RED.
- Do not add dependencies. Use the existing indirect `golang.org/x/crypto` module and promote it to a direct requirement only through `go mod tidy` if needed.
- Never store plaintext passwords or raw session tokens in PostgreSQL, logs, API responses, or browser storage.
- Do not execute `TRUNCATE`, `DELETE`, migration-down, or other destructive database operations without specific user approval. Existing integration setup contains `TRUNCATE news`; do not run it against a populated database.
- New repository integration fixtures use transaction rollback rather than cleanup deletes.
- Do not commit or push unless the user explicitly commands it.

## Review Focus

- Unknown login versus wrong password: both return the same generic `401`, create no session, and do not reveal which field failed — Task 3 service tests and Task 4 HTTP tests.
- Expired, malformed, revoked, and unsupported-role sessions: expect `401`, `401`, `401`, and `403` respectively without reaching news handlers — Task 4 middleware/route tests.
- Cookies crossing frontend/backend origins: credentialed CORS must retain explicit origins and never become `*` — Task 4 CORS tests and Task 5 API tests.
- Rapid/duplicate login attempts: the eleventh request from one IP within five minutes returns `429` without calling the service — Task 4 rate-limit tests.
- Expired session while the admin is open: an API `401` clears frontend auth state and routes to `/admin/login` — Task 5 unauthorized-event tests and Task 6 route/UI checks.

---

### Task 1: Staff schema, domain types, and auth configuration

**Files:**
- Create: `backend/internal/models/admin.go`
- Create: `backend/migrations/00004_admin_auth.sql`
- Modify: `backend/internal/errs/errs.go`
- Modify: `backend/internal/transport/http/config.go`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/config.example.yaml`, `backend/config.docker.yaml`
- Test: `backend/internal/models/admin_test.go`, `backend/internal/config/config_test.go`

**Interfaces:**
- Produces: `models.Role`, `RoleAdministrator`, `RoleModerator`, `Role.Valid() bool`, `models.AdminUser`, `models.AdminSession`.
- Produces: `HTTPConfig.SecureCookies bool`, `HTTPConfig.SessionTTL time.Duration`; defaults are `false` and `12*time.Hour`.
- Produces: `errs.ErrUnauthorized`, `errs.ErrForbidden`, `errs.ErrTooManyRequests`.

- [ ] **Step 1: test-author writes failing model/config tests**

```go
func TestRoleValid(t *testing.T) {
    assert.True(t, models.RoleAdministrator.Valid())
    assert.True(t, models.RoleModerator.Valid())
    assert.False(t, models.Role("owner").Valid())
}
```

Add config assertions that omitted YAML auth keys produce `SessionTTL == 12*time.Hour` and `SecureCookies == false`, and explicit `secure_cookies: true` is preserved.

- [ ] **Step 2: run RED**

Run: `cd backend && go test ./internal/models ./internal/config`
Expected: FAIL because the model types and HTTP fields do not exist.

- [ ] **Step 3: implement schema, models, errors, and defaults**

`AdminUser` contains `ID uuid.UUID`, `Login string`, `PasswordHash string`, `Role Role`. `AdminSession` contains `ID`, `UserID`, `TokenHash []byte`, `CreatedAt`, `ExpiresAt`, and `User AdminUser` for authenticated joins.

Migration Up creates `admin_users` and `admin_sessions` exactly as specified, including role check, unique login/token hash, `ON DELETE CASCADE`, and expiry index. Down drops only these two new objects; it is documentation, not a command to execute.

Set `http.session_ttl: 12h` and `http.secure_cookies: false` in local/Docker examples; document that production must enable secure cookies.

- [ ] **Step 4: run GREEN**

Run: `cd backend && go test ./internal/models ./internal/config`
Expected: PASS.

### Task 2: PostgreSQL staff/session repository

**Files:**
- Create: `backend/internal/repository/postgres/auth_repository.go`
- Test: `backend/internal/repository/postgres/auth_repository_integration_test.go`

**Interfaces:**
- Consumes: Task 1 models and errors.
- Produces:

```go
type AuthDB interface {
    Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
    QueryRow(context.Context, string, ...any) pgx.Row
}
func NewAuthRepo(db AuthDB) *AuthRepo
func (r *AuthRepo) GetAdminUserByLogin(ctx context.Context, login string) (*models.AdminUser, error)
func (r *AuthRepo) CreateAdminSession(ctx context.Context, session *models.AdminSession) error
func (r *AuthRepo) GetAdminSessionByTokenHash(ctx context.Context, hash []byte) (*models.AdminSession, error)
func (r *AuthRepo) DeleteAdminSession(ctx context.Context, hash []byte) error
```

- [ ] **Step 1: test-author writes failing transaction-scoped integration tests**

Tests migrate the test schema, begin a transaction, construct `NewAuthRepo(tx)`, and roll back in cleanup. Cover: insert staff fixture then retrieve by login; create session then retrieve it joined to user; missing user/session returns `errs.ErrUnauthorized`; expired session is not returned; deleting a token hash makes subsequent lookup unauthorized. No `TRUNCATE` or fixture `DELETE`.

- [ ] **Step 2: run RED without a database**

Run: `cd backend && go test ./internal/repository/postgres -run 'TestAuthRepo'`
Expected: compile FAIL because `AuthRepo` is absent; with no `DATABASE_URL`, implemented tests later SKIP cleanly.

- [ ] **Step 3: implement parameterized repository queries**

Select only auth columns. Session lookup joins `admin_users` and requires `expires_at > now()`. Missing rows map to `ErrUnauthorized`; infrastructure errors retain their cause and repository `fc` wrappers.

- [ ] **Step 4: run GREEN**

Run: `cd backend && go test ./internal/repository/postgres -run 'TestAuthRepo'`
Expected: PASS when a safe disposable `DATABASE_URL` is explicitly supplied; otherwise SKIP with exit 0. Do not run the existing truncating suite against real data.

### Task 3: Opaque-token authentication service

**Files:**
- Create: `backend/internal/service/auth.go`
- Test: `backend/internal/service/auth_test.go`

**Interfaces:**
- Consumes: Task 2 repository methods through a small local `AuthRepository` interface.
- Produces:

```go
type AuthResult struct {
    User *models.AdminUser
    Token string
    ExpiresAt time.Time
}
func NewAuthService(repo AuthRepository, logger *slog.Logger, sessionTTL time.Duration) *AuthService
func (s *AuthService) Login(ctx context.Context, login string, password string) (*AuthResult, error)
func (s *AuthService) Authenticate(ctx context.Context, rawToken string) (*models.AdminUser, error)
func (s *AuthService) Logout(ctx context.Context, rawToken string) error
```

- [ ] **Step 1: test-author writes failing service tests with an in-memory repository**

Cover valid login, unknown login, wrong password, blank/oversized input, token randomness, database token equals SHA-256 rather than raw token, valid/expired/malformed authentication, and logout revocation. Use an injectable unexported `now` function in same-package tests.

```go
assert.NotEqual(t, first.Token, second.Token)
assert.NotEqual(t, first.Token, string(repo.lastSession.TokenHash))
assert.ErrorIs(t, err, errs.ErrUnauthorized)
```

- [ ] **Step 2: run RED**

Run: `cd backend && go test ./internal/service -run 'TestAuth'`
Expected: FAIL because the auth service does not exist.

- [ ] **Step 3: implement authentication**

Trim login whitespace, require non-empty login/password and passwords no longer than bcrypt's 72-byte limit. Compare bcrypt hashes; for unknown users compare against a fixed dummy bcrypt hash so the paths remain similar. Return generic `ErrUnauthorized` for either credential failure.

Generate 32 bytes from `crypto/rand`, encode with `base64.RawURLEncoding`, store SHA-256 bytes, and expire at `now().UTC().Add(sessionTTL)`. Reject empty/malformed tokens and expired sessions as unauthorized. Log operational failures without credentials or tokens.

- [ ] **Step 4: run GREEN**

Run: `cd backend && go test ./internal/service -run 'TestAuth'`
Expected: PASS.

### Task 4: Auth HTTP API, middleware, rate limiting, and admin route protection

**Files:**
- Create: `backend/internal/transport/http/dto/auth.go`
- Create: `backend/internal/transport/http/handler/auth.go`
- Create: `backend/internal/transport/http/middleware/auth.go`
- Create: `backend/internal/transport/http/middleware/login_rate_limit.go`
- Modify: `backend/internal/transport/http/handler/errors.go`
- Modify: `backend/internal/transport/http/handler/news.go`
- Modify: `backend/internal/transport/http/middleware/cors.go`
- Modify: `backend/cmd/main.go`
- Modify: `backend/README.md`
- Test: `backend/internal/transport/http/handler/auth_test.go`, `backend/internal/transport/http/middleware/auth_test.go`, `backend/internal/transport/http/handler/news_test.go`

**Interfaces:**
- Produces: `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/auth/me` and JSON user fields `id`, `login`, `role`.
- Produces: `middleware.RequireAuth(auth Authenticator) gin.HandlerFunc`, `middleware.RequireRoles(roles ...models.Role) gin.HandlerFunc`, `middleware.CurrentUser(c *gin.Context) (*models.AdminUser, bool)`.
- Cookie: `mandat_admin_session`, `HttpOnly`, `SameSite=Lax`, `Path=/`, configurable `Secure`, 12-hour expiry.

- [ ] **Step 1: test-author writes failing HTTP and middleware tests**

Cover login `200` plus cookie attributes and no token in JSON; invalid credentials `401`; `/me` `200`; logout `204` and cleared cookie; missing/malformed/expired/revoked session `401`; unsupported role `403`. Table-drive every admin news method/path and assert unauthenticated requests never call the service, while both allowed roles reach it.

Add CORS preflight assertions for `Access-Control-Allow-Credentials: true` and the configured origin. Add a limiter test: ten login attempts per IP per five minutes pass through, the eleventh returns `429` without calling auth.

- [ ] **Step 2: run RED**

Run: `cd backend && go test ./internal/transport/http/...`
Expected: FAIL because auth routes/middleware are absent.

- [ ] **Step 3: implement transport and wiring**

`NewAuthHandler` accepts the auth service and secure-cookie setting. `RequireAuth` reads only the session cookie and stores the authenticated user in Gin context. `RequireRoles` rejects roles not in its explicit allowlist.

Change `NewNewsHandler` to accept the auth/role middleware and apply it to the whole `/api/admin` group; update production wiring and test calls. Add centralized `401`, `403`, and `429` error mappings. Enable credentialed CORS for the existing origins. Use a small in-memory per-IP fixed-window limiter (10 requests/5 minutes) for login only.

Wire `AuthRepo`, `AuthService`, auth handler, and protected news handler in `cmd/main.go`. Update README with endpoints, cookie policy, and a SQL-only account insert containing a placeholder bcrypt hash and no real credentials.

- [ ] **Step 4: run GREEN**

Run: `cd backend && go test ./internal/transport/http/...`
Expected: PASS.

### Task 5: Frontend auth API, provider, and protected route

**Files:**
- Create: `frontend/src/api/auth.ts`
- Create: `frontend/src/context/auth-context.ts`
- Create: `frontend/src/components/auth/AuthProvider.tsx`
- Create: `frontend/src/components/auth/RequireAuth.tsx`
- Create: `frontend/src/hooks/use-auth.ts`
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/api/news.ts`
- Modify: `frontend/src/App.tsx`
- Test: `frontend/tests/auth-api.test.ts`, `frontend/tests/auth-routing.test.tsx`

**Interfaces:**
- Produces: `AuthUser { id: string; login: string; role: 'administrator' | 'moderator' }`.
- Produces: `login(login, password)`, `logout()`, `getCurrentUser()` using `credentials: 'include'`.
- Produces: `AuthProvider`, `RequireAuth`, `useAuth()` with `{ user, loading, login, logout, clearUser }`.

- [ ] **Step 1: test-author writes failing API and route tests**

Assert exact auth URLs, methods, JSON body, and credentials for every call. Assert admin news requests include credentials. Render `RequireAuth` with static context values: loading shows status, no user redirects to `/admin/login`, both roles render children. Assert a simulated admin `401` dispatches `mandat:unauthorized` and does not expose response internals.

- [ ] **Step 2: run RED**

Compile/run the frontend node tests using the established temporary test tsconfig approach.
Expected: FAIL because auth modules/fields are absent.

- [ ] **Step 3: implement provider and route protection**

`AuthProvider` checks `/me` on mount and listens for `mandat:unauthorized` to clear the user. Wrap routes in the provider; register `/admin/login` and wrap `/admin` in `RequireAuth`. Keep tokens out of storage and authorization headers. Add credentials to admin API requests and dispatch the unauthorized event only for admin `401` responses.

- [ ] **Step 4: run GREEN**

Run the new frontend auth tests.
Expected: PASS.

### Task 6: Login screen and admin identity/logout

**Files:**
- Create: `frontend/src/pages/Login.tsx`
- Modify: `frontend/src/pages/Admin.tsx`
- Modify: `frontend/README.md`
- Test: `frontend/tests/admin-auth-ui.test.tsx`

**Interfaces:**
- Consumes: Task 5 `useAuth()` and routes.
- Produces: working `/admin/login` form and visible current login/role plus logout action in the existing admin header.

- [ ] **Step 1: test-author writes failing UI tests**

Using the project's SSR test style, assert labels for login/password, submit button, pending/error regions, no password value persisted in markup, and Ukrainian generic error text. Assert the admin header renders the authenticated login/localized role and a logout button when supplied with auth context.

- [ ] **Step 2: run RED**

Run: the new `admin-auth-ui` frontend test.
Expected: FAIL because the login page and identity controls are absent.

- [ ] **Step 3: implement UI in existing Mandat styling**

On successful login, replace history to `/admin`; an already authenticated visitor to `/admin/login` is redirected to `/admin`. Disable the form while pending and show generic errors. Logout waits for the backend call, clears provider state, and navigates to `/admin/login`. Keep all existing news management available to both roles.

Document the frontend login flow and credentialed API requirement.

- [ ] **Step 4: run GREEN and full verification**

Run: new frontend tests, `cd frontend && npm run lint && npm run build`, and safe backend checks `cd backend && gofmt -w <changed-go-files> && go test ./... && go vet ./...`.
Expected: PASS. If `DATABASE_URL` is set, first obtain approval for the existing `TRUNCATE news` integration helper or run with it unset so integration tests skip.

- [ ] **Step 5: request independent review**

Dispatch `reviewer` for the complete auth diff; require a `.superpowers/reviews/*.txt` report. Fix Critical/Important findings through the primary agent and send any test changes back to `test-author`.
