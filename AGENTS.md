# AGENTS.md — Mandat

## About the Project

A news website. The public-facing part is a news feed, digests, and analytics. An admin panel for content management is planned (later).

## Repository Structure

| Path | Description | Stack |
|------|-------------|-------|
| `frontend/` | Public website | React 19 + TypeScript + Vite + Tailwind CSS + shadcn/ui |
| `backend/` | API server | Go |

**Git:** a single repository — `git@github.com:MaxAixs/mandat.git` (branch `main`). Frontend and backend are versioned together; there is no separate frontend repository.

## Models and Roles

The superpowers workflow is unchanged; only the model performing each role differs.

| Role | Model | Agent |
|------|-------|-------|
| Brainstorming, plans, code, coordination, fixing review findings | `opencode-go/kimi-k3.1-pro` | primary session |
| Writing tests (RED step of TDD) | `opencode/deepseek-v4.1-flash-free` | `test-author` subagent |
| Code review | `opencode/deepseek-v4.1-flash-free` | `reviewer` subagent |

- Tests are never written by the primary agent: dispatch `test-author` before implementing a task.
- `reviewer` writes its report to `.superpowers/reviews/*.txt` (git-ignored); the primary agent reads that file and fixes the findings.
- Directory-specific routing rules live in the local `AGENTS.md` (see `backend/AGENTS.md`).

## Rules for Agents

- Each directory has its own `AGENTS.md` — it is the primary source of rules for working in that directory. Read the local `AGENTS.md` first, then this one.
- Do not touch `backend/` if the task concerns the frontend, and vice versa — the parts of the project have different lifecycles.
- Do not commit build artifacts and temporary files (`dist/`, `node_modules/`, `*.tar.gz`, `*.zip`).

### Git commits

- Commit only on an explicit command; never commit or push on your own initiative. This applies to `frontend/` exactly as to `backend/`.
- Follow the same message conventions for both parts: start every message with `[+]` (new functionality or files), `[*]` (refactoring or behavior-preserving changes), or `[-]` (deleting code or files).
- Do not add a `frontend:` / `backend:` scope to the message — the changed files already show which part is meant, e.g. `[+] admin login and session context`.
- When a change touches both parts, make two separate commits — one for `frontend/`, one for `backend/` — each with its own type prefix.

## Database Safety

- Never delete, truncate, drop, overwrite, or otherwise destroy database data or schema objects without the user's explicit permission for that specific operation.
- This rule applies equally to manual SQL, migrations, scripts, tests, cleanup helpers, Docker commands, and commands that indirectly cause deletion. A general request to run tests or migrations is not permission to delete data.
- Before requesting permission, identify the exact database and operation, explain what will be affected, and inspect the command or test helper for destructive side effects.
- Never assume a database is disposable merely because it is local or used for development. Read-only queries such as `SELECT` do not require deletion approval.
