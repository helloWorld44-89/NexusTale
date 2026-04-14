# NexusTale roadmap

Sci-fi/fantasy novel-writing tool: structured manuscripts (projects → chapters → scenes), worldbuilding, AI-assisted drafting, export, and (eventually) collaboration.

**Companion docs:** [CLAUDE.md](./CLAUDE.md) (how to work in this repo), [docs/PROJECT_PLAN.md](./docs/PROJECT_PLAN.md) (full architecture + phases), [docs/specs/phase-a-mvp.md](./docs/specs/phase-a-mvp.md) (Phase A checklist), [Makefile](./Makefile) (dev commands).

---

## Current state (snapshot)

| Area | Status |
|------|--------|
| **API shell** | Go 1.25 + Gin; `/healthz`; `/api/v1/auth/*`; `/api/v1/projects/*` (CRUD + acts + chapters + scenes), JWT + refresh tokens |
| **Database** | PostgreSQL migrations **(016)** + **sqlc** (`pkg/db/queries` → `pkg/db/sqlcgen`) |
| **Manuscript hierarchy** | **Project → Act → Chapter → Scene**; act layer hidden in UI for single default act; full CRUD + integration tests + Bruno |
| **Git per project** | Non-bare repos on disk; full Chronicle/Lore/Echo/Diverge/TravelTo/Canonize API; 21 handler integration tests; fast-forward merge; Paradox detection |
| **Wiki v1** | `wiki_entities`, `wiki_relationships`, `wiki_magic_rules`, `wiki_timeline_events` — full CRUD + timeline anchoring; all with integration tests; autolink + graph endpoints; relationship graph (d3 force) |
| **Redis / MinIO** | Provisioned in dev compose; MinIO used for EPUB export (async job → presigned URL) |
| **AI proxy** | `internal/ai`: Anthropic, OpenAI, Ollama adapters; beat + continue modes; chapter summaries + AI Bible in every call; `POST /ai/complete`, `/ai/chat`, `/ai/summarize`, `/ai/test-connection`, `GET /ai/usage`; usage recorded per call |
| **AI context** | `BuildContext`: project identity + AI Bible + chapter summaries (raw content fallback) + current scene + @[Entity] refs + story structure; Nexus identity system prompt on every chat |
| **AI Bible** | `projects.ai_instructions` (migration 016); auto-generated from guide steps on completion; editable on ProjectHome; 3 API routes |
| **Writing styles** | `internal/prompts`: `project_prompts` table; CRUD routes; style applied to AI calls via `prompt_id` |
| **Export** | Markdown (sync zip) + EPUB (async job, MinIO, presigned URL); `export_jobs` table; goroutine worker pool |
| **Novel guide** | 5-step wizard (Premise → Characters → World → Outline → First Scene); side effects populate wiki + manuscript; guide steps auto-fill AI Bible |
| **Story structures** | 12 seeded templates + scoring matrix; freeform option; structure badge on ProjectHome; phase banners in WikiHub timeline |
| **Collaboration** | Package stubbed; no HTTP registration |
| **Frontend** | React 18 + Vite + TypeScript + Tailwind; auth, project list, VSCode-style scene editor, act/chapter/scene explorer, wiki hub (entities/timeline/graph), git panel, **Nexus AI chat (SSE, identity, full story context), BeatInput, writing style selector, novel guide wizard, story structure picker, AI Bible editor, export panel, AI usage stats** |
| **Navigation** | TopBar: left nav (logo → Dashboard, Home, Wiki, Guide) + breadcrumb + right area (panel toggles, username, Settings, logout); editor fully navigable |
| **Settings** | AI provider keys (add/remove/test), Ollama URL + model selector, appearance (dark/light), account deletion |
| **OpenAPI + types** | `docs/openapi.yaml` (45+ routes incl. acts); `frontend/src/services/api-types.ts` generated; inline types for AI/prompts/usage/guide/structures |
| **CI/CD** | GitHub Actions (self-hosted) → GHCR → Ansible → dev VM; Go tests, tsc, ESLint, API-types drift, sqlc diff, Docker build + push, Ansible deploy |
| **Bruno collection** | Full integration tests for auth, health, projects, acts, chapters, scenes, wiki (incl. anchor tests), git |
| **README** | Written — prerequisites, quick start, env vars, Redis/MinIO note |
| **K8s / Helm** | Stubs — not yet used |

