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
| A0.1 | **README** at repo root | ✅ done | Prerequisites (Go, Node, Docker), clone → `make dev` → copy `.env.example` → `make run`, link to `backend/.env.example`, smoke: `GET /healthz`. |
| A0.2 | **OpenAPI spec** (`docs/openapi.yaml`) | ✅ done | All 40 routes documented (auth, projects, chapters, scenes, git, wiki). `npm run gen:api` generates `frontend/src/services/api-types.ts`; CI diff-checks generated file against spec. |
| A0.3 | **Infra honesty** | ✅ done | "Redis/MinIO provisioned but not yet consumed — activated in Phase B" note added to README. |
| A0.4 | **Full-stack compose** | ✅ done | `infra/docker/docker-compose.yml` runs postgres + redis + minio + api + frontend; `Dockerfile.frontend` multi-stage build; nginx proxies `/api/` to backend and handles SPA routing. |

**Remaining in A0:** A0.1 and A0.3 (both live in the README; write them together).

---

## A1 — Backend: Wiki v1

| # | Task | Status | Acceptance criteria |
|---|------|--------|---------------------|
| A1.1 | **Schema** | ✅ done | Migration 005: `wiki_entities`, `wiki_relationships`, `wiki_magic_rules`, `wiki_timeline_events` with indexes. |
| A1.2 | **sqlc** | ✅ done | Queries in `pkg/db/queries/wiki.sql`; `make sqlc` clean; generated to `sqlcgen/wiki.sql.go`. |
| A1.3 | **Service + handler** | ✅ done | Full CRUD for entities (with hierarchy), relationships, magic rules, timeline events; autolink; graph endpoint. |
| A1.4 | **Routes** | ✅ done | `/api/v1/projects/:id/wiki/...` registered in `cmd/api/main.go` behind `auth.RequireAuth`. |
| A1.5 | **Tests** | ✅ done | Integration tests for entity CRUD, child entities, relationships, graph, magic rules, timeline events, autolink, unauthenticated. |
| A1.6 | **Timeline date update** | ✅ done | `PATCH /timeline/:tid` accepts `year`/`month`/`day` fields; era-only events supported. |
| A1.7 | **Timeline relative anchoring** | ✅ done | `anchor_event_id` + `anchor_offset_year/month/day`; migration 006; DFS resolution with cycle detection; era inheritance; mutual exclusion validation. Unit tests in `timeline_test.go`. |

---

## A2 — Backend: Git visibility

| # | Task | Status | Acceptance criteria |
|---|------|--------|---------------------|
| A2.1 | **API** | ✅ done | `GET /git/status`, `POST /git/chronicle`, `GET /git/lore`, `GET /git/echo`, `GET /git/timelines`, `POST /git/timelines` (diverge), `POST /git/timelines/:name/travel`, `POST /git/timelines/:name/canonize`. |
| A2.2 | **Auth** | ✅ done | All git routes behind `RequireAuth`; owner-only via project lookup. |
| A2.3 | **Tests** | ✅ done | 21 integration tests in `git_handler_test.go` covering full flow: status, chronicle (creates commit, nothing-to-chronicle, validation), lore + pagination, echo, timelines, diverge, travel, canonize. `testutil.SetupRouterWithGit` wires real `GitService` against `t.TempDir()`. Also fixed go-git `ErrEmptyCommit` reliability bug (now uses `wt.Status()` check). |

---

## A3 — Frontend (React)

| # | Task | Status | Acceptance criteria |
|---|------|--------|---------------------|
| A3.1 | **Bootstrap** | ✅ done | Vite + React 18 + TypeScript + Tailwind under `frontend/`; `npm run dev` / `npm run build`. |
| A3.2 | **API client** | ✅ done | `frontend/src/services/api.ts` covers all routes; types sourced from generated `api-types.ts`. Relationship fields corrected to match backend (`from_entity_id`/`to_entity_id`/`type`). Added `getGraph`, `listMagicRules`, `createMagicRule`, `updateMagicRule`, `deleteMagicRule`. |
| A3.3 | **Auth screens** | ✅ done | Login + Register pages fully wired to `authStore`; redirect flows verified. |
| A3.4 | **Project list** | ✅ done | Dashboard page: project list + create modal fully wired to API. |
| A3.5 | **Scene editor** | ✅ done | Editor page: project + chapter + scene load; 1500ms debounce autosave; inline chapter/scene creation via `ProjectExplorer`. |
| A3.6 | **Wiki hub** | ✅ done | `WikiPanel` (side panel, fully wired for entity CRUD) + `WikiHub` full-page (`/projects/:id/wiki`) with Entities card-grid tab and Timeline tab (`TimelineView` component with create/edit/delete). External-link icon in WikiPanel navigates to hub. |
| A3.7 | **Git panel** | ✅ done | `GitPanel` fully wired: status, timelines list (travel/canonize), lore (paginated history), Chronicle modal, Diverge modal. Fixed pre-existing type bugs: `Timeline` → `TimelineInfo`, `e.timestamp` → `e.created_at`, `status.dirty` removed (not in backend response), `tl.last_chronicle` → `tl.head_sha`. |

