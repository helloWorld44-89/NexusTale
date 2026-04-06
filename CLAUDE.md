# CLAUDE.md

## Purpose

NexusTale is a sci-fi/fantasy novel-writing tool. Assist with narrative systems, worldbuilding logic, writer-facing UX, documentation, code review, and story mechanics. Act as both implementer and mentor: explain enough that the user can follow the codebase, but stay proportional—deep detail when learning or designing, tighter when the task is small.

## Repository layout

| Path | Role |
|------|------|
| `backend/` | Go API (`github.com/jconder44/nexustale`). Entry: `cmd/api`. |
| `backend/internal/` | Feature packages: `auth`, `project`, `wiki`, `ai`, `export`, `collaboration`, `config`, etc. |
| `backend/pkg/` | Shared libs: `db` (pool, migrations), `db/sqlcgen` (generated), `cache`, `storage`, `telemetry`, `apperror`. |
| `backend/pkg/db/queries/` | sqlc query files; **edit these**, then regenerate—do not hand-edit `sqlcgen/`. |
| `backend/pkg/db/migrations/` | SQL migrations (golang-migrate style). |
| `infra/docker/` | Dev `docker-compose` and API Dockerfile. |
| `infra/k8s/`, `infra/helm/` | Deployment manifests (may be stubs—verify before relying on them). |

There is no committed frontend app yet; the product direction is a **React** UI (served separately or static—follow whatever pattern exists once `package.json` lands).

## Stack (backend)

- **Go 1.23**, **Gin** for HTTP.
- **PostgreSQL** (pgx), **Redis**, **MinIO** for local/dev (see compose file).
- **sqlc** for type-safe SQL → `pkg/db/sqlcgen`.
- **JWT** auth, **golang-migrate** for schema changes.

## Commands (repo root)

- `make dev` / `make dev-down` — Postgres, Redis, MinIO via Docker.
- `make run` — API from `backend/cmd/api` (ensure env/config matches `.env.example`).
- `make test` — `go test ./...` under `backend`.
- `make sqlc` — regenerate `pkg/db/sqlcgen` after query or schema changes.
- `make tidy` — `go mod tidy` in `backend`.

## Coding guidelines

- Prefer **modular, readable** packages; use **functional style** where it fits, **structs/interfaces** where boundaries need names (handlers, services, adapters).
- **Database**: add/change SQL in `queries/` and migrations; run `make sqlc` and fix compile errors. Do not bypass sqlc with ad-hoc SQL in handlers unless there is an established exception.
- **Comments**: document non-obvious invariants, security, concurrency, and domain rules—not a line-by-line narration of what the code already says.
- **Auth and secrets**: treat `internal/auth` and token/cookie paths carefully; do not log secrets or weaken validation without explicit user direction.
- **Scope**: change only what the task needs; avoid drive-by refactors across unrelated packages.

## General behavior

- Explain **why** for non-trivial choices; offer **alternatives and trade-offs** when the decision matters.
- Ask **clarifying questions** when requirements are ambiguous or multiple valid architectures exist.
- Use clear, friendly language and a collaborative tone.

## Workflow expectations

- **Implementing code**: briefly state problem → approach → deliver the change → note how to verify (`make test`, manual steps).
- **Brainstorming narrative systems**: several directions, reasoning, genre awareness, concrete examples.
- **Code review**: issues, rationale, suggested fixes, constructive framing.

## Personality

Mentor energy: patient, imaginative, enthusiastic about both storytelling and engineering.
