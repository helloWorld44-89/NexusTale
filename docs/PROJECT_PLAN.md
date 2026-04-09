# NexusTale — end-to-end project plan

A single reference for **backend (Go + Gin)**, **frontend (React)**, and **feature domains**. Aligns with the existing API (`auth`, `projects`, chapters/scenes, Git-backed repos) and grows from there. See also [ROADMAP.md](../ROADMAP.md) and [CLAUDE.md](../CLAUDE.md).

---

## 1. Vision

NexusTale is a **novel-writing platform** that combines:

- Structured manuscript tooling (outline → chapters → scenes)
- **Git-backed** history and branching for narrative experiments
- **Multi-user** collaboration with clear roles
- A **world wiki** (entities, magic, timeline, plot) wired to the manuscript
- **AI** (local models + cloud APIs) for drafting, consistency, and research-style assistance
- **Exports** to common writer workflows (Markdown, Word, Scrivener, EPUB, Final Draft–class structures where feasible)
- **Rich worldbuilding**: reference images, optional **map builder**, **image generation** for wiki entries
- An **interactive, step-by-step guide** (“novel builder”) that teaches craft while driving the user through setup → world → plot → draft → revise

---

## 2. High-level architecture

```mermaid
flowchart TB
  subgraph client [React SPA]
    UI[App shell + routes]
    Editor[Manuscript + CRDT client]
    WikiUI[Wiki + maps + images]
    Guide[Novel guide wizard]
    AIChat[AI panels]
  end

  subgraph api [Go API - Gin]
    HTTP[REST + WebSocket upgrade]
    Auth[JWT + refresh + RBAC]
    Projects[Projects / chapters / scenes]
    GitSvc[Git service layer]
    WikiAPI[Wiki graph API]
    Collab[Collaboration hub]
    AIProxy[AI proxy + adapters]
    ExportJobs[Export workers / async]
    Media[Uploads + presigned URLs]
  end

  subgraph data [Data plane]
    PG[(PostgreSQL + sqlc)]
    Redis[(Redis cache + pub/sub)]
    S3[(MinIO / S3)]
    GitDisk[(Bare repos on disk)]
  end

  UI --> HTTP
  Editor --> Collab
  WikiUI --> HTTP
  Guide --> HTTP
  AIChat --> AIProxy

  HTTP --> Auth
  HTTP --> Projects
  HTTP --> WikiAPI
  HTTP --> ExportJobs
  HTTP --> Media

  Projects --> PG
  Projects --> GitSvc
  GitSvc --> GitDisk
  WikiAPI --> PG
  Collab --> Redis
  Collab --> PG
  AIProxy --> PG
  ExportJobs --> PG
  ExportJobs --> S3
  Media --> S3
  Auth --> PG
```

**Principles**

- **PostgreSQL** is the source of truth for metadata, wiki graph, permissions, guide progress, and AI usage accounting.
- **Git** stores narrative content versions per project (or per branch); DB holds pointers, refs, and merge metadata.
- **Redis** backs sessions (optional), rate limits, pub/sub for multi-pod collaboration, and short-lived AI job status.
- **Object storage** holds exports, generated images, map assets, and large binaries.

---

## 3. Backend infrastructure (Go + Gin)

### 3.1 Service layout (packages)

| Layer | Responsibility |
|-------|----------------|
| `cmd/api` | Process entry: config, DB pool, migrations, router, graceful shutdown |
| `internal/config` | Viper/env; validate required secrets in production |
| `internal/auth` | Register/login, JWT access + refresh, middleware; later OAuth optional |
| `internal/project` | Projects, chapters, scenes; orchestrates Git commits on meaningful saves |
| `internal/wiki` | Entities, types (character, location, faction, magic…), relationships, timeline events, plot beats, attachments |
| `internal/collaboration` | WebSocket hub, rooms per project/doc, CRDT/op sync; Redis fan-out |
| `internal/ai` | Provider adapters (Ollama, OpenAI, Anthropic, OpenRouter…), prompt templates, RAG/embeddings, quotas |
| `internal/export` | Pipelines to Markdown, DOCX, EPUB, Scrivener-compatible zip, Fountain, etc. |
| `internal/guide` | Novel-builder steps, user progress, unlock rules, links to created artifacts |
| `internal/media` | Presigned uploads, image job callbacks, map layer storage |
| `pkg/db` | Pool, migrations, **sqlc** queries only |
| `pkg/cache` | Interface: in-memory dev / Redis prod |
| `pkg/storage` | S3/MinIO client |