---

## Core features (product pillars)

1. **Accounts & access** — Register/login, JWT access + refresh, roles. *Done.*
2. **Manuscript structure** — Projects, chapters, scenes, ordering, summaries, tags; Git-backed. *API done; Git history stubs.*
3. **World wiki** — Entities (character/location/faction/item/concept/lore), relationships graph, timeline, magic rules, autolink. *API + Bruno tests done; no frontend yet.*
4. **AI-assisted writing** — Completion, chat, summarize, adapters, RAG. *B1 + B1.5 + B3 done. B2 (context/memory) next.*
5. **Export** — Markdown, EPUB, Scrivener. *B4 next.*
6. **Collaboration** — Real-time CRDT/WebSocket, Redis pub/sub. *Scaffold only.*
7. **Assets** — Covers and binaries via MinIO/S3. *Package stub; not integrated.*
8. **Writer UI** — React app: editor, wiki, AI panels, export. *Not started.*

---

## Phase A — Product skeleton

### A0 — Documentation & contracts
- [x] **A0.1** README — prerequisites, `make dev` + `make run`, env vars, smoke test
- [~] **A0.2** OpenAPI spec (`docs/openapi.yaml`) — **spec drift**: covers 37 paths through A+ / wiki / git / structures; missing B1–C0.5 (~20 routes: AI, prompts, export, guide, chapter summaries, ai-instructions). Frontend `api.ts` is hand-written for newer routes and working. Catch-up scheduled before C3 (collab). See Phase D.
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
- [x] **A+4.1** Frontend — `F11` toggles focus mode: hides ActivityBar, left panel, ProjectExplorer, TopBar, StatusBar
- [x] **A+4.2** Frontend — full-width `ScribeEditor`; floating `Esc` button (near-invisible until hovered); focus icon in TopBar for mouse users

### A+5 — Project home / stats page
- [x] **A+5.1** Backend — `GET /projects/:id/stats` endpoint: total word count, scene count, chapter count, last updated; SQL aggregate via JOIN across acts/chapters/scenes
- [x] **A+5.2** OpenAPI — `ProjectStats` schema added; `api-types.ts` regenerated; `api.projects.stats()` added to api.ts
- [x] **A+5.3** Frontend — `/projects/:id` is now `ProjectHome` (stat cards + quick-open editor/wiki actions); editor moved to `/projects/:id/editor`

### A+6 — User account deletion
- [x] **A+6.1** Backend — `DELETE /users/me`; `DeleteUser` + `ListProjectGitPaths` sqlc queries; git repos cleaned from disk after DB cascade; `GET /users/me` added
- [x] **A+6.2** OpenAPI — `GET /users/me` + `DELETE /users/me` documented; `api-types.ts` regenerated; `api.users.me` + `api.users.deleteMe` added to api.ts
- [x] **A+6.3** Frontend — Danger Zone section in `/settings`; confirm dialog requires user to type exact email; redirects to `/login` on success

### A+7 — Light theme
- [x] **A+7.1** Frontend — brand colors switched to CSS variables (`rgb(var(--brand-*) / <alpha-value>)`) in `tailwind.config.ts`; `:root` dark defaults + `.light` overrides in `theme.css`; `prefers-color-scheme` media query fallback; added missing `brand-text-muted` token
- [x] **A+7.2** Frontend — `themeStore.ts` (Zustand, localStorage); `App.tsx` initializes theme on mount; Appearance section in `/settings` with sun/moon toggle

