# New Categories + Seed Content Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add «Політика» and «Регіональні новини» categories across backend and frontend, wire navigation routes (plus the missing «Інтерв'ю» route), and seed 6 published articles per new/existing rubric (Політика, Регіональні новини, Інтерв'ю) with source and cover image.

**Architecture:** Backend owns the category enum (`models.Category`) plus a `CHECK` migration; frontend mirrors it in the `Category` union type, `CATEGORIES`, `NAV_LINKS`, and `Route`s. Seed content goes through the real admin HTTP API (JSON `image_url` → service downloads it to R2 via `imgdownloader`), so covers, slugs, and validation behave exactly like production input.

**Tech Stack:** Go (Gin, pgx, goose), React 19 + react-router v7, R2 via S3 API, picsum.photos seeded URLs as cover sources (free-licensed photos).

**Spec:** Owner decisions 2026-11-10: (1) add the two missing categories rather than reusing existing ones; (2) covers come from the internet — no AI-generation provider exists in the project.

## Global Constraints

- Backend: `gofmt` clean, `go test ./...` green, `go vet ./...` clean; `fc` const + `%w` wrapping in service/repo layers; no secrets in code.
- Frontend: `npm run lint` and `npm run build` (`tsc -b`) green; Tailwind classes only, `mandat-*` theme colors, no hardcoded hex.
- Tests live next to code; the primary agent NEVER edits `*_test.go` / `*.test.*` — all test changes go to the `test-author` subagent.
- NEVER commit or push on your own initiative — the plan's steps end at verification, no commit steps.
- Destructive DB ops need explicit owner permission — seeding uses INSERTs only; no TRUNCATE/DELETE of existing data (temp seed admin is created and removed by the plan).

## Review Focus

- A category present in the backend `Valid()` but missing from the DB `CHECK` (or vice versa) breaks writes at runtime — Task 1 tests both layers together via the API.
- A category missing from any frontend `Record<Category, …>` (`mainEventIconByCategory`) breaks `tsc -b` — Task 2 lists every `Category`-keyed site.
- `image_url` download failures (unreachable host, >10 MB, redirect loop) turn a seed POST into 4xx/5xx — Task 3 verifies each article's `imageUrl` is an R2 URL, not the picsum URL.
- Seed texts are DEMO content (interviews use fictional persons, no fabricated quotes from real people, no sensational claims) and must be replaced before production — Task 3 flags this in its verification step.
- Slug collisions across the 18 seeds (similar titles) — the service auto-suffices (`-2`, …), verification lists all 18 slugs unique.

---

### Task 1: Backend — new categories in domain + DB check

**Files:**
- Modify: `backend/internal/models/news.go:26-42` (add `CategoryPolitics Category = "Політика"`, `CategoryRegional Category = "Регіональні новини"`, extend `Valid()`)
- Create: `backend/migrations/00010_new_rubrics.sql` (Up: drop `news_category_check`, re-add with 7 values; Down: revert to the 5-value check from `00003`)
- Test: `backend/internal/service/news_test.go`, `backend/internal/transport/http/handler/news_test.go`, `backend/internal/transport/http/handler/public_news_test.go` (test-author only)

**Interfaces:**
- Consumes: existing `models.Category` enum + `Valid()`; goose `*.sql` convention (`-- +goose Up/Down`).
- Produces: `models.CategoryPolitics`, `models.CategoryRegional` valid in service validation, admin/public handlers, and the DB check.

- [ ] **Step 1: test-author writes the RED tests.** New cases: `Create`/`List`/`ListPublished` accept `Політика` and `Регіональні новини`; existing unknown-category cases that use `"Політика"` as the invalid value switch to a still-unknown value (`"Фантастика"`). Expected: FAIL (unknown category).
- [ ] **Step 2: Run backend unit tests.** Run: `go test ./internal/service/ ./internal/transport/...`. Expected: new cases FAIL, everything else PASS.
- [ ] **Step 3: Implement the model + migration.** Add the two constants, extend `Valid()`; write `00010_new_rubrics.sql` mirroring the `ALTER TABLE … DROP CONSTRAINT / ADD CONSTRAINT` pattern of `00003`.
- [ ] **Step 4: Run all backend checks.** Run: `gofmt -l .`, `go vet ./...`, `go test ./...` (unit; integration tests need `DATABASE_URL` + separate `mandat_test` DB per README). Expected: all PASS, including the migrated CHECK (verify: insert with `category='Фантастика'` fails, `'Політика'` passes — via a throwaway row in the isolated test DB only, rolled back).

