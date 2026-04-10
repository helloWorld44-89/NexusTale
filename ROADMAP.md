# NexusTale roadmap

Sci-fi/fantasy novel-writing tool: structured manuscripts (projects → chapters → scenes), worldbuilding, AI-assisted drafting, export, and (eventually) collaboration.

**Companion docs:** [CLAUDE.md](./CLAUDE.md) (how to work in this repo), [docs/PROJECT_PLAN.md](./docs/PROJECT_PLAN.md) (full architecture + phases), [docs/specs/phase-a-mvp.md](./docs/specs/phase-a-mvp.md) (Phase A checklist), [Makefile](./Makefile) (dev commands).

---

## Current state (snapshot)

| Area | Status |
|------|--------|
| **API shell** | Go 1.23 + Gin; `/healthz`; `/api/v1/auth/*`; `/api/v1/projects/*` (CRUD + acts + chapters + scenes), JWT + refresh tokens |
| **Database** | PostgreSQL migrations (009) + **sqlc** (`pkg/db/queries` → `pkg/db/sqlcgen`) |
| **Manuscript hierarchy** | **Project → Act → Chapter → Scene**; act layer hidden in UI for single default act; full CRUD + integration tests + Bruno |
| **Git per project** | Non-bare repos on disk; full Chronicle/Lore/Echo/Diverge/TravelTo/Canonize API; 21 handler integration tests; fast-forward merge; Paradox detection |
| **Wiki v1** | `wiki_entities`, `wiki_relationships`, `wiki_magic_rules`, `wiki_timeline_events` — full CRUD + timeline anchoring; all with integration tests; autolink + graph endpoints |
| **Redis / MinIO** | Provisioned in dev compose; MinIO targeted by B4 EPUB export |
| **AI proxy** | `internal/ai`: Anthropic, OpenAI, Ollama adapters; beat + continue modes; `POST /ai/complete`, `/ai/chat`, `/ai/summarize`, `GET /ai/usage`; usage recorded per call |
| **Writing styles** | `internal/prompts`: `project_prompts` table; CRUD routes; style applied to AI calls via `prompt_id` |
| **Collaboration, export** | Packages stubbed; no HTTP registration |
| **Frontend** | React 18 + Vite + TypeScript + Tailwind; auth, project list, VSCode-style scene editor, act/chapter/scene explorer, wiki hub, git panel, **ChatBar SSE chat, BeatInput, writing style selector, AI usage stats on ProjectHome** |
| **OpenAPI + types** | `docs/openapi.yaml` (45+ routes incl. acts); `frontend/src/services/api-types.ts` generated; inline types for AI/prompts/usage (not yet in spec) |
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

### B2 — AI memory + context
- [ ] **B2.1** Backend — Migration 012: `chapter_summaries(chapter_id, branch_name PK, ai_summary, stale)` + `project_active_branch(project_id, user_id PK, branch_name)` — no column added to `chapters`
- [ ] **B2.2** Backend — `ResolveBranch` helper: `X-NexusTale-Branch` header → `project_active_branch` → `"canon"`; `TravelTo`/`Diverge` handlers upsert `project_active_branch` after git HEAD switch
- [ ] **B2.3** Backend — auto-summarize goroutine: debounce key `(chapter_id, branch_name)`; upserts `chapter_summaries` per branch; only marks active branch stale on scene save
- [ ] **B2.4** Backend — `BuildContext(…, branchName)`: summaries queried by active branch, falling back to `"canon"`; inline `@[entity]` parsing (up to 5 entries injected before user turn); `Canonize` deletes merged branch's summary rows
- [ ] **B2.5** Backend — `ChapterResponse.ai_summary` sourced from `chapter_summaries` for requesting user's active branch; update OpenAPI; regenerate types
- [ ] **B2.6** Frontend — stale indicator badge on chapter; "Regenerate" button in SceneMetadataPanel; `X-NexusTale-Branch` header sent on all AI and scene-save requests

### B3 — Token usage tracking ✓
- [x] **B3.1** Backend — Migration 011: `ai_usage` table; `recordUsage` goroutine after every AI call (non-blocking); `GET /projects/:id/ai/usage` aggregate (total/monthly tokens + cost + call count)
- [x] **B3.2** OpenAPI — deferred; `AIUsageSummary` inline in `api.ts`
- [x] **B3.3** Frontend — AI usage row on ProjectHome (tokens total/month, calls/month, cost/month); hidden when no calls recorded yet

