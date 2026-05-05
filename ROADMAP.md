# NexusTale roadmap

Sci-fi/fantasy novel-writing tool: structured manuscripts (projects → chapters → scenes), worldbuilding, AI-assisted drafting, export, and (eventually) collaboration.

**Companion docs:** [CLAUDE.md](./CLAUDE.md) (how to work in this repo), [docs/PROJECT_PLAN.md](./docs/PROJECT_PLAN.md) (full architecture + phases), [docs/specs/phase-a-mvp.md](./docs/specs/phase-a-mvp.md) (Phase A checklist), [Makefile](./Makefile) (dev commands).

---

## Current state (snapshot)

| Area | Status |
|------|--------|
| **API shell** | Go 1.25 + Gin; `/healthz`; `/api/v1/auth/*`; `/api/v1/projects/*` (CRUD + acts + chapters + scenes), JWT + refresh tokens |
| **Database** | PostgreSQL migrations **(028)** + **sqlc** (`pkg/db/queries` → `pkg/db/sqlcgen`) |
| **Manuscript hierarchy** | **Project → Act → Chapter → Scene**; act layer hidden in UI for single default act; full CRUD + integration tests + Bruno |
| **Git per project** | Non-bare repos on disk; full Chronicle/Lore/Echo/Diverge/TravelTo/Canonize API; 21 handler integration tests; fast-forward merge; Paradox detection |
| **Wiki v1** | `wiki_entities`, `wiki_relationships`, `wiki_magic_rules`, `wiki_timeline_events` — full CRUD + timeline anchoring; all with integration tests; autolink + graph endpoints; relationship graph (d3 force) |
| **Redis / MinIO** | Provisioned in dev compose; MinIO used for EPUB export (async job → presigned URL) |
| **AI proxy** | `internal/ai`: Anthropic, OpenAI, Ollama adapters; beat + continue modes; chapter summaries + AI Bible in every call; `POST /ai/complete`, `/ai/chat`, `/ai/summarize`, `/ai/test-connection`, `GET /ai/usage`; usage recorded per call |
| **AI context** | `BuildContext`: project identity + AI Bible + chapter summaries (raw content fallback) + current scene + @[Entity] refs + story structure; Nexus identity system prompt on every chat |
| **AI Bible** | `projects.ai_instructions` (migration 016); auto-generated from guide steps on completion; editable on ProjectHome; 3 API routes |
| **Writing styles** | `internal/prompts`: `project_prompts` table; CRUD routes; style applied to AI calls via `prompt_id` |
| **Export** | Markdown (sync zip) + EPUB + DOCX (async jobs, MinIO, presigned URL); `export_jobs` table; goroutine worker pool (`asyncJob{format}`) |
| **Novel guide** | 5-step wizard (Premise → Characters → World → Outline → First Scene); side effects populate wiki + manuscript; guide steps auto-fill AI Bible |
| **Story structures** | 12 seeded templates + scoring matrix; freeform option; structure badge on ProjectHome; phase banners in WikiHub timeline |
| **Collaboration** | C3.0–C3.5 all complete — roles, invite system, clone-per-collaborator; collaborator-scoped git ops, branch-prefix enforcement, reviewer read-only; notifications (migration 026) + `NotificationBell`; **merge requests** (migration 027) — `internal/merge` open/list/get/diff/resolve; `FetchBranchFromClone` + `EchoBranches` on `GitService`; per-scene `SceneDiff` parsing; fast-forward canonize + HasParadox detection; `mr_*` notifications; `MergeRequestsPanel` + **`ProseDiffViewer`** (word-level diff-match-patch, per-scene resolution, bulk accept); **reviewer annotations** (migration 028 `manuscript_annotations`, `internal/annotations`, `AnnotationSidebar`, floating popover in `ScribeEditor`, note/suggestion/question types, `jumpToAnnotation` imperative handle) |
| **Frontend** | React 18 + Vite + TypeScript + Tailwind; auth, project list, VSCode-style scene editor, act/chapter/scene explorer, wiki hub (entities/timeline/graph/research notes), git panel, **Nexus AI chat (SSE, identity, full story context), BeatInput, writing style selector, novel guide wizard, story structure picker, AI Bible editor, export panel, AI usage stats, context pins panel, multi-session Workshop panel, NotificationBell (60s polling, unread badge, dropdown)** |
| **Navigation** | TopBar: left nav (logo → Dashboard, Home, Wiki, Guide) + breadcrumb + right area (panel toggles, username, Settings, logout); editor fully navigable |
| **Settings** | AI provider keys (add/remove/test), Ollama URL + model selector, appearance (dark/light), account deletion |
| **OpenAPI + types** | `docs/openapi.yaml` (45+ routes incl. acts); `frontend/src/services/api-types.ts` generated; inline types for AI/prompts/usage/guide/structures |
| **CI/CD** | GitHub Actions (self-hosted) → GHCR → Ansible → dev VM; Go tests, tsc, ESLint, API-types drift, sqlc diff, Docker build + push, Ansible deploy |
| **Bruno collection** | Full integration tests for auth, health, projects, acts, chapters, scenes, wiki (incl. anchor tests), git, collaboration (C3.0 + C3.1 — 44 tests in `10-collaboration/`) |
| **README** | Written — prerequisites, quick start, env vars, Redis/MinIO note |
| **K8s / Helm** | Stubs — not yet used |