### A+8 — Relationship graph visualization
- [x] **A+8.1** Frontend — `d3` installed; `RelationshipGraph.tsx` consuming `GET /wiki/graph`; 200-tick synchronous force layout snapshotted to React SVG state
- [x] **A+8.2** Frontend — nodes colored by entity type; directional arrow edges labeled with relationship type; halo on selected node; pan/zoom via d3-zoom; reset view button; entity type legend
- [x] **A+8.3** Frontend — "Graph" tab in `WikiHub` alongside Entities and Timeline; clicking a node switches to Entities tab and auto-opens entity detail

**Deferred to Phase B+ (AI or infra dependent):**

- Plot hole detection and narrative consistency checks (needs AI routes)
- Async worker / job queue (needed for heavy export jobs)
- Vector memory / RAG / embeddings — pgvector + chapter summary anchors
- Admin dashboard — system health, queue status, log access (Phase C+)

---

## Phase B — AI + export core

**Actionable checklist:** [specs/phase-b.md](./docs/specs/phase-b.md)

### B1 — AI proxy + adapters ✓
- [x] **B1.1** Backend — `Adapter` interface: `Complete/Chat/Summarize/StreamComplete/StreamChat/IsThinkingModel/Provider`; `CompleteMode` (`continue` | `beat`)
- [x] **B1.2** Backend — `OpenAIAdapter`, `AnthropicAdapter`, `OllamaAdapter`; `AdapterFactory` with thinking model auto-detection (`o1/o3/o4/deepseek-reasoner/qwq/r1`)
- [x] **B1.3** Backend — `POST /projects/:id/ai/complete` (beat + continue), `/ai/chat`, `/ai/summarize`; `GET /ai/usage`; SSE via `io.Pipe`; Ollama fallback when no cloud key
- [x] **B1.4** OpenAPI — deferred; types defined inline in `api.ts`
- [x] **B1.5** Frontend — `ChatBar` wired to `/ai/chat` with SSE; blinking cursor; Stop button; `sceneId` context passed through

### B1.5 — Writing styles (prose prompts) ✓
- [x] **B1.5.1** Backend — Migration 010: `project_prompts` table; `user_api_keys.force_non_streaming BOOL`
- [x] **B1.5.2** Backend — `GET/POST /projects/:id/prompts`, `PUT/DELETE /:promptId`; `applyPromptPreset` merges `system_content` override + `content` style block
- [x] **B1.5.3** OpenAPI — deferred; `PromptResponse` inline in `api.ts`
- [x] **B1.5.4** Frontend — Writing style dropdown in `SceneMetadataPanel`; lazy-loads on open; badge in panel header; `promptId` flows to every AI call
- [x] **B1.5.5** Frontend — `BeatInput` in `ScribeEditor` toolbar; beat sentence → SSE stream → prose preview; Accept/Retry/Discard; `api.ai.streamComplete` added

### B2 — AI memory + context ✓
- [x] **B2.1** Backend — Migration 012: `chapter_summaries(chapter_id, branch_name PK, ai_summary, stale)` + `project_active_branch(project_id, user_id PK, branch_name)`
- [x] **B2.2** Backend — `ResolveBranch` helper: `X-NexusTale-Branch` header → `project_active_branch` → `"canon"`; `TravelTo`/`Diverge` upsert `project_active_branch`
- [x] **B2.3** Backend — auto-summarize goroutine: debounce key `(chapter_id, branch_name)`; 30s quiet period; upserts `chapter_summaries`; marks stale immediately on scene save
- [x] **B2.4** Backend — `BuildContext`: summaries by active branch (canon fallback); `@[entity]` inline ref parsing; story structure phase injection; project identity preamble; current scene full text; raw content fallback for unsummarised chapters
- [x] **B2.5** Frontend — stale indicator badge on chapter; "Regenerate" button in ProjectExplorer; `X-NexusTale-Branch` header on all AI + scene-save requests

