# Phase A — product skeleton (MVP vertical)

**Goal:** A contributor or early user can run the stack locally, sign in, manage projects/chapters/scenes from a **React** UI, see **basic Git history** for narrative saves, and use **Wiki v1** (typed entities + list/detail via API). This matches [Phase A in PROJECT_PLAN.md](../PROJECT_PLAN.md#7-phased-delivery-suggested).

**Exit criteria (Phase A done):**

- README documents local dev; `make dev` + `make run` + frontend dev server are enough to try the flow.
- At least one **new** vertical beyond raw projects is live: **wiki CRUD** (or explicitly scoped subset) behind auth, with tests.
- React app: **login/register**, **project list**, **scene editor** (load/save scene content via existing APIs).
- **Git:** user-visible **snapshot or commit list** for a project (API + minimal UI), even if read-only first.

---

## A0 — Documentation & contracts

| # | Task | Acceptance criteria |
|---|------|---------------------|
| A0.1 | **README** at repo root | Prerequisites (Go, Node, Docker), clone → `make dev` → copy `.env.example` → `make run`, link to `backend/.env.example`, smoke: `GET /healthz`. |
| A0.2 | **OpenAPI stub** (`openapi.yaml` or under `docs/`) | Documents `/api/v1/auth/*` and `/api/v1/projects/*` paths used by Phase A frontend; can be hand-maintained initially. |
| A0.3 | **Infra honesty** | Either wire `pkg/cache` / `pkg/storage` for first consumer **or** add a short “Redis/MinIO optional until …” section in README + ROADMAP. |

---

## A1 — Backend: Wiki v1

| # | Task | Acceptance criteria |
|---|------|---------------------|
| A1.1 | **Schema** | Migrations for wiki: e.g. `wiki_entities` (project_id, type, title, body/summary, metadata JSON, timestamps), indexes for `project_id` + `type`. |
| A1.2 | **sqlc** | Queries in `pkg/db/queries/`; `make sqlc` clean; no hand-edits in `sqlcgen/`. |
| A1.3 | **Service + handler** | `internal/wiki`: CRUD (create, get, list by project, update, delete) with **project ownership** checks (same pattern as `internal/project`). |
| A1.4 | **Routes** | `RegisterRoutes` on `/api/v1/projects/:id/wiki/...` (or consistent prefix); registered in `cmd/api/main.go` behind `auth.RequireAuth`. |
| A1.5 | **Tests** | Handler or integration tests for happy path + 403 for wrong user; `make test` green. |

**Entity types (v1):** start with a string `type` enum in code or DB check: `character`, `location`, `faction`, `magic`, `lore`, `plot_note` — enough to back “world, characters, magic, timeline, plot” later without redesigning the table.

---

## A2 — Backend: Git visibility (minimal)

| # | Task | Acceptance criteria |
|---|------|---------------------|
| A2.1 | **API** | Endpoints e.g. `GET /api/v1/projects/:id/git/log` and/or `GET .../git/branches` using existing `go-git` layer in `internal/project`. |
| A2.2 | **Auth** | Same access as project (owner/collaborator when collaborators exist; until then owner only). |
| A2.3 | **Tests** | At least one test with a temp repo or existing testutil pattern. |

*Defer:* branch create/merge UI until Phase B/C if needed to ship A faster.

---

## A3 — Frontend (React)

| # | Task | Acceptance criteria |
|---|------|---------------------|
| A3.1 | **Bootstrap** | Vite + React + TypeScript under `frontend/`; `npm run dev` / `npm run build`; `.env.example` for `VITE_API_URL`. |
| A3.2 | **API client** | Fetch wrapper with base URL, attach Bearer from login response; refresh flow aligned with backend (refresh cookie or body — pick one and document). |
| A3.3 | **Auth screens** | Register + login forms; store session; redirect to dashboard when valid. |
| A3.4 | **Project list** | Lists user projects; create project; navigate to project detail. |
| A3.5 | **Scene editor** | Pick chapter → scene; load content from API; save (debounced or explicit button); handles errors. |
| A3.6 | **Wiki list/detail** | List entities for project; create/edit simple entity; calls A1 API. |
| A3.7 | **Git panel (minimal)** | On project page: show last N commits from A2 API (read-only list). |

---

## A4 — Quality bar for Phase A

| # | Task | Acceptance criteria |
|---|------|---------------------|
| A4.1 | **CI** | Single workflow or documented script: `go test ./...`, frontend `npm run build` (and lint if configured). |
| A4.2 | **CLAUDE.md / ROADMAP** | `frontend/` mentioned in repo layout; Phase A checkboxes in [ROADMAP.md](../../ROADMAP.md) updated as items ship. |

---

## Suggested order (parallelizable)

1. A0.1 → A0.3 (unblocks everyone)  
2. A1.1–A1.5 (wiki API) in sequence  
3. A3.1–A3.3 (shell + auth) while A1 is in progress  
4. A3.4–A3.5 (projects/scenes)  
5. A2.* (Git log API) + A3.7  
6. A3.6 (wiki UI)  
7. A0.2 OpenAPI + A4.*

---

## Out of scope for Phase A

- Real-time collaboration (WebSocket / CRDT)  
- AI routes beyond a possible stub  
- EPUB/Scrivener export jobs  
- Map builder, image generation, novel guide wizard  
- Production Helm/K8s (document compose-only is fine)

---

*Linked from [ROADMAP.md](../../ROADMAP.md) and [PROJECT_PLAN.md](../PROJECT_PLAN.md).*
