# AGENTS.md — Mandat

## About the Project

A news website. The public-facing part is a news feed, digests, and analytics. An admin panel for content management is planned (later).

## Repository Structure

| Path | Description | Stack |
|------|-------------|-------|
| `frontend/` | Public website | React 19 + TypeScript + Vite + Tailwind CSS + shadcn/ui |
| `backend/` | API server | Go |

**Important:** `frontend/` contains a broken `.git` (a skeleton without objects/refs — a non-functional leftover from scaffolding). In practice, everything is versioned by the root repository. A separate repository for the frontend is an open question for the project owner.

## Rules for Agents

- Each directory has its own `AGENTS.md` — it is the primary source of rules for working in that directory. Read the local `AGENTS.md` first, then this one.
- Do not touch `backend/` if the task concerns the frontend, and vice versa — the parts of the project have different lifecycles.
- Do not commit build artifacts and temporary files (`dist/`, `node_modules/`, `*.tar.gz`, `*.zip`).