### B3 — Token usage tracking ✓
- [x] **B3.1** Backend — Migration 011: `ai_usage` table; `recordUsage` goroutine after every AI call (non-blocking); `GET /projects/:id/ai/usage` aggregate (total/monthly tokens + cost + call count)
- [x] **B3.2** OpenAPI — deferred; `AIUsageSummary` inline in `api.ts`
- [x] **B3.3** Frontend — AI usage row on ProjectHome (tokens total/month, calls/month, cost/month); hidden when no calls recorded yet

### B4 — Export ✓
- [x] **B4.1** Backend — Markdown synchronous zip: acts → chapters → scenes as `.md` with YAML front matter; streamed response
- [x] **B4.2** Backend — EPUB async job (Migration 013: `export_jobs`); 2-worker goroutine pool; go-epub → MinIO upload; presigned URL (1h TTL)
- [x] **B4.3** Backend — `GET /projects/:id/export/:job_id` polling; `POST /projects/:id/export` body `{format}`
- [x] **B4.4** Frontend — Export panel on ProjectHome: Markdown download (fetch → blob), EPUB trigger + 3s poll + download link

### B5 — Novel guide ✓
- [x] **B5.1** Backend — Migration 014: `guide_steps` table; `GET /projects/:id/guide`; `POST /projects/:id/guide/:step` (save); `POST .../complete` (side effects)
- [x] **B5.2** Backend — step side effects: Characters → wiki entities; World → location entities + magic rules; Outline → chapters; First scene → first scene content
- [x] **B5.3** Frontend — `/projects/:id/guide` linear wizard; step sidebar; skippable; resumes from last incomplete step

### B5.5 — Story structure (optional templates) ✓
- [x] **B5.5.1** Backend — Migration 015: `novel_structures` (12 seeded templates); `projects.structure_id UUID NULL`; `projects.structure_custom JSONB NULL`
- [x] **B5.5.2** Backend — `internal/guide/score.go`: deterministic scoring matrix; 8 unit tests; freeform recommended when no structure clears threshold
- [x] **B5.5.3** Backend — `GET /novel-structures` (no auth), `POST .../guide/structure/score`, `GET/PUT .../structure`
- [x] **B5.5.4** Frontend — Guide Step 3.5: 4-path chooser (questionnaire / browse templates / freeform / skip); result card with "Use / Choose different / Continue without"
- [x] **B5.5.5** Frontend — Structure badge on ProjectHome; `BuildContext` injects `## Story structure` block with phase list
- [x] **B5.5.6** Frontend — WikiHub timeline: era-grouped events; muted italic phase banners above each group when structure selected

## Phase C — Polish + depth

Scale key: **Light** · **Medium** · **Heavy** · **Heavier** · **Heaviest**

### C0 — Pre-C polish ✓ (2026-04-14)
- [x] **`[Light]`** Editor navigation — TopBar full redesign: left nav (logo/Home/Wiki/Guide) + breadcrumb + right (toggles/username/Settings/logout)
- [x] **`[Light]`** AI connection test in Settings — per-provider Test button; Ollama returns model list; cloud returns "Connected" or error
- [x] **`[Light]`** Nexus rename — ChatBar → Nexus AI; on-theme intro shown only when ≥1 key configured; no-connection message with Settings link
- [x] **`[Light]`** Per-user Ollama model selection — `user_api_keys(provider="ollama_model")`; model list from Test Connection is clickable to save

### C0.5 — AI context quality ✓ (2026-04-14)
- [x] **`[Medium]`** BuildContext enrichment — project identity always injected; raw scene content fallback for chapters without summaries; current scene labeled + included; N+1 entity query fixed
- [x] **`[Light]`** StreamChat Nexus identity — always prepends "You are Nexus…" system prompt; context appended
- [x] **`[Heavy]`** AI Bible — migration 016 `projects.ai_instructions`; guide auto-fills on step completion (only when empty); `GET/PUT /projects/:id/ai-instructions` + `POST .../generate`; injected as `## Story bible` in every AI call; ProjectHome card with autosave textarea + Regenerate button