### 3.2 API surface (conceptual)

- **REST** `/api/v1/...` for CRUD, exports, guide state, wiki graph.
- **WebSocket** `/api/v1/projects/:id/collab` (or per-document) for real-time editing.
- **Optional SSE** for AI streaming tokens if you want chat without WS complexity.

### 3.3 Git versioning (backend behavior)

- Each **project** maps to a **bare repo** (already aligned with `git_repo_path`).
- **Commits** on explicit checkpoints: scene save, “snapshot,” branch create, merge.
- **Branches** for alternate plotlines or A/B drafts; API: list/create/merge/delete branch.
- DB tables: `git_ref` or store branch head SHA + metadata; tie scenes to blob IDs or paths in repo (`content/scenes/{id}.md`).
- **Conflict policy**: last-write-wins for simple MVP; CRDT/OT for collaboration reduces merge pain.

### 3.4 Collaboration (backend)

- **Roles**: owner, editor, commenter, viewer (extend `user_role` / `project_collaborators`).
- **Presence**: who’s online, which scene focused (Redis TTL keys).
- **Operations**: CRDT (e.g. Yjs-compatible binary over WS) or operational transform; persist periodically to DB + Git snapshot.
- **Multi-instance**: Redis pub/sub between Gin pods (as sketched in `internal/collaboration`).

### 3.5 AI integration (backend)

- **Unified internal API**: `Complete`, `Chat`, `Summarize`, `Embeddings`, `Image` (delegate to image provider or local).
- **Adapters**: interface per provider; config selects default + per-project overrides.
- **Local**: Ollama (and optional llama.cpp server) via HTTP.
- **Cloud**: OpenAI, Anthropic, OpenRouter, etc.; API keys server-side only.
- **Safety**: rate limits, token budgets, audit log of requests (hashed prompts optional), PII warnings in guide copy.
- **RAG**: pgvector (or separate table) for wiki + scene chunk embeddings; query at prompt-build time.

### 3.6 Exports (backend)

- **Synchronous** for small (single Markdown file).
- **Async job** for large (full project EPUB, Scrivener zip): job ID, poll or webhook, file in MinIO, time-limited download URL.
- **Mappers**: internal canonical model → target format (one module per format).

### 3.7 Media & image generation

- **Wiki images**: upload reference art; optional “generate from prompt” via configured image API (cloud) or local diffusion HTTP service.
- **Map builder**: store JSON (layers, pins linked to wiki entities) + rendered PNG/WebP preview in S3; version alongside project.

### 3.8 Novel guide (backend)

- **Guide definition**: versioned JSON or DB rows (steps, copy, prerequisites, linked templates).
- **User progress**: `guide_progress` per user per project (current step, completed flags, skipped optional steps).
- **Side effects**: guide actions call existing APIs (create wiki template entities, seed plot outline, open first scene).

### 3.9 Cross-cutting

- **Observability**: structured logs (`slog`), metrics, tracing on hot paths (AI, export, WS).
- **Testing**: table-driven unit tests; integration tests with testcontainers (Postgres, Redis) for auth and one collab path.

---

## 4. Frontend infrastructure (React)

### 4.1 Recommended stack (adjust to taste)

