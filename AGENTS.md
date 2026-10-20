# AGENTS.md — Mandat

## About the Project

A news website. The public-facing part is a news feed, digests, and analytics. An admin panel for content management is planned (later).

## Repository Structure

| Path | Description | Stack |
|------|-------------|-------|
| `frontend/` | Public website | React 19 + TypeScript + Vite + Tailwind CSS + shadcn/ui |
| `backend/` | API server | Go |

**Git:** a single repository — `git@github.com:MaxAixs/mandat.git` (branch `main`). Frontend and backend are versioned together; there is no separate frontend repository.

## Rules for Agents

- Each directory has its own `AGENTS.md` — it is the primary source of rules for working in that directory. Read the local `AGENTS.md` first, then this one.
- Do not touch `backend/` if the task concerns the frontend, and vice versa — the parts of the project have different lifecycles.
- Do not commit build artifacts and temporary files (`dist/`, `node_modules/`, `*.tar.gz`, `*.zip`).
