# Phase A — product skeleton (MVP vertical)

**Goal:** A contributor or early user can run the stack locally, sign in, manage projects/chapters/scenes from a **React** UI, see **basic Git history** for narrative saves, and use **Wiki v1** (typed entities + list/detail via API). This matches [Phase A in PROJECT_PLAN.md](../PROJECT_PLAN.md#7-phased-delivery-suggested).

**Exit criteria (Phase A done):**

- README documents local dev; `make dev` + `make run` + frontend dev server are enough to try the flow.
- At least one **new** vertical beyond raw projects is live: **wiki CRUD** (or explicitly scoped subset) behind auth, with tests.
- React app: **login/register**, **project list**, **scene editor** (load/save scene content via existing APIs).
- **Git:** user-visible **snapshot or commit list** for a project (API + minimal UI), even if read-only first.

---

## A0 — Documentation & contracts

| # | Task | Status | Acceptance criteria |
|---|------|--------|---------------------|
| A0.1 | **README** at repo root | ⬜ todo | Prerequisites (Go, Node, Docker), clone → `make dev` → copy `.env.example` → `make run`, link to `backend/.env.example`, smoke: `GET /healthz`. |
| A0.2 | **OpenAPI stub** (`docs/openapi.yaml`) | ⬜ todo | Documents `/api/v1/auth/*`, `/api/v1/projects/*`, wiki, and git routes used by Phase A frontend; can be hand-maintained initially. |
| A0.3 | **Infra honesty** | ⬜ todo | Add "Redis/MinIO provisioned but not yet consumed — activated in Phase B" note to README. Wire nothing yet. |
| A0.4 | **Full-stack compose** | ✅ done | `infra/docker/docker-compose.yml` runs postgres + redis + minio + api + frontend; `Dockerfile.frontend` multi-stage build; nginx proxies `/api/` to backend and handles SPA routing. |

---

## A1 — Backend: Wiki v1

| # | Task | Status | Acceptance criteria |
|---|------|--------|---------------------|
| A1.1 | **Schema** | ✅ done | Migration 005: `wiki_entities`, `wiki_relationships`, `wiki_magic_rules`, `wiki_timeline_events` with indexes. |
| A1.2 | **sqlc** | ✅ done | Queries in `pkg/db/queries/wiki.sql`; `make sqlc` clean; generated to `sqlcgen/wiki.sql.go`. |
| A1.3 | **Service + handler** | ✅ done | Full CRUD for entities (with hierarchy), relationships, magic rules, timeline events; autolink; graph endpoint. |
| A1.4 | **Routes** | ✅ done | `/api/v1/projects/:id/wiki/...` registered in `cmd/api/main.go` behind `auth.RequireAuth`. |
| A1.5 | **Tests** | ✅ done | 8 integration tests (entity CRUD, child entities, relationships, graph, magic rules, timeline events, autolink, unauthenticated). |
| A1.6 | **Timeline date update** | ✅ done | `PATCH /timeline/:tid` accepts `year`/`month`/`day` fields; era-only events supported. |

---

## A2 — Backend: Git visibility

| # | Task | Status | Acceptance criteria |
|---|------|--------|---------------------|
| A2.1 | **API** | ✅ done | `GET /git/status`, `POST /git/chronicle`, `GET /git/lore`, `GET /git/echo`, `GET /git/timelines`, `POST /git/timelines` (diverge), `POST /git/timelines/:name/travel`, `POST /git/timelines/:name/canonize`. |
| A2.2 | **Auth** | ✅ done | All git routes behind `RequireAuth`; owner-only via project lookup. |
| A2.3 | **Tests** | ⬜ todo | Integration tests for Chronicle/Lore/Echo flows with a temp repo. Bruno collection covers manual flows (`07-git/`). |

---

## A3 — Frontend (React)

| # | Task | Status | Acceptance criteria |
|---|------|--------|---------------------|
| A3.1 | **Bootstrap** | ✅ done | Vite + React 18 + TypeScript + Tailwind under `frontend/`; `npm run dev` / `npm run build`. |
| A3.2 | **API client** | ✅ done | `frontend/src/services/api.ts` covers auth, projects, chapters, scenes, git, wiki. Note: wiki relationship fields use `source_id`/`target_id`/`label` — must align with backend (`from_entity_id`/`to_entity_id`/`type`) before A3.6. |
| A3.3 | **Auth screens** | ⬜ in-progress | Login + Register pages exist; need to verify store wiring and redirect flows work end-to-end. |
| A3.4 | **Project list** | ⬜ in-progress | Dashboard page scaffolded; needs project list + create form wired to API. |
| A3.5 | **Scene editor** | ⬜ in-progress | Editor page + ScribeEditor component scaffolded; needs chapter/scene load and save wired to API. |
| A3.6 | **Wiki list/detail** | ⬜ todo | WikiPage + WikiPanel components exist; fix relationship field mismatch, then wire to A1 API. |
| A3.7 | **Git panel (minimal)** | ⬜ in-progress | GitPanel component scaffolded; needs Lore endpoint wired to show last N chronicles. |

---

## A4 — Quality bar

| # | Task | Status | Acceptance criteria |
|---|------|--------|---------------------|
| A4.1 | **CI** | ⬜ todo | GitHub Actions workflow: `go test ./...` + `npm run build`. |
| A4.2 | **CLAUDE.md / ROADMAP** | ⬜ todo | `frontend/` in repo layout; Phase A checkboxes updated as items ship. |

---

## Suggested order for remaining work

1. **A0.1 + A0.3** — README with local dev instructions and infra note
2. **A3.3 + A3.4** — verify auth + dashboard work end-to-end against running API
3. **A3.5** — scene editor save/load
4. **A0.2** — OpenAPI after screens are stable (reduces field-name churn)
5. **A3.6** — wiki UI (fix relationship field mismatch first)
6. **A3.7** — git panel
7. **A2.3** — git integration tests
8. **A4.1** — CI

---

## Out of scope for Phase A

- Real-time collaboration (WebSocket / CRDT)
- AI routes beyond stubs
- EPUB/Scrivener export jobs
- Map builder, image generation, novel guide wizard
- Production Helm/K8s

---

*Linked from [ROADMAP.md](../../ROADMAP.md) and [PROJECT_PLAN.md](../PROJECT_PLAN.md).*