### B4 — Export
- [ ] **B4.1** Backend — `GET /projects/:id/export/markdown` synchronous zip: acts → chapters → scenes as `.md` with YAML front matter
- [ ] **B4.2** Backend — `POST /projects/:id/export/epub` async job (Migration 012: `export_jobs`); goroutine pool; MinIO upload; presigned URL (1h TTL)
- [ ] **B4.3** Backend — `GET /projects/:id/export/jobs/:jobId` polling; OpenAPI schemas; regenerate types
- [ ] **B4.4** Frontend — Export section on ProjectHome: Markdown download button; EPUB trigger + 3s poll + download link

### B5 — Novel guide
- [ ] **B5.1** Backend — Migration 013: `guide_steps` table; `GET /projects/:id/guide`; `POST /projects/:id/guide/:stepKey` with step side effects
- [ ] **B5.2** Backend — step handlers: Premise → update project description; Characters → create wiki entities; World → entities + magic rule; Outline → chapters; First scene → scene + optional AI opening
- [ ] **B5.3** OpenAPI — guide endpoints + `GuideStepResponse`; regenerate types
- [ ] **B5.4** Frontend — `/projects/:id/guide` linear wizard; step sidebar with ✓ / current / muted states; skip allowed; "Finish guide" → ProjectHome

### B5.5 — Story structure (optional templates)
> Structure is a tool, not a requirement — freeform is a first-class choice at every step.

- [ ] **B5.5.1** Backend — Migration 015: `novel_structures` table (seeded with 12 templates from `NOVEL_STRUCTURES.md`); `projects.structure_id UUID NULL`; `projects.structure_custom JSONB NULL`
- [ ] **B5.5.2** Backend — `internal/guide/score.go`: deterministic scoring matrix from `STRUCTURESELCTION.md`; returns empty when no structure clears threshold (freeform recommended)
- [ ] **B5.5.3** Backend — `GET /novel-structures` (no auth), `POST /projects/:id/guide/structure/score` (score-only, no side effects), `GET/PUT /projects/:id/structure`
- [ ] **B5.5.4** OpenAPI — structure schemas; regenerate types
- [ ] **B5.5.5** Frontend — Guide Step 3.5: 4-path chooser (questionnaire / browse / freeform / skip); "Continue without structure" always visible; questionnaire → score → result card with "Use / Choose different / Continue without"
- [ ] **B5.5.6** Frontend — Structure badge on ProjectHome (only shown when a structure is selected); `BuildContext` injects phase context only when present
- [ ] **B5.5.7** Frontend — Timeline tab in WikiHub: when a structure is selected, phase banners (muted, italic) appear above each act's events using index-based mapping; renders identically to today when no structure set; client-side join, no new backend route

## Phase C — Collaboration + depth

- WebSocket + CRDT for scene editing; roles and project invites
- DOCX export; wiki image upload for entity portraits
- **Explicit AI context panel** — writer-curated context: pin specific wiki entries by ID or tag, include scenes as full text or summary-only, per-chapter granularity (power-user alternative to the automatic context window)
- **Multi-session Workshop** — promote `ChatBar` to tabbed named sessions per project; each session persists `[{role, content, timestamp}]`; separate `category: "workshop"` system prompt; export session to Markdown
- **Prompt history browser** — store first 500 chars of assembled prompt + the user's beat/instruction in `ai_usage`; UI panel to browse and re-apply previous beats
- **Import/export writing styles** — download project styles as JSON; import from file or another project

## Phase D — Premium / advanced

- Map builder; image generation pipelines for wiki entities
- Scrivener / Fountain export; advanced Git branching UX
- Series-level continuity management
- Multi-region, scale-out collaboration tuning

---

## How to use this file

Treat unchecked items as **Claude Code / issue seeds**: one checkbox → one focused task with acceptance criteria. For deep design, add `docs/specs/<topic>.md` and link from a roadmap line.

*Last updated: Phase A + A+ fully done. Phase B in progress — B1 (AI proxy), B1.5 (writing styles + beat input), B3 (token tracking) complete. B2 (AI memory/context), B4 (export), B5 (novel guide), B5.5 (story structures) remain.*