| Choice | Role |
|--------|------|
| **Vite + React + TypeScript** | Fast dev, simple deployment |
| **React Router** | SPA routing |
| **TanStack Query** | Server state, cache, mutations |
| **Zustand or Redux Toolkit** | UI-local state (editor, presence) |
| **TipTap or Lexical or CodeMirror 6** | Rich text for scenes; pick one and standardize |
| **Yjs + y-websocket or custom sync** | If CRDT matches backend protocol |
| **React Hook Form + Zod** | Forms (wiki, settings) |
| **Tailwind or CSS modules** | Styling; align with design system early |

### 4.2 App structure (folders)

```
frontend/
  src/
    app/           # providers, router, layout
    features/
      auth/
      dashboard/
      project/     # outline, manuscript, Git UI
      wiki/        # graph, entity detail, timeline, magic
      collab/      # presence, cursors (if CRDT)
      ai/          # chat, inline assist, model picker
      export/
      maps/        # map builder canvas
      guide/       # step wizard, progress
    api/           # generated or hand-written client
    components/    # shared UI
    lib/           # utils, tokens, websocket helpers
```

### 4.3 Key screens (UX map)

1. **Auth** — login/register, forgot password (later).
2. **Project list** — create, archive, collaborators.
3. **Project home** — novel guide CTA, outline, recent scenes, wiki shortcuts.
4. **Scene editor** — full screen, AI sidebar, save / snapshot / branch controls.
5. **Wiki** — entity list, filters by type, graph view, timeline, magic codex, plot summary page.
6. **Maps** — layer list, entity pins, export image.
7. **Exports** — format picker, job status, download.
8. **Settings** — AI providers (which cloud keys server uses is admin; user picks model prefs), theme, guide reset.

### 4.4 Frontend ↔ backend contracts

- OpenAPI or **openapi-typescript** codegen from a maintained `openapi.yaml` (generate when API stabilizes).
- **Auth**: store access token in memory + refresh in httpOnly cookie (preferred) or secure storage pattern you choose; align with existing JWT handlers.

---

## 5. Feature domains (detailed outline)

### 5.1 Git versioning

- **User stories**: snapshot before risky edit; branch “what-if”; compare diff; merge branch to main storyline.
- **Backend**: Git operations service; protect against repo corruption; async for heavy diffs.
- **Frontend**: branch picker, timeline of commits, diff viewer (text).
- **Dependencies**: stable on-disk layout, backups, path traversal hardening.

### 5.2 Co-author collaboration

- **User stories**: two editors same scene; comments; suggest mode (later).
- **Backend**: WS hub + Redis; persistence debounce; optional CRDT merge to Git.
- **Frontend**: presence avatars, CRDT provider, offline queue (stretch).
- **Dependencies**: role checks on every op; conflict UX copy.

### 5.3 Full wiki

| Sub-area | Content | Notes |
|----------|---------|--------|
| **World** | Settings, regions, cultures, tech level | Link scenes ↔ locations |
| **Characters** | Bios, arcs, relationships | Relationship graph edges |
| **Magic / systems** | Rules, costs, limits | Consistency checks via AI optional |
| **Timeline** | Dated events, eras | Sort + filter; link entities |
| **Plot** | Acts, beats, summaries | Connect to chapters/scenes (many-to-many) |

- **Data model**: generic `entities` + `entity_type` + JSON attributes vs normalized tables; start generic for speed, normalize hot paths later.
- **Autolink**: scan scene text for `@Entity` or wiki links; backend index optional.

### 5.4 AI integration (local + cloud)

- **Modes**: inline completion, chat, “lint” voice, summarize scene, generate alternate lines.
- **Model registry**: list models from Ollama + configured cloud; capability flags (vision, long context).
- **Cost control**: per-project daily caps; show estimated cost only for cloud.
- **Privacy**: local-first messaging in UI when using Ollama; data handling note in guide.

### 5.5 Export options (major platforms)

| Target | Purpose |
|--------|---------|
| **Markdown** | Universal, Git-friendly |
| **DOCX** | Word / editors |
| **EPUB** | e-readers |
| **Scrivener** | `.scriv` zip structure (document compatibility expectations) |
| **Fountain** | Screenplay-adjacent workflows |
| **PDF** | Sharing (optional, via renderer) |
| **Plain JSON** | Backup / migrations |

