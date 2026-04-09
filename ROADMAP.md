# NexusTale roadmap

Sci-fi/fantasy novel-writing tool: structured manuscripts (projects → chapters → scenes), worldbuilding, AI-assisted drafting, export, and (eventually) collaboration.

**Companion docs:** [CLAUDE.md](./CLAUDE.md) (how to work in this repo), [docs/PROJECT_PLAN.md](./docs/PROJECT_PLAN.md) (full architecture + phases), [docs/specs/phase-a-mvp.md](./docs/specs/phase-a-mvp.md) (Phase A checklist), [Makefile](./Makefile) (dev commands).

---

## Current state (snapshot)

| Area | Status |
|------|--------|
| **API shell** | Go 1.23 + Gin; `/healthz`; `/api/v1/auth/*`; `/api/v1/projects/*` (CRUD + chapters + scenes), JWT + refresh tokens |
| **Database** | PostgreSQL migrations (006) + **sqlc** (`pkg/db/queries` → `pkg/db/sqlcgen`) |
| **Git per project** | Non-bare repos on disk; full Chronicle/Lore/Echo/Diverge/TravelTo/Canonize API; 21 handler integration tests; fast-forward merge; Paradox detection |
| **Wiki v1** | `wiki_entities`, `wiki_relationships`, `wiki_magic_rules`, `wiki_timeline_events` — full CRUD + timeline anchoring; all with integration tests; autolink + graph endpoints |
| **Redis / MinIO** | Provisioned in dev compose; **not yet consumed** by API (Phase B) |
| **Collaboration, AI, export** | Packages stubbed; no HTTP registration |
| **Frontend** | React 18 + Vite + TypeScript + Tailwind; auth, project list, VSCode-style scene editor (debounce save), wiki hub (entities + timeline CRUD), git panel (chronicle/lore/echo/diverge/canonize) |
| **OpenAPI + types** | `docs/openapi.yaml` (40 routes); `frontend/src/services/api-types.ts` generated; CI drift check |
| **CI/CD** | GitHub Actions (self-hosted) → GHCR → Ansible → dev VM; Go tests, tsc, ESLint, API-types drift, sqlc diff, Docker build + push, Ansible deploy |
| **Bruno collection** | Full integration tests for auth, health, projects, chapters, scenes, wiki (incl. anchor tests), git |
| **README** | Written — prerequisites, quick start, env vars, Redis/MinIO note |
| **K8s / Helm** | Stubs — not yet used |

---

## Core features (product pillars)

1. **Accounts & access** — Register/login, JWT access + refresh, roles. *Done.*
2. **Manuscript structure** — Projects, chapters, scenes, ordering, summaries, tags; Git-backed. *API done; Git history stubs.*
3. **World wiki** — Entities (character/location/faction/item/concept/lore), relationships graph, timeline, magic rules, autolink. *API + Bruno tests done; no frontend yet.*
4. **AI-assisted writing** — Completion, chat, summarize, adapters, RAG. *Scaffold only.*
5. **Export** — Markdown, EPUB, Scrivener. *Scaffold only.*
6. **Collaboration** — Real-time CRDT/WebSocket, Redis pub/sub. *Scaffold only.*
7. **Assets** — Covers and binaries via MinIO/S3. *Package stub; not integrated.*
8. **Writer UI** — React app: editor, wiki, AI panels, export. *Not started.*

---

## Phase A — Product skeleton

### A0 — Documentation & contracts
- [x] **A0.1** README — prerequisites, `make dev` + `make run`, env vars, smoke test
- [x] **A0.2** OpenAPI spec (`docs/openapi.yaml`) — all 40 routes; TypeScript codegen via `npm run gen:api`
- [x] **A0.3** Infra honesty — Redis/MinIO "provisioned but not yet consumed" note in README

