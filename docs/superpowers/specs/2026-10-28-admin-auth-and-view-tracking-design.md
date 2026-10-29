# Admin authentication and public news view tracking

## Intent

Protect the existing admin panel with real authentication and role-aware middleware, and make the public article view counter reflect visitor activity instead of remaining a static number.

The editorial roles are deliberately small: a moderator manages news, and an administrator currently has the same news-management permissions. Staff accounts are provisioned only through SQL; there is no account-management UI or public registration.

## Decisions from the conversation

- Roles are `administrator` and `moderator`.
- Moderators can manage news. Administrators have the same currently available permissions; no separate administrator-only UI is introduced.
- Staff accounts are added only through SQL.
- A visitor can add at most one view to a particular article per hour. Another visit after that hour can add another view.
- The updated view count is shown on the public article page. No new view analytics are added to the admin panel.
- An authenticated admin session lasts 12 hours, including across browser restarts. Logout ends it earlier.

## Goals

- Add a login screen and protect the `/admin` frontend route and every `/api/admin/*` endpoint.
- Support login, logout, and current-user discovery.
- Store staff credentials and revocable server-side sessions in PostgreSQL.
- Keep passwords and raw session tokens out of the database, logs, browser storage, and API responses.
- Record public article views atomically with a one-hour per-visitor cooldown.
- Preserve the existing public article and news-management behavior for authorized users.

## Non-goals

- Registration, password reset, account administration, role administration, or account-management endpoints.
- Administrator-only news permissions that do not exist in the current product.
- Unique-lifetime visitor counts, IP-based fingerprinting, detailed analytics, or an admin analytics dashboard.
- JWT authentication, OAuth, SSO, CSRF-token infrastructure beyond the protections described below, or new frontend state libraries.

## Architecture and data model

Follow the existing Clean Architecture boundaries: domain models, services with small repository interfaces, PostgreSQL repositories, thin Gin handlers, and explicit constructor wiring in `cmd/main.go`.

### Staff accounts

Create an `admin_users` table with:

- `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`;
- `login TEXT NOT NULL UNIQUE` (normalized length and format constraints where practical);
- `password_hash TEXT NOT NULL`;
- `role TEXT NOT NULL CHECK (role IN ('administrator', 'moderator'))`;
- `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`;
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`.

Only the bcrypt password hash is stored. A plaintext-password column is intentionally not created. Document an SQL `INSERT` example with a placeholder bcrypt hash and without real credentials.

### Authenticated sessions

Create an `admin_sessions` table with:

- `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`;
- `user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE`;
- `token_hash BYTEA NOT NULL UNIQUE`;
- `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`;
- `expires_at TIMESTAMPTZ NOT NULL`;
- an index on `expires_at` for future cleanup.

Login verifies the login and bcrypt hash using `golang.org/x/crypto/bcrypt`, creates a cryptographically random 32-byte token, stores only its SHA-256 hash, and sets the raw token in a `mandat_admin_session` cookie. The cookie is `HttpOnly`, `SameSite=Lax`, `Path=/`, and has a 12-hour lifetime. `Secure` is configurable so local HTTP development remains usable while production enables it.

Session lookup joins the hashed token to a non-expired session and its staff user. Logout deletes the current session row and expires the cookie. Expired sessions are treated as unauthenticated; cleanup can be added later without changing behavior.

### News view cooldown

Create a `news_viewers` table with:

- `news_id UUID NOT NULL REFERENCES news(id) ON DELETE CASCADE`;
- `viewer_id UUID NOT NULL`;
- `last_viewed_at TIMESTAMPTZ NOT NULL`;
- primary key `(news_id, viewer_id)`;
- an index on `last_viewed_at` for possible future cleanup.

The backend issues a random anonymous `mandat_viewer` UUID cookie when it is absent or invalid. Public article-detail `GET` responses issue it before the frontend starts view recording, so simultaneous follow-up requests share one visitor UUID; the view endpoint uses the same middleware as a fallback. The cookie is `HttpOnly`, `SameSite=Lax`, scoped to `/api/news`, and lasts one year; production uses `Secure`. No IP address, user agent, or other fingerprint is stored.

## HTTP API

### Authentication

- `POST /api/auth/login` accepts `{ "login", "password" }`, returns the current user's UUID, login, and role, and sets the session cookie. Invalid credentials return the same generic `401` response whether the login or password is wrong.
- `POST /api/auth/logout` requires a valid session, revokes it, clears the cookie, and returns `204`.
- `GET /api/auth/me` requires a valid session and returns UUID, login, and role.

Apply a small per-IP in-memory login rate limit to slow password guessing without introducing external infrastructure. Do not log passwords, session tokens, or complete login payloads.

### Admin authorization

Authentication middleware loads the session and puts only the authenticated user identity and role into the Gin request context. Authorization middleware permits both existing roles on `/api/admin/*`. Missing, malformed, expired, or revoked sessions return `401`; a valid session with an unsupported role returns `403`.

Both roles can currently list, create, edit, and delete news because news management is the moderator's stated responsibility and no distinct administrator-only operation exists yet.

### Public views

- `POST /api/news/:slug/view` records a view for a published article and returns `{ "views": number }` with the current total.
- A missing or draft article returns the existing public `404` response.
- The repository atomically updates `news_viewers` only when the visitor has no row or their previous accepted view is at least one hour old, increments `news.views` only for an accepted view, and returns the resulting count. Concurrent duplicate requests must not bypass the cooldown.
- The existing `GET /api/news/:slug` JSON contract remains unchanged, but its response may set the anonymous viewer cookie. Public article-detail and view requests use credentials so cross-origin development cookies work.

CORS allows credentials only for the already configured frontend origins. It does not switch to wildcard origins.

## Frontend behavior

### Admin login and session lifecycle

Add `/admin/login` with a compact login/password form consistent with the existing Mandat admin styling. It has accessible labels, pending state, generic invalid-credentials messaging, and no password persistence.

The `/admin` route checks `GET /api/auth/me` before showing news management:

- pending checks show a loading state;
- unauthenticated users are redirected to `/admin/login`;
- authenticated administrators and moderators see the existing admin page and their login/role;
- successful login returns the user to `/admin`;
- logout calls the backend and returns to the login screen;
- an admin API `401` clears the local authenticated state and redirects to login.

All authentication and admin requests use `credentials: 'include'`. Cookies are never copied into `localStorage` or JavaScript-managed authorization headers.

### Public article views

After the public article loads successfully, the frontend calls the view endpoint with credentials and replaces the displayed view count with the returned current value. React development-mode duplicate effects and rapid repeated requests are harmless because the backend enforces the cooldown atomically.

A view-recording failure does not hide or break the article; the last successfully loaded count remains visible. The public article continues to render while the view request is pending.

## Configuration and operations

- Add configurable cookie-security and session-duration settings with safe documented defaults; update `config.example.yaml`, Docker configuration where required, and backend README.
- The chosen session duration is 12 hours and the view cooldown is one hour. These are named service/configuration values, not scattered literals.
- Add one forward migration for the new tables and indexes. Do not execute destructive migration-down operations or delete existing data.
- Document SQL-only staff creation with a bcrypt-hash placeholder and no committed real login, password, token, or secret.

## Testing and verification

The `test-author` subagent writes RED tests before the primary agent implements each production task. Coverage includes:

- valid and invalid login, bcrypt verification, session issuance and expiry, logout, and `/me`;
- authentication and role middleware, including `401` versus `403`;
- protection of every admin news endpoint;
- first view, repeat view within one hour, accepted view after one hour, concurrent duplicate views, and drafts;
- frontend route guarding, credentials handling, login/logout behavior, and public view-count updates/failure tolerance;
- PostgreSQL integration tests for atomic cooldown behavior where they can run without destructive cleanup or after explicit approval for any destructive fixture operation.

Run `gofmt`, `go test ./...`, `go vet ./...`, frontend `npm run lint`, and frontend `npm run build`. Inspect integration-test cleanup before executing it; no database data or schema may be deleted without specific user permission.

## Implementation calls made by the designer

- Server-side opaque sessions are preferred over JWT so logout and expiration are immediately enforceable without a JWT secret or token-refresh flow.
- A random anonymous cookie is preferred over IP/user-agent tracking for the one-hour view cooldown.
- View recording is a separate `POST` so fetching article content remains read-only.
- Both roles receive the same current news permissions; the role check still establishes the boundary for future differentiation.
- Login rate limiting stays in memory and per IP to avoid a new service or dependency.