- Prioritize **Markdown + EPUB + DOCX** for MVP; add Scrivener when internal model is stable.

### 5.6 Image generation (wiki)

- Upload reference + optional “generate cover portrait for character X.”
- Store prompt metadata, model id, parent entity id; allow regenerate.
- Content policy: NSFW toggles, project-level disable.

### 5.7 World map builder

- **MVP**: image upload as basemap + draggable pins → wiki entities.
- **V2**: tiled map, layers (political / terrain), vector export.
- **Tech**: canvas (Konva, Pixi, or MapLibre if geo); save JSON + thumbnail to S3.

### 5.8 Step-by-step interactive novel guide

- **Structure**: linear steps with optional branches (e.g. “pantsing vs outlining”).
- **Each step**: short lesson, 1–3 actions in-app (create entity, write logline, outline act I).
- **Progress**: persistent; “resume guide” on login.
- **Content**: separate content pack (JSON/CMS) so writers can improve copy without redeploying logic.
- **Success**: user finishes with populated wiki skeleton + act outline + first scene draft.

---

## 6. Infrastructure & DevOps

### 6.1 Current state (as of 2026-04-09) ✅

- **Local dev**: `docker compose` (Postgres, Redis, MinIO) via `make dev`; API runs with `make run`.
- **CI/CD — dev branch**: GitHub Actions on a self-hosted runner (mgmt VM).
  - Push to `dev` → run `go test` → build & push API + frontend images to GHCR (`ghcr.io/helloworld44-89/nexustale/{api,frontend}:dev` and `:{sha}`).
  - Ansible playbook (`infra/ansible/deploy-dev.yml`) deploys to dev VM via `docker compose` pulling from GHCR.
  - Secrets stored as GitHub repository secrets.
- **Dev VM**: full stack running — API (port 8080), frontend/nginx (port 80), Postgres, Redis, MinIO.
- **nginx**: single `/api/` location block proxies REST + WebSocket; SPA fallback for React Router.
- **Images**: `infra/docker/Dockerfile.api` (multi-stage Go build), `infra/docker/Dockerfile.frontend` (Vite + nginx).
- **Deploy compose**: `infra/docker/docker-compose.deploy.yml` — pulls from GHCR, env vars from `.env` written by Ansible.

### 6.2 Remaining infra work

- **Environments**: `staging`, `prod` pipelines not yet built; follow same Ansible pattern.
- **Secrets**: currently GitHub repo secrets; move to Ansible Vault or a secret manager for prod.
- **CI additions**: frontend typecheck + lint, `sqlc diff` check to catch uncommitted regen.
- **K8s/Helm**: templates exist as stubs; fill when scaling beyond a single VM.
- **Ollama**: add as optional service in local compose for AI dev.

---

## 7. Phased delivery (suggested)

### Phase A — Product skeleton (MVP vertical)

**Actionable checklist:** [specs/phase-a-mvp.md](./specs/phase-a-mvp.md) (tasks **A0–A4** with acceptance criteria).

Summary: README + OpenAPI stub + infra honesty; Wiki v1 (sqlc + REST + tests); Git visibility API; React app (auth, projects, scene editor, wiki, minimal Git panel); CI/docs touch-up.

**Completed as of 2026-04-09:**
- ✅ Auth (register/login/refresh/logout), JWT middleware
- ✅ Projects, chapters, scenes — full CRUD + integration tests
- ✅ Git versioning — Chronicle/Lore/Echo/Diverge/TravelTo/Timelines/Canonize
- ✅ Wiki — entities, relationships, magic rules, timeline events (all with integration tests)
- ✅ Timeline relative anchoring — `anchor_event_id` + offset fields, DFS resolution with cycle detection (migration 006, unit tested)
- ✅ Frontend scaffold — React + Vite + TypeScript + Tailwind; auth, project list, scene editor, wiki components
- ✅ CI/CD — GitHub Actions (self-hosted runner) → GHCR → Ansible → dev VM; API + frontend deployed and reachable
- ✅ Bruno test collection — auth, projects, chapters, scenes, wiki (incl. anchor tests), git flows