---

## Core features (product pillars)

1. **Accounts & access** — Register/login, JWT access + refresh, roles. *Done.*
2. **Manuscript structure** — Projects, chapters, scenes, ordering, summaries, tags; Git-backed. *API done; Git history stubs.*
3. **World wiki** — Entities (character/location/faction/item/concept/lore), relationships graph, timeline, magic rules, autolink. *API + Bruno tests done; no frontend yet.*
4. **AI-assisted writing** — Completion, chat, summarize, adapters, RAG. *B1 + B1.5 + B3 done. B2 (context/memory) next.*
5. **Export** — Markdown, EPUB, Scrivener. *B4 next.*
6. **Collaboration** — Git-backed async: per-collaborator clones, invite system, merge requests, prose diff + conflict resolution, reviewer annotations, notifications. *C3.0 + C3.1 + C3.5 + C3.2 done.*
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
- [x] **B4.3** Backend — `GET /projects/:id/export/:job_id` polling; `POST /projects/:id/export` body `{format: "markdown"|"epub"|"docx"}`
- [x] **B4.4** Frontend — Export panel on ProjectHome: Markdown download (fetch → blob), EPUB + DOCX trigger + 3s poll + download link

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

### C1 — Export depth ✅ complete
- [x] **`[Medium]`** DOCX export — raw OOXML builder (`internal/export/docx.go`); Times New Roman 12pt double-spaced; page breaks between chapters; `# # #` scene breaks; no new dependency (2026-04-15)
- [x] **`[Medium]`** Wiki image upload — migration 017 `wiki_entities.image_key TEXT`; multipart upload → backend → MinIO (`PutObject`/`DeleteObject`); `PresignedGetURL` (4 hr TTL) in `EntityResponse.image_url`; portrait display + upload/remove in `EntityDetail`; OpenAPI spec updated + types regenerated (2026-04-15)

### C2 — AI depth
- [x] **`[Heavy]`** Explicit AI context panel — writer-curated additions to the AI context window: pin wiki entities by name or tag, include specific chapters/scenes as full text or summary
- [x] **`[Heavy]`** Multi-session Workshop — named persistent chat sessions per project (`workshop_sessions` table); each session stores `[{role, content, timestamp}]`; sidebar panel; exportable to Markdown
- [x] **`[Medium]`** Research notes — freeform per-project scratchpad (`research_notes` table: title, body, source_url, tags); accessible from WikiHub "Research" tab and injectable into AI context; designed for web quotes, worldbuilding references, and craft notes
- [x] **`[Medium]`** Prompt history browser — migration 021 adds `mode/beat_text/scene_id` to `ai_usage`; `GET /ai/beat-history` (deduplicated by beat text); "Recent beats" list inside BeatInput (lazy-loaded, shown when input empty, click to re-fill)
- [x] **`[Light]`** Import/export writing styles — download project style presets as JSON; import into another project

### C2.5 — AI manuscript tools (agent write access)