---

## A4 — Quality bar

| # | Task | Status | Acceptance criteria |
|---|------|--------|---------------------|
| A4.1 | **CI — Go tests** | ✅ done | `go test -p 1 ./...` in GitHub Actions (self-hosted runner). |
| A4.2 | **CI — Frontend typecheck** | ✅ done | `npx tsc --noEmit` added to `dev.yml` CI pipeline. |
| A4.3 | **CI — API types drift check** | ✅ done | `npm run gen:api && git diff --exit-code src/services/api-types.ts` fails build if spec and generated types drift. |
| A4.4 | **CI — `sqlc diff` check** | ✅ done | Step added to `dev.yml`: `go install sqlc && sqlc generate && git diff --exit-code pkg/db/sqlcgen/`. |
| A4.5 | **CI — ESLint** | ✅ done | `eslint.config.js` (flat config, `@typescript-eslint` + react-hooks); `npm run lint` added to CI; 0 errors. |
| A4.6 | **CLAUDE.md / ROADMAP update** | ✅ done | CLAUDE.md repo layout updated; all ROADMAP.md Phase A checkboxes ticked; current-state table refreshed. |

---

## Remaining work to close Phase A

Three small tasks separate the current state from Phase A done:

### 1 — README (A0.1 + A0.3) — ~1–2 hours

Write `README.md` at the repo root:

```
## Prerequisites
Go 1.23, Node 20+, Docker + Compose

## Quick start (local dev)
git clone ...
cp backend/.env.example backend/.env   # fill in values
make dev          # starts postgres, redis, minio
make run          # starts Go API on :8080
cd frontend && npm install && npm run dev   # React on :5173

## Smoke test
curl http://localhost:8080/api/v1/healthz

## Note on Redis / MinIO
Both are provisioned by `make dev` but not yet consumed by the API.
Redis will be used for collaboration pub/sub and rate limiting in Phase B.
MinIO will store exports and wiki image uploads in Phase B.
```

Also add architecture diagram link, Bruno collection note, and CI badge.

### 2 — CI hardening (A4.4 + A4.5) — ~2–3 hours

**`sqlc diff`** step in `dev.yml`:
```yaml
- name: Check sqlc output is up to date
  working-directory: backend
  run: |
    go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
    sqlc generate
    git diff --exit-code pkg/db/sqlcgen/
```

**ESLint** setup:
```bash
npm install --save-dev eslint @typescript-eslint/eslint-plugin \
  @typescript-eslint/parser eslint-plugin-react-hooks
```
Add `eslint.config.js` (flat config), then:
```yaml
- name: Lint frontend
  working-directory: frontend
  run: npm run lint
```

### 3 — CLAUDE.md / ROADMAP update (A4.6) — ~30 minutes

- Update `CLAUDE.md` repo layout table to include `frontend/` and `docs/openapi.yaml`
- Check off Phase A exit criteria in any linked ROADMAP.md

---

## Suggested order for the remaining A0/A4 work

1. **README** (A0.1 + A0.3) — unblocks any new contributor
2. **ESLint config** (A4.5) — configure locally first, verify it passes, then add CI step
3. **sqlc diff** (A4.4) — one CI step, no local config needed
4. **CLAUDE.md / ROADMAP** (A4.6) — tidy up; declare Phase A shipped

---

## Out of scope for Phase A

- Real-time collaboration (WebSocket / CRDT)
- AI routes beyond stubs
- EPUB/Scrivener export jobs
- Map builder, image generation, novel guide wizard
- Production Helm/K8s

---

*Linked from [ROADMAP.md](../../ROADMAP.md) and [PROJECT_PLAN.md](../PROJECT_PLAN.md).*