### A1 — Backend: Wiki v1
- [x] **A1.1** Schema — `wiki_entities` (with `parent_entity_id`), `wiki_relationships`, `wiki_magic_rules`, `wiki_timeline_events`
- [x] **A1.2** sqlc — queries generated; `make sqlc` clean
- [x] **A1.3** Service + handler — entity CRUD, children, relationships, graph, magic rules, timeline, autolink
- [x] **A1.4** Routes — registered in `cmd/api/main.go` behind `RequireAuth`
- [x] **A1.5** Tests — integration tests for wiki happy path + Bruno collection `06-wiki/`
- [x] **A1.6** Timeline anchoring — `anchor_event_id` + offset fields; DFS cycle detection; migration 006

### A2 — Backend: Git / Chronicle
- [x] **A2.1** `git.go` — non-bare repos; `InitRepo` creates `canon` branch with initial commit
- [x] **A2.2** Chronicle (commit), Lore (history), Echo (diff), Timelines (list branches)
- [x] **A2.3** Diverge (branch + checkout), TravelTo (switch branch), Canonize (fast-forward merge + Paradox detection)
- [x] **A2.4** HTTP routes — `/:id/git/status|chronicle|lore|echo|timelines` behind `RequireAuth`
- [x] **A2.5** Handler integration tests — 21 tests in `git_handler_test.go`; `SetupRouterWithGit` test helper

### A3 — Frontend (React)
- [x] **A3.1** Bootstrap — Vite + React + TypeScript + Tailwind under `frontend/`
- [x] **A3.2** API client — typed fetch wrapper sourced from generated `api-types.ts`
- [x] **A3.3** Auth screens — register + login with NexusTale branding
- [x] **A3.4** Project list — list, create, navigate
- [x] **A3.5** Scene editor — load/save scene content with 1500ms debounce autosave
- [x] **A3.6** Wiki hub — full-page `/projects/:id/wiki` with Entities tab + Timeline tab (CRUD)
- [x] **A3.7** Git panel — Chronicle, Lore (paginated), Echo, Timelines, Diverge, TravelTo, Canonize

### A4 — Quality bar
- [x] **A4.1** CI — `go test -p 1 ./...` on push to dev
- [x] **A4.2** CI — frontend `npx tsc --noEmit` typecheck
- [x] **A4.3** CI — API types drift check (`npm run gen:api && git diff --exit-code`)
- [x] **A4.4** CI — `sqlc diff` check (regenerate + `git diff --exit-code pkg/db/sqlcgen/`)
- [x] **A4.5** CI — ESLint (`@typescript-eslint` + react-hooks plugin; `npm run lint`)
- [x] **A4.6** CLAUDE.md repo layout updated; ROADMAP checkboxes complete

---

## Phase A+ — Pre-Phase B foundations

**Actionable checklist:** [specs/phase-aplus.md](./docs/specs/phase-aplus.md)

### A+1 — Word count + scene metadata in editor
- [x] **A+1.1** Migration 007 — `word_count INTEGER NOT NULL DEFAULT 0` on `scenes`
- [x] **A+1.2** Backend — word count computed server-side on content save; `tags` patchable; exposed in `SceneResponse`
- [x] **A+1.3** Frontend — `SceneMetadataPanel` collapsible drawer: POV, tense, tags, summary with debounced PATCH
- [x] **A+1.4** Frontend — word count and POV shown in collapsed panel header; word count from server on scene load

### A+2 — Secure AI key storage
- [x] **A+2.1** Migration 008 — `user_api_keys` table (`provider`, `encrypted_key` BYTEA, `key_hint`, unique per user+provider)
- [x] **A+2.2** Backend — AES-256-GCM encryption; `NEXUSTALE_ENCRYPTION_KEY` in config; `POST/GET/DELETE /users/me/api-keys`; internal `DecryptAPIKey` for AI adapters
- [x] **A+2.3** OpenAPI — new endpoints + `APIKeyResponse`/`UpsertAPIKeyRequest` schemas; `api-types.ts` regenerated
- [x] **A+2.4** Frontend — `/settings` page: stored key hints list, add/replace form (password input, never raw key shown), remove button; linked from Dashboard

