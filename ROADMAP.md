# NexusTale roadmap

Sci-fi/fantasy novel-writing tool: structured manuscripts (projects → chapters → scenes), worldbuilding, AI-assisted drafting, export, and (eventually) collaboration.

**Companion docs:** [CLAUDE.md](./CLAUDE.md) (how to work in this repo), [docs/PROJECT_PLAN.md](./docs/PROJECT_PLAN.md) (full architecture + phases), [docs/specs/phase-a-mvp.md](./docs/specs/phase-a-mvp.md) (Phase A checklist), [Makefile](./Makefile) (dev commands).

---

## Current state (snapshot)

| Area | Status |
|------|--------|
| **API shell** | Go 1.23 + Gin; `/healthz`; `/api/v1/auth/*`; `/api/v1/projects/*` (CRUD + chapters + scenes), JWT + refresh tokens |
| **Database** | PostgreSQL migrations + **sqlc** (`pkg/db/queries` → `pkg/db/sqlcgen`) |
| **Git per project** | Non-bare repos on disk; full Chronicle/Lore/Echo/Diverge/TravelTo/Canonize API; fast-forward merge; Paradox detection |
| **Wiki v1** | `wiki_entities`, `wiki_relationships`, `wiki_magic_rules`, `wiki_timeline_events` — full CRUD API behind auth, mounted at `/api/v1/projects/:id/wiki/`; autolink + graph endpoints |
| **Redis / MinIO** | Env + config present; **not wired** into services yet |
| **Collaboration, AI, export** | Packages stubbed; no HTTP registration |
| **Frontend** | React + Vite + TypeScript under `frontend/`; auth (login/register), VSCode-style editor shell (TopBar, ActivityBar, ChatBar, ScribeEditor, ProjectExplorer, StatusBar) |
| **Bruno collection** | Full integration tests for auth, health, projects, chapters, scenes, wiki, git; `environments/local.bru` |
| **README** | Empty — onboarding gap |
| **K8s / Helm** | Files largely empty — deployment gap |

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
- [ ] **A0.1** README — prerequisites, `make dev` + `make run`, env vars, smoke test
- [ ] **A0.2** OpenAPI stub for auth + project + wiki routes
- [ ] **A0.3** Infra honesty — document Redis/MinIO as "optional until feature X" in README

### A1 — Backend: Wiki v1
- [x] **A1.1** Schema — `wiki_entities` (with `parent_entity_id`), `wiki_relationships`, `wiki_magic_rules`, `wiki_timeline_events`
- [x] **A1.2** sqlc — queries generated; `make sqlc` clean
- [x] **A1.3** Service + handler — entity CRUD, children, relationships, graph, magic rules, timeline, autolink
- [x] **A1.4** Routes — registered in `cmd/api/main.go` behind `RequireAuth`
- [x] **A1.5** Tests — integration tests for wiki happy path + Bruno collection `06-wiki/`

### A2 — Backend: Git / Chronicle
- [x] **A2.1** `git.go` — non-bare repos; `InitRepo` creates `canon` branch with initial commit
- [x] **A2.2** Chronicle (commit), Lore (history), Echo (diff), Timelines (list branches)
- [x] **A2.3** Diverge (branch + checkout), TravelTo (switch branch), Canonize (fast-forward merge + Paradox detection)
- [x] **A2.4** HTTP routes — `/:id/git/status|chronicle|lore|echo|timelines` behind `RequireAuth`
- [x] **A2.5** Bruno collection `07-git/` — 14 tests covering happy path + error cases

### A3 — Frontend (React)
- [x] **A3.1** Bootstrap — Vite + React + TypeScript under `frontend/`
- [x] **A3.2** API client — fetch wrapper, Bearer attach, refresh flow (`services/api.ts`)
- [x] **A3.3** Auth screens — register + login with NexusTale branding
- [x] **A3.4** Project list — list, create, navigate
- [x] **A3.5** Scene editor — load/save scene content (wire mock editor to real API)
- [x] **A3.6-shell** VSCode-style editor shell — TopBar, ActivityBar, ChatBar (mock), ScribeEditor, ProjectExplorer, StatusBar
- [x] **A3.7** Git panel — ChronicleModal, TimelineDrawer, Echo view (Chronicle + Diverge + Canonize UI)
- [x] **A3.8** Wiki list/detail — list entities, create/edit

### A4 — Quality bar
- [ ] **A4.1** CI — `go test ./...` + frontend `npm run build`
- [ ] **A4.2** README — prerequisites, `make dev` + `make run`, env vars, smoke test

---

## Phase B — Guide + AI + export core (after Phase A)

- Novel guide backend + wizard UI (happy path only)
- AI proxy: one cloud + Ollama; chat + summarize
- Export: Markdown zip + EPUB (async job + download)

## Phase C — Collaboration + depth (after Phase B)

- WebSocket + CRDT for scene editing; roles and invites
- Timeline + plot wiki views; graph visualization
- DOCX export; image upload for wiki entries

## Phase D — Premium / advanced

- Map builder v2; image generation pipelines
- Scrivener/Fountain; advanced Git branching UX
- Multi-region, scale-out collab tuning

---

## How to use this file

Treat unchecked items as **Claude Code / issue seeds**: one checkbox → one focused task with acceptance criteria. For deep design, add `docs/specs/<topic>.md` and link from a roadmap line.

*Last updated: A3 complete — project list, scene editor, git panel (Chronicle/Timelines/Diverge/Canonize), wiki panel (entity list/detail/create/edit/delete).*