The author opts in to letting Nexus write directly to the manuscript — appending prose, creating scenes, chapters, and acts — from any AI panel. Modelled after Claude Code's file-write tools.

- [x] **`[Light]`** Continue button — "Continue →" in the ScribeEditor toolbar; streams `/ai/complete?mode=continue`; same Accept/Retry/Discard flow as BeatInput; no backend changes needed
- [x] **`[Light]`** Insert into scene — hover-reveal "insert into scene" button on every completed assistant message in Nexus chat and Workshop; appends text to active scene with autosave; only shown when a scene is active
- [x] **`[Medium]`** Manuscript tool definitions — `adapters/tools.go` (ToolAdapter interface); Anthropic + OpenAI `ChatTools` + `BuildToolResultMessages`; `ai/tools.go` defines 5 tools (append_to_scene, replace_scene_content, create_scene, create_chapter, create_act) + executor; `StreamChatWithTools` agentic loop (max 10 rounds); `tools_enabled` in WorkshopChat; WorkshopPanel Agent toggle + tool notice banners
- [x] **`[Medium]`** Tool execution author control — `ToolEvent` SSE struct carries undo metadata (before_content, scene_id, chapter_id, created_id/type); per-action Undo in Workshop (restores scene content or deletes created entity); collapsible `AgentRunBlock` groups tool events per run; live scene refresh via `onToolWrite` callback; `api.scenes.get/delete` + `api.chapters.delete` added
- [x] **`[Heavy]`** Agent mode in Workshop — `maxRounds` param (default 25); `{agent_planning, round}` SSE events per model round; `AgentPhase` state (planning/executing/replying) drives status bar copy; Stop button always visible during agent run with "content is kept" tooltip; agent-optimized 2-row input with task-focused placeholder; `NexusThinking` component (18 general + 10 agent sci-fi/fantasy phrases, cycling with fade, shown before first token in ChatBar / Workshop / BeatInput)

### C3 — Collaboration (last, largest)

Architecture: **git-backed async** — each collaborator gets a per-project git clone (`repos/{projectId}-collab-{userId}/`) on their own branch. Owner merges via merge requests (like a manuscript PR). No CRDT / WebSocket needed for MVP; Redis used for future notification push only.

- [x] **`[Heavy]`** **C3.0 — Roles + invite system** ✓ (2026-04-19) — Migration 022 (`users.plan` free/writer/studio); migration 023 (`project_collaborators` extended + `project_invites`); `internal/collaboration` package (service + handler + `RequireProjectAccess` middleware); roles: co-author/editor/reviewer; 32-byte hex invite token, 7d TTL, email-matched accept, clone + branch on accept; `ListProjectsForUser` union query for dashboard; frontend: `CollaboratorsPanel`, `InviteAccept` page, Login redirect param, router entry; `api.collaboration.*` in api.ts
- [x] **`[Heavy]`** **C3.1 — Collaborator-scoped git operations** ✓ — Chronicle, Diverge, Lore scoped to collaborator clone via `repoPathForUser`; branch prefix enforcement (403 outside own prefix); reviewer blocked from Chronicle; 44 Bruno tests in `10-collaboration/`
- [x] **`[Heavier]`** **C3.2 — Merge request system** ✓ — Migration 027 `merge_requests` table; open/list/get/diff/resolve flows; `FetchBranchFromClone` + `EchoBranches`; per-scene `SceneDiff` parsing; fast-forward Canonize + HasParadox → 400; `mr_*` notifications; `MergeRequestsPanel` on ProjectHome
- [x] **`[Heavy]`** **C3.3 — Prose diff + conflict resolution UI** ✓ — `ProseDiffViewer.tsx`; `diff-match-patch` word-level diff; `extractTexts` reconstructs canon/coauthor text from unified diff; per-scene Keep Canon / Use Co-author / manual editor; bulk accept; merge blocked until all conflicts resolved; "Review Diff →" overlay in `MergeRequestsPanel`
- [x] **`[Heavy]`** **C3.4 — Reviewer annotations** ✓ — Migration 028 `manuscript_annotations` (char offset, type: note/suggestion/question, resolved_by); `internal/annotations` (service + handler); floating selection popover in `ScribeEditor` (`forwardRef` + `useImperativeHandle`); `AnnotationSidebar.tsx` right panel with open/resolved sections; `jumpToAnnotation` impl via `setSelectionRange`; ActivityBar "Annotations" button + unread badge; `api.annotations.*` in api.ts
- [x] **`[Medium]`** **C3.5 — Notifications** ✓ — Migration 026 `notifications` table; `internal/notifications` service + handler; `NotificationBell.tsx` in TopBar; 60s poll; invite_received, mr_opened/approved/rejected/merged events; mark-read / mark-all-read