**Completed (Phase A closed 2026-04-09):**
- ✅ Git handler integration tests — 21 tests covering full Chronicle/Lore/Echo/Diverge/TravelTo/Canonize flows
- ✅ Frontend wired to real API — scene editor autosave, wiki hub (entities + timeline CRUD), git panel
- ✅ OpenAPI spec (`docs/openapi.yaml`, 40 routes); TypeScript codegen (`npm run gen:api`)
- ✅ CI — frontend typecheck (`tsc --noEmit`), ESLint, API types drift check, `sqlc diff` check

### Phase B — Guide + AI + export core

- Novel guide backend + wizard UI (happy path only)
- AI proxy: one cloud + Ollama; chat + summarize
- Export: Markdown zip + EPUB (async job + download)

### Phase C — Collaboration + depth

- WebSocket + CRDT for scene editing; roles and invites
- Timeline + plot wiki views; graph visualization
- DOCX export; image upload for wiki

### Phase D — Premium / advanced

- Map builder v2; image generation pipelines
- Scrivener/Fountain; advanced Git branching UX
- Multi-region, scale-out collab tuning

---

## 8. Risks & open decisions

| Risk | Mitigation |
|------|------------|
| CRDT + Git semantics clash | Define “source of truth” windows; snapshot to Git on idle or explicit save |
| Scrivener format fragility | Document “best effort”; start with documented subset |
| AI cost spikes | Quotas, caching summaries, smaller models for lint tasks |
| Scope creep | Ship guide + wiki + editor before map builder v2 |

**Decisions to lock early**

- CRDT library and wire protocol (Yjs vs Automerge vs OT-only).
- Canonical scene format in Git (Markdown with front matter vs JSON).
- Whether plot/wiki uses graph DB later or stays in Postgres with recursive CTEs.

---

## 9. Next actions

### Phase A+ — Pre-Phase B polish

**Completed:**
- ✅ A+1 — Word count + scene metadata (`SceneMetadataPanel`, migration 007, server-side word count)
- ✅ A+2 — Secure AI key storage (migration 008, AES-256-GCM, `/users/me/api-keys`, `/settings` page)
- ✅ A+3 — Autolink wired in editor (debounced wiki entity match badges in WikiPanel)

**In progress / next up** ([full spec](./specs/phase-aplus.md)):
- ⬜ A+4 — Focus/distraction-free mode (frontend only; `F11` toggle, full-width editor)
- ⬜ A+5 — Project home/stats page (`GET /projects/:id/stats` + project overview before editor)
- ⬜ A+6 — User account deletion (`DELETE /users/me` + danger zone in settings)
- ⬜ A+7 — Light theme (CSS variable swap, Zustand theme store, toggle in settings)
- ⬜ A+8 — Relationship graph visualization (d3 force layout, WikiHub "Graph" tab)

### Phase B (next major milestone)
5. **AI integration** — wire `internal/ai` adapters to routes; Ollama for local dev, Anthropic/OpenAI for cloud; chat + scene continuation + summarize endpoints.
6. **AI memory** — chapter summaries as context anchors; sliding window; pgvector for RAG.
7. **Export** — wire `internal/export`; Markdown zip (sync) + EPUB (async job + MinIO download URL).
8. **Novel guide** — step wizard backend + happy-path UI.

### Phase C
7. **Collaboration** — WebSocket hub (`/api/v1/projects/:id/collab`), CRDT sync, presence via Redis.
8. **Timeline + plot wiki views** — frontend graph visualization, timeline browser.
9. **DOCX export + wiki image upload**.

### Infrastructure
10. **Staging/prod pipelines** — clone dev Ansible playbook; parameterize environment; add prod secrets to vault.
11. **Ollama in local compose** — optional service for AI dev without cloud keys.

This plan is meant to evolve — trim or reorder phases based on your first beta cohort’s feedback.