### C1 — Export depth ← next
- [ ] **`[Medium]`** DOCX export — add to existing export worker (reuse job table + MinIO path); clean manuscript formatting (headings, scene breaks, front matter)
- [ ] **`[Medium]`** Wiki image upload — presigned MinIO upload for entity portrait images; stored URL displayed in entity detail panel

### C2 — AI depth
- [ ] **`[Heavy]`** Explicit AI context panel — writer-curated additions to the AI context window: pin wiki entities by name or tag, include specific chapters/scenes as full text or summary
- [ ] **`[Heavy]`** Multi-session Workshop — named persistent chat sessions per project (`workshop_sessions` table); each session stores `[{role, content, timestamp}]`; sidebar panel; exportable to Markdown
- [ ] **`[Medium]`** Prompt history browser — store first 500 chars of assembled prompt + beat text in `ai_usage`; UI panel to browse and re-apply previous beats
- [ ] **`[Light]`** Import/export writing styles — download project style presets as JSON; import into another project

### C3 — Collaboration (last, largest)
- [ ] **`[Heaviest]`** WebSocket + CRDT — real-time co-editing per scene; CRDT library choice (Yjs vs Automerge — lock before starting); Redis pub/sub fan-out; presence indicators; roles (editor/commenter/viewer) + invite flow; Git snapshot on idle

## Phase D — Premium / advanced

- Map builder; image generation pipelines for wiki entities
- Scrivener / Fountain export; advanced Git branching UX
- Series-level continuity management
- Multi-region, scale-out collaboration tuning
- **OpenAPI catch-up** — bring `docs/openapi.yaml` current with all B1–C routes; regenerate `api-types.ts`; restore codegen for newer endpoints (schedule before C3)

### D-Desktop — Native desktop app (optional, Tauri-based)

> Prerequisite: SQLite migration (heavy). Do not start until C-series is stable.

The existing React frontend and Go backend are well-suited for desktop packaging. No frontend code changes required.

**Phase 1 — Tauri wrapper** `[Medium]`
- Add Tauri to the frontend; bundle compiled Go API binary as a Tauri sidecar
- Tauri starts/stops the Go process on app launch/close
- API base URL resolved dynamically to `http://localhost:<port>`
- Still requires Docker for Postgres at this stage — partial desktop only
- Output: `.app` / `.exe` / `.deb` that launches the full stack

**Phase 2 — SQLite + local storage** `[Heavy]`
- Add sqlc SQLite driver; port queries (most translate directly)
- Replace MinIO with local file system (`~/Library/Application Support/NexusTale/` on macOS, `%APPDATA%` on Windows, `~/.local/share` on Linux)
- Drop Redis (not needed until real-time collab; replace with in-memory stub)
- golang-migrate SQLite runner; new migration set
- Output: fully self-contained — no Docker, no external services

**Phase 3 — Packaging + auto-update** `[Medium]`
- Code signing (Apple Developer ID, Windows Authenticode)
- Tauri updater wired to GitHub releases
- CI: Go cross-compile matrix + Tauri build for macOS/Windows/Linux
- Output: signed installers with silent auto-update

**What stays identical:** all React code, all Go business logic, all Git versioning (already file-based), all Ollama integration (desktop users run Ollama natively — no URL configuration needed)

---

## How to use this file

Treat unchecked items as **Claude Code / issue seeds**: one checkbox → one focused task with acceptance criteria. For deep design, add `docs/specs/<topic>.md` and link from a roadmap line.

*Last updated 2026-04-14: Phase A, A+, and B fully complete. C0 (editor navigation, AI connection test, Nexus rename, Ollama model selector) and C0.5 (AI context enrichment, AI Bible) complete. Next: C1 — DOCX export + wiki image upload.*