### Task 2: Frontend — wire categories into types, nav, routes, filters

**Files:**
- Modify: `frontend/src/types/index.ts:5` (extend `Category` union with `'Політика' | 'Регіональні новини'`)
- Modify: `frontend/src/constants/index.ts:14,17-25` (`CATEGORIES` += 2; `NAV_LINKS`: `Політика` → `/politics`, `Регіональні новини` → `/regional`, `Інтерв'ю` → `/interview`)
- Modify: `frontend/src/App.tsx:18-19` (add `/politics`, `/regional`, `/interview` routes rendering `CategoryPage`)
- Modify: `frontend/src/pages/Feed.tsx:15-16` (filter options += 2 new + `Інтерв'ю` if missing — check file first)
- Modify: `frontend/src/pages/Admin.tsx:80-86` (`mainEventIconByCategory: Record<Category, LucideIcon>` += `Політика: Landmark`, `'Регіональні новини': MapPin`; `tsc` enforces completeness)

**Interfaces:**
- Consumes: `models.Category*` values from Task 1 (exact Ukrainian strings must match byte-for-byte).
- Produces: browsable `/politics`, `/regional`, `/interview` pages; admin/feed filters aware of the new rubrics.

- [ ] **Step 1: Implement the frontend changes** (no new tests exist for these files; type-check is the gate).
- [ ] **Step 2: Run frontend checks.** Run: `npm run lint`, `npm run build`. Expected: both PASS (a missing `Record<Category,…>` key fails `tsc -b` here, not later).
- [ ] **Step 3: Click-verify in dev.** Open `/politics`, `/regional`, `/interview` — each renders the category heading with red stripe and the empty-state text (no articles yet).

### Task 3: Seed 18 articles via the admin API

**Files:**
- Create (git-ignored scratch, deleted after): `/tmp/opencode/seed/` — bcrypt helper (`go run` inside `backend/` module using `golang.org/x/crypto/bcrypt`, cost 12), `seed.sh`, `articles.json` (18 entries: category, title, description, text 3–6 paragraphs, language `uk`, status `published`, `source` + `source_url`, `image_url: https://picsum.photos/seed/<slug>/1200/800`).
- Modify: none in the repo.

**Interfaces:**
- Consumes: admin API (`POST /api/auth/login` → `mandat_admin_session` cookie; `POST /api/admin/news` JSON with `category,title,description,text,language,status,source,source_url,image_url`); backend running per the local-dev workaround (host binary + `127.0.0.1` config) or Compose if the network is fixed.
- Produces: 18 published articles with R2 covers; temp admin removed afterwards.

- [ ] **Step 1: Create temp seed admin via SQL.** Generate bcrypt hash, `INSERT INTO admin_users (login, password_hash, role)` with `login='seed_bot'`, `role='admin'` (INSERT only — allowed without extra permission).
- [ ] **Step 2: POST the 18 articles.** 6 × `Політика`, 6 × `Регіональні новини`, 6 × `Інтерв'ю` (interviewees are fictional persons; texts neutral/evergreen, no real-person quotes). Covers: distinct `picsum.photos/seed/<n>/1200/800` URLs; the service fetches them into R2 automatically.
- [ ] **Step 3: Verify.** `GET /api/news?category=<each>&limit=10` returns 6 items per rubric; every item's `imageUrl` starts with the R2 public base (not picsum); all slugs unique; homepage + `/politics` + `/regional` render cards with images.
- [ ] **Step 4: Cleanup.** `DELETE FROM admin_users WHERE login='seed_bot'` (single-row delete of the plan's own temp row); remove `/tmp/opencode/seed/`.
- [ ] **Step 5: Report.** Tell the owner the seed content is demo filler (replace before production) and list the 18 slugs.

### Task 4: Review + fix

- [ ] **Step 1: Dispatch the `reviewer` subagent** (model `opencode-go/deepseek-v4.1-flash`) over the backend+frontend diff; report lands in `.superpowers/reviews/*.txt`.
- [ ] **Step 2: Fix the findings** (test fixes go back to `test-author`); re-run `go test ./...`, `npm run lint`, `npm run build`.