## Phase C+ — Security & Code Review (pre-alpha gate)

All items must be resolved or explicitly deferred before the alpha invite goes out.
Priority tags: **P0** = blocks alpha · **P1** = fix before beta · **P2** = nice-to-have.

### Security review

**Authentication & session management**
- [x] **P0** `NEXUSTALE_JWT_SECRET` and `NEXUSTALE_ENCRYPTION_KEY` rotated to ≥32-byte random values in prod (not dev defaults) — `config.ValidateProd()` now exits on startup if defaults detected in release mode; `NEXUSTALE_SERVER_MODE=release` and `NEXUSTALE_ENCRYPTION_KEY` injected via Ansible; add `NEXUSTALE_ENCRYPTION_KEY` GitHub secret (`openssl rand -hex 32`)
- [x] **P0** MinIO root credentials changed from defaults (`minioadmin`) — `config.ValidateProd()` now rejects defaults in release mode; credentials injected via Ansible from GitHub secrets; ensure `NEXUSTALE_MINIO_ACCESSKEY` / `NEXUSTALE_MINIO_SECRETKEY` secrets are non-default values
- [x] **P0** CORS: `AllowOrigins` locked to the app domain, not `*`, in prod — `corsMiddleware` wired; `NEXUSTALE_SERVER_ALLOWEDORIGIN` env var; `ValidateProd` rejects `*` in release mode
- [x] **P0** TLS on all external traffic — nginx terminates (Let's Encrypt via certbot); HSTS header set; `infra/ansible/templates/nginx.ssl.conf.j2` (TLSv1.2/1.3, Mozilla Intermediate ciphers, HSTS 2yr); certbot standalone for initial issuance + weekly renewal cron with pre/post hooks; `docker-compose.deploy.yml` mounts `/etc/letsencrypt` + `nginx.ssl.conf`; add `NEXUSTALE_DOMAIN` + `NEXUSTALE_ALERT_EMAIL` GitHub secrets
- [x] **P1** Refresh token single-use rotation policy — `DeleteRefreshToken` called in `Refresh()` before issuing new pair; audited clean
- [x] **P1** `RequireProjectAccess` middleware: `RequireChapterAccess` middleware added to `collaboration/middleware.go` — reads `:cid`, looks up `chapter.ProjectID`, applies identical owner/collaborator check; wired onto `chaptersGroup` in `main.go`; all scene routes now enforce project membership
- [x] **P1** API key storage: `toAPIKeyResponse()` maps only `ID`/`Provider`/`KeyHint`; `EncryptedKey` never in any response; `DecryptAPIKey` internal only; audited clean
- [ ] **P2** httpOnly + Secure + SameSite flags on any future cookie use (access token is in-memory / Authorization header today — this is precautionary)

**Input validation & injection**
- [x] **P0** Git branch names from user input (`branch_name`, `from_branch`) validated against `^[a-zA-Z0-9/_-]+$` — `validateBranchName` in `project/handler.go` (Diverge/TravelTo/Canonize) + `branchNameRE` in `merge/handler.go` (from_branch)
- [x] **P1** File uploads (wiki images): magic-byte sniffing via `http.DetectContentType` added to `UploadEntityImage`; extension pre-check + magic check both required; SVG absent from both allowlists; `router.MaxMultipartMemory = 5 MiB` set in `main.go`
- [x] **P1** DOCX/EPUB export: `xmlEscape` applied to all user strings in `buildDocumentXML` (title, chapter title, scene title, body paragraphs); style-name args are constants; EPUB chapter title/project title auto-escaped by go-epub's `encoding/xml` marshaler; audited clean
- [x] **P1** AI prompt: `BuildContext` output appended via `+` string concat into system prompt; no template engine; no injection vector; audited clean
- [ ] **P2** Timeline anchor DFS cycle detection: add depth limit (currently unbounded recursion on malformed data)

**Access control**
- [x] **P0** Non-owner cannot approve, reject, or merge MRs — `ResolveMergeRequest` in `merge/service.go` checks `p.OwnerID != callerID` → 403 (verified correct)
- [x] **P0** Non-owner cannot resolve annotations — `Resolve` in `annotations/service.go` now checks `p.OwnerID != resolverID` → 403
- [x] **P1** Collaborator can only read/write their own clone path — all `repoPathForUser` callers pass `auth.GetUserID(c)` / `claims.UserID` from verified JWT; no user-supplied ID accepted; audited clean
- [x] **P1** `DELETE /users/me` cascade: `ListUserWikiImageKeys`, `ListUserExportMinioKeys`, `ListUserCollaboratorClonePaths` sqlc queries added; `auth.Service.WithStorage` wired from `main.go`; MinIO objects + clone dirs cleaned best-effort after DB cascade
- [ ] **P2** Rate limiting on `POST /auth/login` and `POST /auth/register` (brute-force and account enumeration risk)
- [ ] **P2** Rate limiting on AI endpoints (`/ai/complete`, `/ai/chat`) — cost-abuse protection for users sharing server-side keys

**Dependencies**
- [x] **P1** `govulncheck ./...` in backend — fixed `GO-2026-4910` (go-git v5.12.0 → v5.17.1, malicious idx DoS) and `GO-2025-3553` (golang-jwt v5.2.1 → v5.2.2, header parsing DoS in `ValidateAccessToken`); clean
- [x] **P1** `npm audit --audit-level=high` in frontend — 0 high/critical; 2 moderate (esbuild/vite dev-server only, fix requires breaking vite@8 upgrade — deferred to Phase D); postcss XSS fixed via `npm audit fix`
- [x] **P1** Review `go-git` version for known path traversal CVEs — `GO-2026-4910` found and fixed (v5.12.0 → v5.17.1)

---

### Code review

**Backend**
- [x] **P0** `ScheduleSummarize`: debounce map cleanup — `cancelForChapter` on debouncer; `CancelSummarize(chapterID)` on `ai.Service` satisfies new `SummaryNotifier` interface method; `DeleteChapter` in `project/service.go` calls it before DB delete
- [x] **P0** `AcceptInvite`: non-atomic clone + DB insert — on `CreateCollaborator` error, `os.RemoveAll(clonePath)` is called to clean up the orphaned working tree; warning logged if cleanup also fails
- [x] **P1** Git operations: `GitService.repoLock` (per-path `sync.Mutex` map guarded by `sync.Mutex`) already serialises all write ops — `Chronicle`, `Diverge`, `TravelTo`, `Canonize`, `FetchBranchFromClone`; read ops are safely concurrent; audited clean
- [x] **P1** All handlers use `handleError(c, err)` — the two `c.JSON(500)` lines in `collaboration/handler.go` and `annotations/handler.go` are inside each file's own `handleError` function; no bypass; audited clean
- [x] **P1** SSE goroutines: `defer pw.Close()` confirmed on all three SSE pipes (`Complete`, `Chat`, `WorkshopChat`); audited clean
- [x] **P1** `buildPinnedContext` / `appendPinnedNote` (full mode): `pinnedContentLimit = 2000` constant applied in `appendPinnedScene`, `appendPinnedNote`, `appendPinnedEntity`; audited present in `context.go`
- [ ] **P2** `numericToFloat64()`: verify COALESCE(SUM(...)) nil handling when the aggregate returns zero rows (Postgres returns NULL for SUM over empty set even with COALESCE on the column)
- [ ] **P2** DB pool: confirm `context.Background()` is not used in request handlers — all queries should respect the request context for proper timeout/cancellation

**Frontend**
- [x] **P0** React error boundaries — `ErrorBoundary.tsx` class component created; all major panels in `Editor.tsx` wrapped (Nexus, Context, Workshop, Chronicle, Wiki, Annotations, scene editor, project explorer); shows error message + "Try again" reset button
- [x] **P1** `ScribeEditor` navigate-away race: `pendingSaveRef` stores current save callback; flushed via `useEffect` cleanup on `selectedSceneId` change and via `beforeunload` handler
- [x] **P1** SSE `EventSource` cleanup: `useEffect(() => () => abortRef.current?.abort(), [])` added to `ChatBar`, `WorkshopPanel`, and `BeatInput` — aborts in-flight fetch streams on unmount
- [x] **P1** `ProseDiffViewer`: 20-per-page pagination with Prev/Next controls replaces full synchronous render of all `SceneDiffCard` components
- [ ] **P2** `localStorage` for access token: consider moving to an in-memory variable (survives React re-renders via module scope) to reduce XSS token exposure surface; refresh token flow already handles persistence
- [ ] **P2** `api.ts`: confirm no `Authorization` header is ever sent to third-party origins; the fetch wrapper should assert `url.startsWith('/')` or compare to configured base URL

**API contract**
- [x] **P1** OpenAPI spec (`docs/openapi.yaml`) brought current — 49 operations added across AI, export, guide, prompts, research, collaboration, merge requests, notifications, and annotations; spec grew from 1907 → 3335 lines; `api-types.ts` regenerated clean (`npm run gen:api`); `npx tsc --noEmit` passes
- [ ] **P2** No versioning strategy beyond `/api/v1/` prefix — document policy for breaking changes before beta clients exist

---

## Alpha release plan

**Alpha definition:** invite-only (target 20–50 writers), solo-writer focused, no SLA. The dev VM doubles as the alpha environment. No public sign-up until beta.

### Feature scope

| Area | In alpha | Deferred |
|------|----------|----------|
| Manuscript (write/outline/branch/export) | ✅ all | — |
| Wiki (entities/relationships/timeline/magic/graph) | ✅ all | — |
| AI (Nexus, Workshop, Beat, Context pins, Bible) | ✅ all | RAG/embeddings |
| Novel guide + story structure | ✅ all | — |
| Collaboration (invite, clone, MR, annotations, notifications) | ✅ all | — |
| Exports (Markdown, EPUB, DOCX) | ✅ all | Scrivener, Fountain, PDF |
| Monetization | ❌ | Phase D |
| Map builder v2 / image generation | ❌ | Phase D |
| Desktop app (Tauri) | ❌ | Phase D |
| Customizable workspaces | ❌ | Phase D |

### Environment checklist

- [ ] **P0** TLS certificate provisioned for the alpha domain (Let's Encrypt via certbot in Ansible)
- [ ] **P0** All P0 security items above resolved
- [x] **P0** Postgres daily backup — `pg_dump` cron via Ansible (02:00 daily); gzip to `/opt/backups/nexustale/db/`; 7-day retention; dumps on same VM disk for alpha — add off-host rclone sync before beta
- [x] **P0** Git repo backup — `docker run alpine tar` against the `git-repos` volume (02:15 daily); gzip to `/opt/backups/nexustale/repos/`; 7-day retention
- [x] **P1** Structured log capture: `x-logging` YAML anchor in `docker-compose.deploy.yml` applies `json-file` driver with `max-size=50m` / `max-file=5` to all five services
- [ ] **P1** Uptime monitor on `GET /healthz` — **manual step**: sign up at uptimerobot.com (free tier), add HTTP monitor for `https://<domain>/healthz`, set alert contact to `NEXUSTALE_ALERT_EMAIL`, threshold = 2 consecutive failures
- [x] **P1** Disk usage alert — cron every 4 h checks `df /`; if ≥70% writes to syslog via `logger` and emails `nexustale_alert_email`; wired in Ansible
- [ ] **P2** AI usage dashboard for admin — `ai_usage` table already queryable; a simple `psql` query or Grafana panel is sufficient for alpha

### Pre-launch code checklist

- [x] **P0** Git-first architecture migration (Steps 1–4) complete — `scenes.content` dropped (migration 029); scene prose lives in git working tree; Postgres is metadata-only; export, BuildContext, Chronicle, and agent tools all read/write from the working tree; Step 5 (wiki JSON files) explicitly deferred (N+1 concern does not exist in current implementation)
- [ ] All **P0** code review items resolved
- [ ] All **P0** security review items resolved
- [ ] `govulncheck` and `npm audit` clean (P1 items)
- [ ] `npx tsc --noEmit` and `go build ./...` clean on the release commit
- [ ] Smoke test the full user loop on alpha env: register → guide wizard → write scene → Chronicle → wiki entity → export Markdown → invite collaborator → open MR

### Alpha UX / onboarding

- [ ] Error messages are writer-facing — no Go stack traces or raw DB errors leak through (`apperror` messages audited)
- [ ] Guide wizard surfaced prominently on first project (existing CTA on ProjectHome)
- [ ] "Give feedback" link visible in the app (Settings or TopBar) — Discord/email/form
- [ ] Invite email template with direct link to `/invites/:token`
- [ ] Known limitations doc (one-pager) shared with alpha users: collaboration is async (no live co-editing), no mobile support, AI requires your own API key

### Rollback plan

- Docker images are tagged by git SHA (`:{sha}`) — rollback = re-run Ansible with previous SHA tag
- DB migration `.down.sql` files exist for all 29 migrations; test rollback from 029 → 028 on a staging DB before launch (note: 029 down restores `scenes.content` with `DEFAULT ''` — content cannot be recovered from rollback alone; use a DB backup)
- Alpha user data export: any user can export their full manuscript as Markdown at any time (no lock-in)

### Alpha → beta graduation criteria

A milestone, not a date. Graduate when all of the following are true:

- [ ] ≥10 writers have completed the novel guide wizard (premise → first scene)
- [ ] ≥3 collaborative projects have had at least one merge request opened and resolved
- [ ] Core user loop (register → write → export) completed without Claude Code intervention by ≥5 non-dev users
- [ ] No P0 bugs open for >48 hours sustained over a 2-week window
- [ ] Feedback triaged — Phase D backlog updated with top writer requests

---

## Phase C7 — Craft depth (continued)

### C7.0 — Auto-detection backend + mentions panel ✅ (2026-05-05)

- [x] **`[Medium]`** Migration 035 — `scene_entity_mentions(id, scene_id, entity_id, project_id, branch_name, match_text, suppressed)` + `projects.auto_tag_enabled BOOLEAN DEFAULT TRUE`
- [x] **`[Medium]`** `internal/wiki/tagger.go` — debounced (5s) whole-word case-insensitive detection; respects `suppressed` (never re-adds author removals) and `auto_tag_enabled`; `IndexSceneMentions` satisfies `project.MentionNotifier`; wired via `projectService.WithMentionNotifier(wikiService)`
- [x] **`[Light]`** Routes — `GET /projects/:id/scenes/:sid/mentions?branch=`, `DELETE .../mentions/:mid` (suppress), `DELETE .../mentions` (suppress all); `GET /wiki/entities/:eid/appearances?branch=`
- [x] **`[Light]`** `BuildContext` section 5 reads from `scene_entity_mentions` (pre-computed, respects suppressed); falls back to `@[entity]` regex when no mentions indexed yet
- [x] **`[Medium]`** Frontend — `MentionsBar.tsx` chip row below ScribeEditor; type-colored chips; click navigates to wiki entry; right-click → "Remove tag"; "Clear all" button; `auto_tag_enabled` in `ProjectResponse` + OpenAPI spec

---

## Phase C8 — App administration

A protected admin area for monitoring and managing NexusTale during alpha. Gated on `RequireRole(RoleAdmin)` — the role enum and `RequireRole` middleware already exist. No public UI.

Infrastructure already in place:
- `users.role` column with `UserRoleAdmin` value (in DB since migration 001)
- `users.plan` column (`free` / `writer` / `studio`) since migration 022
- `RequireRole` middleware in `internal/auth/middleware.go`
- `ai_usage` table (queryable for system-wide stats)
- `waitlist` table + `internal/waitlist` package

### C8.0 — Admin backend

- [ ] **`[Light]`** `internal/admin` package — `Service` with queries; `Handler` with `RequireRole(RoleAdmin)` guard; `GET /admin/users` (paginated list: id, email, display_name, role, plan, created_at, project_count); `PATCH /admin/users/:uid` (set role and/or plan)
- [ ] **`[Light]`** `GET /admin/stats` — aggregate: total users, total projects, total scenes, total AI calls, total tokens used, total cost (USD); all from existing tables with simple COUNT/SUM queries
- [ ] **`[Light]`** `GET /admin/ai-usage` — per-user AI usage summary (user_id, email, total_tokens, total_cost, call_count) for the last 30 days; ordered by total_tokens DESC
- [ ] **`[Light]`** `GET /admin/waitlist` + `PATCH /admin/waitlist/:wid` — list waitlist entries with status; mark approved/rejected (thin wrapper over existing `internal/waitlist` service)
- [ ] **`[Light]`** sqlc queries for the above (new `pkg/db/queries/admin.sql`)
- [ ] **`[Light]`** Promote-to-admin script / one-off: `POST /admin/users/:uid` with `{role: "admin"}` (or a simple `psql` command documented in the ops runbook)

### C8.1 — Admin frontend

- [ ] **`[Medium]`** `/admin` route — guarded in React by `role === 'admin'` from the JWT claims; redirect to `/dashboard` if not admin
- [ ] **`[Medium]`** Admin Dashboard page — stat cards (users, projects, AI calls, tokens, cost); sections for User Management and Waitlist
- [ ] **`[Light]`** User table — paginated; role badge; plan badge; "Set Plan" dropdown (free/writer/studio); "Set Role" dropdown (author/admin); changes call `PATCH /admin/users/:uid`
- [ ] **`[Light]`** AI Usage table — top-N users by token spend (last 30 days); useful for spotting abuse before billing is wired
- [ ] **`[Light]`** Waitlist table — email, joined_at, status; Approve / Reject buttons
- [ ] **`[Light]`** Role surfaced in JWT — `role` is already in `Claims`; expose it from `GET /users/me` response so the frontend can gate the admin link in TopBar settings

---

## Phase D — Premium / advanced

- **Monetization** — `users.plan` column already added (migration 022: free/writer/studio tiers); plan-gated invite limits in `InviteCollaborator` (`TODO(monetization)` marker in service.go); billing integration (Stripe), upgrade flow, and feature gates are Phase D work
- Map builder; image generation pipelines for wiki entities
- Scrivener / Fountain export; advanced Git branching UX
- Series-level continuity management
- Multi-region, scale-out collaboration tuning
- **OpenAPI catch-up** — bring `docs/openapi.yaml` current with all B1–C routes; regenerate `api-types.ts`; restore codegen for newer endpoints (schedule before C3)
- **Customizable workspaces** — per-user, per-project saved panel layouts (which panels are open, their widths, which scene/chapter is active); named workspace presets ("drafting", "research", "editing") switchable from the TopBar; stored in `user_workspaces` table (JSONB layout blob); synced across sessions so the editor opens exactly where you left off
- **Object storage migration (MinIO → S3-compatible)** — The self-hosted MinIO server is AGPL v3.0 licensed; running it as part of a commercial SaaS without a commercial license creates legal exposure. The `minio-go` client SDK (Apache 2.0) is not the issue — only the server binary. Plan: replace `pkg/storage/storage.go` with `aws-sdk-go-v2/service/s3` (~100 line rewrite; interface unchanged: `PutObject`, `PresignedGetURL`, `DeleteObject`); target **Cloudflare R2** for alpha/beta (free tier: 10 GB storage, no egress fees, S3-compatible) or Backblaze B2 as an alternative; dev Docker Compose keeps MinIO locally since `aws-sdk-go-v2` speaks the same S3 API. Config env vars (`NEXUSTALE_MINIO_*`) can be renamed to `NEXUSTALE_S3_*` at the same time. No DB migrations needed. Must be done before any paid tier goes live.

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

*Last updated 2026-05-05: C7.0 scene entity mentions complete (migration 035, `IndexSceneMentions`, appearances endpoint, WikiHub "Appears In" panel). Phase C8 admin section planned. Phase C+ environment checklist and UX/onboarding items remain before first alpha invite.*