### A+3 — Autolink wired in editor
- [x] **A+3.1** Frontend — `api.wiki.autolink` call debounced (1.2s) on scene content change in `WikiPanel`
- [x] **A+3.2** Frontend — matched entity names shown as clickable cyan badges ("In this scene" section); clicking opens entity detail

### A+4 — Focus / distraction-free mode
- [ ] **A+4.1** Frontend — keyboard shortcut (e.g. `F11` or toolbar button) toggles focus mode: hides ActivityBar, left panel, ProjectExplorer, TopBar, StatusBar
- [ ] **A+4.2** Frontend — full-width `ScribeEditor` with minimal chrome; `Esc` or button exits focus mode

### A+5 — Project home / stats page
- [ ] **A+5.1** Backend — `GET /projects/:id/stats` endpoint: total word count, scene count, chapter count, last updated; aggregate via SQL
- [ ] **A+5.2** OpenAPI — document `ProjectStats` schema; regenerate types
- [ ] **A+5.3** Frontend — `/projects/:id` becomes a project home page (before entering editor); shows stats, recent scenes, quick-open links, wiki snapshot

### A+6 — User account deletion
- [ ] **A+6.1** Backend — `DELETE /users/me` endpoint; cascades via FK to all owned projects, chapters, scenes, wiki, git repos on disk, API keys, refresh tokens
- [ ] **A+6.2** OpenAPI — document endpoint; regenerate types
- [ ] **A+6.3** Frontend — "Danger zone" section in `/settings`; confirm dialog requiring the user to type their email before deletion; redirect to `/login` on success

### A+7 — Light theme
- [ ] **A+7.1** Frontend — define light-mode CSS variable overrides in `tailwind.config.js` / `index.css`; `prefers-color-scheme` media query fallback
- [ ] **A+7.2** Frontend — theme toggle button in `/settings` (stored in `localStorage`); apply `dark` / `light` class to `<html>`

### A+8 — Relationship graph visualization
- [ ] **A+8.1** Frontend — install `d3` (or `@visx/network`); `RelationshipGraph` component consuming existing `GET /wiki/graph` endpoint
- [ ] **A+8.2** Frontend — force-directed layout; nodes colored by entity type; edges labeled with relationship type; click node → entity detail
- [ ] **A+8.3** Frontend — embed in `WikiHub` as a third "Graph" tab alongside Entities and Timeline

**Deferred to Phase B+ (AI or infra dependent):**

- Plot hole detection and narrative consistency checks (needs AI routes)
- Async worker / job queue (needed for heavy export jobs)
- Vector memory / RAG / embeddings — pgvector + chapter summary anchors
- Admin dashboard — system health, queue status, log access (Phase C+)

---

## Phase B — AI + export core

- AI proxy: OpenAI / Anthropic / Ollama adapters wired to routes; scene continuation, summarize, chat
- AI memory: chapter summaries as context anchors; sliding window; manual pinning
- Token usage tracking and cost estimation per project
- Export: Markdown zip (sync) + EPUB (async job + MinIO download)
- Novel guide: step wizard backend + happy-path UI

## Phase C — Collaboration + depth

- WebSocket + CRDT for scene editing; roles and project invites
- Relationship graph visualization; plot arc view
- DOCX export; wiki image upload
- Per-project model selection; fallback model config

## Phase D — Premium / advanced

- Map builder; image generation pipelines for wiki entities
- Scrivener / Fountain export; advanced Git branching UX
- Series-level continuity management
- Multi-region, scale-out collaboration tuning

---

## How to use this file

Treat unchecked items as **Claude Code / issue seeds**: one checkbox → one focused task with acceptance criteria. For deep design, add `docs/specs/<topic>.md` and link from a roadmap line.

*Last updated: A+1–A+3 complete; A+4–A+8 laid out and ready to implement.*
