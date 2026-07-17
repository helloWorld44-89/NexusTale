# NexusTale roadmap

Sci-fi/fantasy novel-writing tool: structured manuscripts (projects → chapters → scenes), worldbuilding, AI-assisted drafting, export, and (eventually) collaboration.

**Companion docs:** [CLAUDE.md](./CLAUDE.md) (how to work in this repo), [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) (system design), [Makefile](./Makefile) (dev commands).

---

## Current state (snapshot) — 2026-07-14

| Area | Status |
|------|--------|
| **API shell** | Go 1.25 + Gin; `/healthz`; `/api/v1/auth/*`; `/api/v1/projects/*` (CRUD + acts + chapters + scenes), JWT + refresh tokens; rate-limited (auth: 10/min per IP, AI: 30/min per user) |
| **Database** | PostgreSQL migrations **(037)** + **sqlc** (`pkg/db/queries` → `pkg/db/sqlcgen`); pgvector extension for semantic RAG |
| **Manuscript hierarchy** | **Project → Act → Chapter → Scene**; git-first (scenes.content dropped migration 029 — all prose in git working tree); full CRUD + integration tests + Bruno |
| **Git per project** | Non-bare repos on disk; full Chronicle/Lore/Echo/Diverge/TravelTo/Canonize API; fast-forward merge; Paradox detection; `repoPathForUser` for collaborator clones |
| **Wiki** | `wiki_entities` (with image upload), `wiki_relationships`, `wiki_magic_rules` (structured attrs), `wiki_timeline_events` (anchor chain, DFS depth-capped at 50); autolink + graph endpoints; scene entity auto-tagging (migration 035) + suppress; **rename cascade** (preview/confirm with word-level diff, case-preserving replace, Chronicle commit) |
| **AI pipeline** | 7 providers: Anthropic, OpenAI, OpenRouter (dual-model: quality + background), Gemini, Groq, DeepSeek, Ollama; **task-tier routing** (haiku/mini for summarize, sonnet/gpt-4o for beat/continue/chat); beat + continue (behavioral constraints, NarrativePhase dropdown), chat (brainstorm/editorial/lore modes), workshop (agent tool use, 25 rounds), summarize (EVENTS/CHANGES/PRESSURE format + retry), context pins, AI Bible, writing styles |
| **AI context** | `BuildContext` 8-section layout; **semantic RAG** via pgvector (migration 037) — `pkg/embedding/` (OpenAI + Ollama); brute-force window fallback when no embeddings; chapter summary cap (anchor + recent 5); entity type caps (5 chars, 3 locs, 2 other); chat injects summary not full scene text; 24k/32k char budgets |
| **AI quality (C9)** | Prose fingerprinting (migration 036, auto-refreshes every 3 saves, `## Author's prose style` in Beat/Continue); `isValidSummary()` retry loop; chapter position header in summarize input; scene separators; agent PLANNING MANDATE + tool intent routing |
| **Export** | Markdown (sync zip) + EPUB + DOCX (async jobs, MinIO, presigned URL) |
| **Import** | `POST /projects/import` (preview) + `/import/confirm` — parses `.md`, `.txt`, `.docx` into act/chapter/scene tree; preview UI with editable titles, merge/remove scenes |
| **Novel guide** | 5-step wizard (Premise → Characters → World → Outline → First Scene); story structure templates (12 seeded + scoring matrix) |
| **Collaboration** | Roles + invite system; per-collaborator git clones; branch-prefix enforcement; merge requests (diff/resolve/merge); prose diff viewer (word-level, paginated); reviewer annotations; notifications (60s polling) |
| **Admin** | `GET /admin/stats`, `/admin/users` (paginated, role+plan dropdowns), `/admin/ai-usage`; `RequireRole(RoleAdmin)`; `/admin` React page; "Admin Panel" link in Settings for admins |
| **Frontend** | React 18 + Vite + TypeScript + Tailwind; full editor (TipTap/ProseMirror), entity mention highlighting + hover cards, wiki hub (entities/timeline/graph/research/story threads), Nexus chat (modes), Workshop (agent), Beat/Continue toolbar (NarrativePhase dropdown), Import manuscript page, rename cascade modal, Dashboard Import button |
| **Settings** | All 7 AI providers; per-provider model selection; OpenRouter dual-model (quality + background); Ollama model picker; appearance; account deletion; Admin Panel link |
| **Security** | TLS via certbot/Ansible; CORS locked; branch name validation; file upload magic-byte check; `RequireProjectAccess` + `RequireChapterAccess` on all project routes |
| **Deployment** | Self-hosted; Docker Compose (`pgvector/pgvector:pg16`); GitHub Actions → GHCR; Ansible deploy; daily pg_dump + git-repos backup; disk usage alert |

---

## Core features (product pillars)

1. **Accounts & access** — Register/login, JWT access + refresh, roles, plan. *Done.* Admin panel for role/plan management.
2. **Manuscript structure** — Projects, chapters, scenes, ordering; Git-first (prose in working tree). *Done.* Import from .md/.txt/.docx.
3. **World wiki** — Entities, relationships graph, timeline (anchor-relative, DFS depth-capped), magic rules, research notes, story threads. *Done.* Entity rename cascade patches prose across manuscript.
4. **AI-assisted writing** — 7 providers, task-tier model routing, semantic RAG, prose fingerprinting, beat/continue/chat/workshop/agent tools, C9-P1–P7 quality improvements. *Done.*
5. **Export** — Markdown, EPUB, DOCX. *Done.* Scrivener/Fountain/PDF deferred to Phase D.
6. **Collaboration** — Git-backed async: per-collaborator clones, invite system, merge requests, prose diff + conflict resolution, reviewer annotations, notifications. *Done.*
7. **Assets** — Wiki image upload via MinIO (presigned URLs). Map builder / image generation deferred to Phase D.
8. **Writer UI** — Full React SPA: editor, wiki, AI panels, export, import, admin, guide wizard, collaboration. *Done.*

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
- [x] **P2** Timeline anchor DFS cycle detection: `maxAnchorDepth = 50` added to `ResolveEvents` — depth counter passed through recursive `resolve(id, depth+1)` call; returns error when exceeded

**Access control**
- [x] **P0** Non-owner cannot approve, reject, or merge MRs — `ResolveMergeRequest` in `merge/service.go` checks `p.OwnerID != callerID` → 403 (verified correct)
- [x] **P0** Non-owner cannot resolve annotations — `Resolve` in `annotations/service.go` now checks `p.OwnerID != resolverID` → 403
- [x] **P1** Collaborator can only read/write their own clone path — all `repoPathForUser` callers pass `auth.GetUserID(c)` / `claims.UserID` from verified JWT; no user-supplied ID accepted; audited clean
- [x] **P1** `DELETE /users/me` cascade: `ListUserWikiImageKeys`, `ListUserExportMinioKeys`, `ListUserCollaboratorClonePaths` sqlc queries added; `auth.Service.WithStorage` wired from `main.go`; MinIO objects + clone dirs cleaned best-effort after DB cascade
- [x] **P2** Rate limiting on `POST /auth/login` and `POST /auth/register` — `ratelimit.ByIP(10, time.Minute)` on `authGroup` in `main.go` (already wired, checkbox missed)
- [x] **P2** Rate limiting on AI endpoints — `ratelimit.ByUser(30, time.Minute)` on `aiGroup` in `main.go` (already wired, checkbox missed)

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
- [x] **P2** `numericToFloat64()`: verified via `TestNumericToFloat64` (`internal/ai/service_test.go`) — nil interface, wrong type, and invalid `pgtype.Numeric` all safely return `0`; SQL always wraps aggregates in `COALESCE(SUM(...), 0)` so the driver never actually hands back an untyped nil, but the fallback is now locked in by a test either way
- [x] **P2** DB pool: audited every `context.Background()` in `internal/`/`pkg/` — all occurrences are in genuinely detached goroutines (debounced summarizer/tagger jobs, async export workers, fire-and-forget usage recording, one-time bucket init); none found threading through a live Gin request path

**Frontend**
- [x] **P0** React error boundaries — `ErrorBoundary.tsx` class component created; all major panels in `Editor.tsx` wrapped (Nexus, Context, Workshop, Chronicle, Wiki, Annotations, scene editor, project explorer); shows error message + "Try again" reset button
- [x] **P1** `ScribeEditor` navigate-away race: `pendingSaveRef` stores current save callback; flushed via `useEffect` cleanup on `selectedSceneId` change and via `beforeunload` handler
- [x] **P1** SSE `EventSource` cleanup: `useEffect(() => () => abortRef.current?.abort(), [])` added to `ChatBar`, `WorkshopPanel`, and `BeatInput` — aborts in-flight fetch streams on unmount
- [x] **P1** `ProseDiffViewer`: 20-per-page pagination with Prev/Next controls replaces full synchronous render of all `SceneDiffCard` components
- [x] **P2** `localStorage` for access token — `authStore.ts` `partialize` no longer persists `accessToken`; it now lives only in the Zustand store's in-memory state. `refreshToken` remains persisted so login survives a reload; `onRehydrateStorage` calls `silentRefresh()` to mint a fresh access token on load instead of restoring one from disk
- [x] **P2** `api.ts`: added `apiUrl(path)` helper — throws if `path` isn't a same-origin-relative string (`startsWith('/')`, not `//`); all 7 raw `fetch()` call sites plus the shared `request()` helper now route through it before the `Authorization` header is attached

**API contract**
- [x] **P1** OpenAPI spec (`docs/openapi.yaml`) brought current — 49 operations added across AI, export, guide, prompts, research, collaboration, merge requests, notifications, and annotations; spec grew from 1907 → 3335 lines; `api-types.ts` regenerated clean (`npm run gen:api`); `npx tsc --noEmit` passes
- [x] **P2** API versioning policy documented directly in `docs/openapi.yaml` info.description — breaking changes ship under a new `/api/v2` prefix once beta clients exist (with a deprecation window for `v1`); additive/non-breaking changes continue landing in-place under `v1`

---

## Alpha / self-hosted deployment ✅ complete

NexusTale is self-hosted. AWS deployment removed (2026-07-14). All features deployed via Docker Compose on a self-managed server. Public registration reopened (2026-07-16) — `NEXUSTALE_SERVER_REGISTRATIONOPEN=true` in `deploy-dev.yml` + compose default; waitlist form removed from Landing (backend `internal/waitlist` package + admin view kept, unused). MinIO stays as the object storage backend for now (AGPL concern only applies once a paid tier exists — see Phase D S3 migration note).

**Public self-hosting path added (2026-07-16):** `infra/docker/docker-compose.selfhost.yml` + `.env.selfhost.example` — pulls prebuilt public GHCR images, no build step, no bundled TLS (self-hosters front it with their own reverse proxy: Nginx Proxy Manager, Caddy, Traefik, Cloudflare Tunnel, etc., same pattern as other self-hosted docker-compose apps). Only 5 required env vars (JWT secret, encryption key, DB password, MinIO access/secret keys); everything else has a working default. `NEXUSTALE_SERVER_MODE` defaults to `debug` so a fresh install never crash-loops on `ValidateProd()`'s strict release-mode checks. `infra/ansible/*` is unchanged and remains the path for the maintainer's own dev VM auto-deploy — the two paths are intentionally separate.

**Versioning introduced (2026-07-16):** semver git tags (`vX.Y.Z`) trigger `.github/workflows/release.yml`, which builds and pushes `:vX.Y.Z` + `:latest` images — this is the tag self-hosters pin `NEXUSTALE_VERSION` to for reproducible upgrades. Distinct from the existing floating `:alpha` (master push) and `:dev` (dev push) tags, which remain for internal testing only. First tagged release is `v0.3.0` — the product is still alpha (per Landing page framing and unmet beta graduation criteria above), so 1.0.0 is reserved for when that's no longer true.

**Manual step still required:** the `nexustale/api` and `nexustale/frontend` GHCR packages are currently private and must be flipped to public through the GitHub web UI (package Settings → Danger Zone → Change visibility) — the REST API returns 404 for visibility changes on packages linked to a repository, so this can't be scripted.

**Beta graduation criteria** (milestone, not a date):
- [ ] ≥10 writers completed the novel guide wizard (premise → first scene)
- [ ] ≥3 collaborative projects had at least one merge request opened and resolved
- [ ] Core user loop (register → write → export) completed without developer assistance by ≥5 non-dev users
- [ ] No P0 bugs open >48 hours sustained over a 2-week window
- [ ] Feedback triaged — Phase D backlog updated with top writer requests

---

## Phase C6 — Craft depth (first pass)

### C6.1 — Magic rule attributes ✅ (migration 031)
- [x] **`[Light]`** Migration 031 — structured attribute columns on `wiki_magic_rules` (cost, activation, range, limitation, discovery_state); UI fields in magic rule editor

### C6.3 — Scene attributes ✅ (migration 032)
- [x] **`[Light]`** Migration 032 — `scene_attributes` table (role, goal, conflict, pov_character_id, tense as structured columns rather than free text); `SceneContextAttrs` + `ParseSceneContextAttrs` exported from `context.go`; resolved scene attrs injected as `## This scene` block in beat and continue system prompts via `buildSceneDirective`

### C6.4 — Story threads ✅ (migration 033)
- [x] **`[Medium]`** Migration 033 — `story_threads(id, project_id, title, description, status open/closed/resolved, created_at)`; `internal/storythreads` service + handler; CRUD routes under `/projects/:id/story-threads`; `ListOpenThreadsByProject` JOIN query; `buildOpenThreadsContext` injects open threads as `## Open story threads` section 8 in `BuildContext`; Story Threads tab in WikiHub

### C6.5 — Revision pass system ✅ (migration 034)
- [x] **`[Medium]`** Migration 034 — `projects.phase TEXT DEFAULT 'drafting'`; `GET/PUT /projects/:id/phase`; 5 phases: drafting / outlining / revision / language_pass / done; `workshopSystemForPhase()` in `workshop_handler.go` — 4 phase-specific system prompts injected before user messages; `phase` field in `ProjectResponse` + OpenAPI spec; `api.phase.get/set` in api.ts; TopBar phase badge (clickable modal with phase picker); `projectPhase` state in `Editor.tsx`; BeatInput "Focus: prose quality" hint in language_pass; WorkshopPanel "Start [Phase] Checklist" pre-fills a session prompt

### C6.6 — BuildContext + prompt engineering audit ✅
- [x] **`[Heavy]`** `context.go` rewritten with 8-section layout — `## Project` / `## Story structure` / `## Magic systems` (new sec 3, Limitations-first, hard cap 5) / `## Entities in this scene` / `## Chapter context` / `## Pinned context` / `## This scene` / `## Open story threads` (new sec 8)
- [x] **`[Medium]`** `buildEntityContextLine` dispatches by entity type — character: motivation|arc|capability; location: description+history; `arcPositionHint` (early/mid/late based on chapter index in sequence)
- [x] **`[Light]`** `contextBudgetWarnChars = 20_000` warn log; `truncateRunes` utility; `resolvedScene` extended with role/goal/conflict

---

## Phase C7 — Craft depth (continued)

### C7.0 — Auto-detection backend + mentions panel ✅ (2026-05-05)

- [x] **`[Medium]`** Migration 035 — `scene_entity_mentions(id, scene_id, entity_id, project_id, branch_name, match_text, suppressed)` + `projects.auto_tag_enabled BOOLEAN DEFAULT TRUE`
- [x] **`[Medium]`** `internal/wiki/tagger.go` — debounced (5s) whole-word case-insensitive detection; respects `suppressed` (never re-adds author removals) and `auto_tag_enabled`; `IndexSceneMentions` satisfies `project.MentionNotifier`; wired via `projectService.WithMentionNotifier(wikiService)`
- [x] **`[Light]`** Routes — `GET /projects/:id/scenes/:sid/mentions?branch=`, `DELETE .../mentions/:mid` (suppress), `DELETE .../mentions` (suppress all); `GET /wiki/entities/:eid/appearances?branch=`
- [x] **`[Light]`** `BuildContext` section 5 reads from `scene_entity_mentions` (pre-computed, respects suppressed); falls back to `@[entity]` regex when no mentions indexed yet
- [x] **`[Medium]`** Frontend — `MentionsBar.tsx` chip row below ScribeEditor; type-colored chips; click navigates to wiki entry; right-click → "Remove tag"; "Clear all" button; `auto_tag_enabled` in `ProjectResponse` + OpenAPI spec

### C7.1 — Inline entity highlighting + hover popup ✅ (2026-05-05)

- [x] **`[Heavy]`** ScribeEditor migrated from `<textarea>` to **TipTap v3** (StarterKit); `plainToHTML` + `editorGetText` round-trip (hardBreak → `\n`) preserve plain-text backend storage; `buildCharToPos` / `buildPosToChar` for offset-based annotation compat
- [x] **`[Heavy]`** `EntityMentionExtension.ts` — ProseMirror Plugin; `DecorationSet` rebuilt on every transaction; type-colored dotted underlines (`character` cyan, `location` amber, `faction` violet, `item` green, `concept` slate, `lore` rose) via inline `Decoration.inline` with class attributes; updated via `tr.setMeta` when `MentionsBar` mention list changes
- [x] **`[Medium]`** `EntityHoverCard.tsx` — 400 ms show delay, 150 ms hide delay; fetches entity on mount; portrait thumbnail + summary + "Open in Wiki →" deep link; dismisses on mouse-leave
- [x] **`[Light]`** Right-click on decorated span → suppress context menu (same as MentionsBar chip); copy/cut/paste preserved in entity mention right-click menu
- [x] **`[Light]`** `MentionsBar` updated to controlled mode — accepts `mentions` / `onSuppressOne` / `onSuppressAll` props; driven by parent state in `ScribeEditor`

---

## Phase C8 — App administration

A protected admin area for monitoring and managing NexusTale during alpha. Gated on `RequireRole(RoleAdmin)` — the role enum and `RequireRole` middleware already exist. No public UI.

Infrastructure already in place:
- `users.role` column with `UserRoleAdmin` value (in DB since migration 001)
- `users.plan` column (`free` / `writer` / `studio`) since migration 022
- `RequireRole` middleware in `internal/auth/middleware.go`
- `ai_usage` table (queryable for system-wide stats)
- `waitlist` table + `internal/waitlist` package

### C8.0 — Admin backend ✅ complete (2026-07-14)

- [x] **`[Light]`** `internal/admin` package — `Handler` with `RequireRole(RoleAdmin)` guard on group; `GET /admin/users` (paginated, limit/offset, includes project_count); `PATCH /admin/users/:uid` (role + plan validation + update); `GET /admin/stats`; `GET /admin/ai-usage`
- [x] **`[Light]`** `pkg/db/queries/admin.sql` — `AdminListUsers`, `AdminGetStats`, `AdminListAIUsage`, `AdminSetUserRole`, `AdminSetUserPlan`; sqlc regenerated
- [x] **`[Light]`** Promote-to-admin: `PATCH /admin/users/:uid` with `{"role":"admin"}`; or `psql -c "UPDATE users SET role='admin' WHERE email='...'"` documented in ops runbook

### C8.1 — Admin frontend ✅ complete (2026-07-14)

- [x] **`[Medium]`** `/admin` route — `Navigate` to `/dashboard` when `user.role !== 'admin'`; wrapped in `ProtectedRoute`
- [x] **`[Medium]`** Admin Dashboard page — 6 stat cards (users, projects, scenes, AI calls, tokens, cost); tabbed layout (Users · AI Usage 30d); paginated user table with inline role/plan dropdowns; AI usage table ordered by token spend
- [x] **`[Light]`** `plan` added to `UserResponse` in Go + OpenAPI spec + `api-types.ts` regenerated; `AdminStats`, `AdminUser`, `AdminAIUsageRow` inline types in `api.ts`; `api.admin.*` methods added
- [x] **`[Light]`** "Admin Panel" link shown in Settings header when `user.role === 'admin'`
- [x] **`[Light]`** `pgvector-go` v0.4.0 added as dependency (required by sqlc v1.31.1 auto-detecting vector columns from migration 037); `research/service.go` updated to use dedicated row types produced by new sqlc

---

## Phase C9 — AI Quality Improvements

> Driven by a three-model consensus review (o3, GPT-4o, Gemini 2.5 Pro) of the full AI pipeline that scored the system 2.9/5 and identified five root issues: brute-force RAG, declarative-not-behavioral craft prompting, broken summarization architecture, no task-specific model routing, agent has no planning phase. All seven phases below (P1–P7) address those findings and are complete as of 2026-07-14.

Scale key: **Light** · **Medium** · **Heavy** · **Heavier** · **Heaviest**

---

### C9-P1 — Prompt Text Fixes ✅ complete (2026-07-14)

**Expected gain:** Immediate Beat/Continue output quality improvement; eliminates repetition artifacts and meta-narration.

- [x] **`[Light]`** **1.1 Beat — behavioral craft constraints + user-turn framing** — replaced "Match the author's tone. Show, don't tell." with explicit DO-NOT rules (no repeating scene tail, no meta-narration, no forward summary, emotion through sensation not abstraction, avoid adverbs/suddenly/realized); paragraph count → "approximately 2–3 — expand or compress as needed"; user turn wrapped: `Expand the following story beat into prose, continuing directly from the scene ending above: <BEAT TEXT>` — `service.go:beatSystemPrompt()` + `StreamComplete`
- [x] **`[Light]`** **1.2 Continue — narrative phase awareness + user-turn framing** — replaced "Continue the story naturally from where it left off." with DO-NOT/DO constraint block; added `NarrativePhase` field to `CompleteRequest` and `completeRequest` wire type; `narrativePhaseDirective()` maps 6 phases to focus directives; `narrative_phase` dropdown in Continue toolbar (BeatInput.tsx); `narrative_phase` field in `api.ts:streamComplete`
- [x] **`[Light]`** **1.3 Workshop — fix prompt order + increase digest limit** — `workshopSystemPrompt()` now returns `base + "\n\n" + directive` (identity first); `workshopDigestMaxRunes` raised 200 → 600
- [x] **`[Light]`** **1.4 Workshop phases — PRIMARY / SECONDARY / FAILURE format** — all four phase directives rewritten with `PRIMARY OBJECTIVE / SECONDARY OBJECTIVE / FAILURE CONDITIONS` structure; removed generic encouragement
- [x] **`[Light]`** **1.5 Agent — planning mandate + append-vs-replace guidance** — PLANNING MANDATE block (state plan → list IDs → then call tools) + TOOL SELECTION RULES (append for new content, replace ONLY on explicit writer request, never shorten) prepended to agent system prompt in `StreamChatWithTools`

---

### C9-P2 — Context Efficiency ✅ complete (2026-07-14, items 2.1–2.5)

**Expected gain:** 40–60% reduction in context tokens for mid-to-large projects; faster calls; lower cost.

- [x] **`[Medium]`** **2.1 Hard context budget with priority drop policy** — `maxContextCharsGeneration = 24_000` / `maxContextCharsChat = 32_000`; `BuildContext` refactored to collect an `always` block and a `distant` (droppable) block; distant chapter summaries (beyond anchor + recent 5) are appended only when the budget allows; warn log if still over after drop — `context.go`
- [x] **`[Light]`** **2.2 Cap chapter summaries** — inject only first `storySoFarAnchorCount=1` chapter (story anchor) + last `storySoFarRecentWindow=5` before current; a 30-chapter novel: 30 → ~7 summaries; distant middle chapters go into the droppable block (2.1) — `context.go:BuildContext`
- [x] **`[Light]`** **2.3 Cap entity mentions by type** — `capAndBuildEntityLines()` applies caps: characters max 5, locations max 3, other combined max 2; entities beyond caps are silently dropped in entity query order — `context.go:buildEntityContext()` + `capAndBuildEntityLines()`
- [x] **`[Medium]`** **2.4 Chat/Workshop — inject scene summary not full text** — `buildCurrentSceneContext()` new helper; when `currentSceneID != uuid.Nil` in chat/workshop mode, injects AI chapter summary as `*(Chapter summary)*`; falls back to first 1,600 runes of scene content; beat/continue still suppress section 6 entirely via `uuid.Nil` caller pattern — `context.go`
- [x] **`[Light]`** **2.5 Chat — digest compression** — `StreamChat` now calls `applyWorkshopHistoryWindow` (600-rune digest limit) instead of `applyHistoryWindow` (silent truncation); older turns preserved as summary rather than discarded — `service.go:StreamChat()`
- [x] **`[Medium]`** **2.6 Agent — pass only relevant tools per intent** — `selectToolsForIntent()` in `tools.go` keyword-classifies the last user message; sends wiki-only (5 tools: list_project_structure + 4 wiki) or write-only (6 tools: list_project_structure + 5 manuscript) when intent is unambiguous; falls back to full 10-tool set when mixed; `StreamChatWithTools` calls `selectToolsForIntent(historyMsgs)` per round

---

### C9-P3 — Summarization Architecture Overhaul ✅ complete (2026-07-14)

**Expected gain:** Better `## Story so far` quality → better Beat/Continue output; structured output feeds directly into story threads system.

- [x] **`[Medium]`** **3.1 Multi-layer summary format** — `summarizeSystemPrompt()` rewritten with `EVENTS / CHANGES / PRESSURE` three-section output format; dynamic token cap `min(sceneCount × 120, 350)` in `summarizeWithTokens()` (background path); `isValidSummary()` added to filter over-narration
- [x] **`[Light]`** **3.2 Chapter title + position** — `buildChapterPositionHeader()` prepends `Chapter N of TOTAL: "Title"` to summarize input; uses `ListChaptersByProject` + `GetChapter` (no new query needed)
- [x] **`[Light]`** **3.3 Scene separators** — concatenation changed from `"\n\n"` to `"\n\n---\n\n"` + `## Scene: <title>` label; applied in both `regenerateSummary()` (background) and `RegenerateChapterSummary()` (manual)
- [x] **`[Light]`** **3.4 Retry + validation** — `isValidSummary()` rejects strings <30 chars or starting with "in this chapter"/"this chapter"/"the chapter"; `regenerateSummary()` retries up to 2× before giving up; bad output logged with prefix sample

---

### C9-P4 — Task-Specific Model Routing ✅ complete (2026-07-14)

**Expected gain:** 40–70% cost reduction on background tasks; better prose on generation tasks without writer having to manually select a model.

- [x] **`[Medium]`** **4.1 Task-tier model selection** — `taskTier` enum (`tierBackground | tierAnalysis | tierCreative`) in `service.go`; `getAdapterForTier()` wraps `getAdapter` with model override: background → adapter defaults (haiku/gpt-4o-mini), analysis (chat/workshop/agent) → `claude-sonnet-4-6` / `gpt-4o`, creative (beat/continue) → same; writer's explicit `Provider` field still bypasses tier routing; `StreamComplete` → tierCreative, `StreamChat` + `StreamChatWithTools` → tierAnalysis, `Summarize` → tierBackground (unchanged)

---

### C9-P5 — Style Fingerprinting ✅ complete (2026-07-14)

**Expected gain:** Beat/Continue output that matches the writer's prose DNA without manual style preset configuration.

- [x] **`[Heavy]`** **5.1 Prose fingerprint extraction** — migration 036 `projects.prose_fingerprint JSONB`; `internal/ai/fingerprint.go`: `ExtractProseFingerprint()` computes avg sentence length, avg paragraph length, dialogue ratio, adverb density, sentence variance from all project scenes; `RefreshProseFingerpint()` runs in background; `NotifySceneSaved()` on `SummaryNotifier` interface triggers refresh every 3 saves; `FingerprintContextBlock()` renders `## Author's prose style` block; injected into both Beat and Continue system prompts in `StreamComplete`
- [x] **`[Light]`** **5.2 Fingerprint-aware paragraph count** — `beatSystemPrompt()` accepts `*ProseFingerprint`; paragraph count hint: `AvgParagraphLength ≤2` → "3–5 short paragraphs", `≥5` → "1–2 longer paragraphs", else "approximately 2–3"

---

### C9-P6 — Regular Chat Mode Specialization ✅ complete (2026-07-14)

**Expected gain:** Nexus chat gives sharper answers when the writer's intent is known; reduces generic responses.

- [x] **`[Medium]`** **6.1 Chat modes** — `mode` field added to `chatRequest` wire type and `ChatRequest`; `nexusChatSystemPrompt(mode)` function in `service.go` returns tailored identity: `brainstorm` (2–3 directions, generative not evaluative), `editorial` (structural editor — cause/effect, pacing, promises-and-payoffs), `lore` (wiki oracle — answers only from existing project data, says so when not found), default (general Nexus); mode pills rendered in ChatBar input footer with dynamic placeholder text; `chatMode` passed to `api.ai.streamChat()`

---

### C9-P7 — Semantic Retrieval / Embedding-Based RAG ✅ complete (2026-07-14)

**Expected gain:** 60–80% reduction in context tokens; project-size-independent context quality; eliminates attention dilution from large brute-force injection.

- [x] **`[Heavy]`** **7.1 Embedding targets** — chapter summaries (embedded immediately after `regenerateSummary()`), wiki entities + research notes (via `BackgroundReembed` worker, 10-min interval, catches `embedding IS NULL` or stale rows); scene text, magic rules, story structure not embedded
- [x] **`[Heavy]`** **7.2 pgvector storage** — migration 037: `CREATE EXTENSION IF NOT EXISTS vector`; `vector(768)` columns on `chapter_summaries`, `wiki_entities`, `research_notes`; `embedding_updated_at TIMESTAMPTZ`; IVFFlat indexes (lists=10); postgres image switched to `pgvector/pgvector:pg16` in both compose files; dimension 768 matches `text-embedding-3-small` (with dimensions=768), `nomic-embed-text` (Ollama), `text-embedding-004` (Gemini)
- [x] **`[Heavier]`** **7.3 Retrieval at call time** — `BuildContext` checks `HasEmbeddings()` first; semantic path: `embedding <=> $query_vec::vector` via raw pgx (bypasses sqlc — vector operator unsupported); always injects first chapter (anchor) + current chapter + top-5 semantically relevant summaries from `buildSemanticStorySoFar()`; falls back transparently to brute-force window when no embeddings exist
- [x] **`[Heavy]`** **7.4 Embedding pipeline** — `pkg/embedding/` package: `Embedder` interface + `OpenAIEmbedder` + `OllamaEmbedder`; `EmbedStore` in `embeddings.go` (upsert + search + background worker); `WithEmbedding(pool, provider)` on `ai.Service`; `NEXUSTALE_AI_EMBEDOPENAIKEY` env var enables OpenAI embeddings; Ollama fallback when only Ollama URL is configured; `BackgroundReembed` started as goroutine in `main.go`

---

## Phase D — Premium / advanced

- **AI-assisted map builder** — see [D-Map](#d-map--ai-assisted-map-builder) below
- **AI-assisted entity artwork** — see [D-Portraits](#d-portraits--ai-assisted-entity-artwork) below
- Scrivener / Fountain export; advanced Git branching UX
- Series-level continuity management
- Multi-region, scale-out collaboration tuning
- **OpenAPI catch-up** — bring `docs/openapi.yaml` current with all B1–C routes; regenerate `api-types.ts`; restore codegen for newer endpoints (schedule before C3)
- **Customizable workspaces** — per-user, per-project saved panel layouts (which panels are open, their widths, which scene/chapter is active); named workspace presets ("drafting", "research", "editing") switchable from the TopBar; stored in `user_workspaces` table (JSONB layout blob); synced across sessions so the editor opens exactly where you left off

### D-Map — AI-assisted map builder

> Scoped 2026-07-17. Two-layer model: structural **layout** data (JSON, git-diffable — shapes + symbols) is separate from rendered artwork (binary, regenerated from the layout on demand). AI can both edit the layout (tool calls, like the existing manuscript agent) and generate art from it (new image-adapter layer, multi-provider). Draft iteration is ephemeral — nothing persists until the user commits. Everything lives in the project's git working tree, so Chronicle/Diverge/TravelTo/Canonize apply to maps for free, same as scenes. A project can hold many maps at different scales (world, region, city, galaxy, planet, custom), and maps plug into the existing wiki relationship graph so they can reference each other and other entities.

**M1 — Data model + git plumbing** `[Medium]` ✅ complete (2026-07-17)
- Migration `000038_map_entity_type` adds `'map'` to the `wiki_entities.type` CHECK constraint (the first constraint-altering migration in this repo — drop+recreate, since the original was unnamed/auto-named) — maps are a `wiki_entities` row rather than a standalone table, so they get `parent_entity_id` hierarchy and `wiki_relationships` edges for free
- `attributes` JSONB holds only `{map_type: 'world'|'region'|'city'|'galaxy'|'planet'|'custom'}` — **deviates from the original spec, which also included a redundant `git_path`**: since the layout file path is 100% deterministic from the entity ID (`maps/<id>.json`), it's derived everywhere instead of stored
- New `internal/maps` package (`service.go`/`handler.go`/`models.go`), constructed with `(*sqlcgen.Queries, *project.GitService)` — mirrors `internal/merge`'s pattern of taking `GitService` directly and doing its own inline owner/collaborator repo-path resolution rather than importing `project.Service`
- `GitService.ReadMapFile`/`WriteMapFile` (`internal/project/git.go`) mirror `ReadSceneFile`/`WriteSceneFile` exactly: `maps/<id>.json`, no staging/commit — picked up by the writer's next `Chronicle`, same as scene prose. The `.png` rendered-artwork half of the original spec is deferred to M5 — nothing produces image bytes until then, so the plumbing would be dead code today
- CRUD routes `GET/POST /projects/:id/maps`, `GET/PUT/DELETE /projects/:id/maps/:mid`; branch-awareness comes for free from the same mechanism scenes use — reads/writes touch whatever's currently checked out in the resolved repo path, no explicit branch parameter needed in the map calls themselves
- A project can have any number of map entities; each is committed/saved independently, so a "world map," its "region map," and a "city map" nested under it coexist as separate git-versioned files linked through `parent_entity_id`
- 8 integration tests in `internal/maps/handler_test.go` (create with defaults/parent/invalid type, layout round-trip through git, list omits layout, update name/type, delete, cross-project rejection)
- No frontend work in this pass — M2's Map Studio canvas is the first thing that will actually call these routes

**M2 — Manual canvas editor (layered layout)** `[Heavy]` ✅ complete (2026-07-17)
- `MapsHub.tsx` (list/create/delete, mirrors `WikiHub.tsx`'s `EntitiesTab`/`CreateEntityModal` shell) + `MapStudio.tsx` (the canvas editor), new routes `/projects/:id/maps` and `/projects/:id/maps/:mid`, entry point via a fourth `ActionCard` on `ProjectHome`
- `konva`/`react-konva` added as dependencies (pinned to `react-konva@18.2.16`/`konva@^10` for React 18 compatibility — latest `react-konva` requires React 19)
- Map creation picks a **map type** (world/region/city/galaxy/planet/custom); `paletteForMapType()` (`components/maps/palette.ts`) selects terrain (mountain/lake/river/ocean/forest/desert/swamp/city) vs. space (star/planet/moon/nebula/asteroid_field/space_station/wormhole/black_hole) — custom defaults to terrain; palette is data, `MapIcons.tsx` has one hand-drawn SVG per icon for the toolbar, canvas rendering uses separate Konva-primitive shapes (`renderSymbolShape` in `MapStudio.tsx`) since react-konva can't host DOM `<svg>` nodes
- **Shape layer**: click-to-place polygon points, double-click to close, then a popup picks type + fill color from a curated set (`REGION_COLORS`); whole-shape drag to move
- **Symbol layer**: click a palette icon, click the canvas to place — a point-in-polygon check against existing regions auto-sets `region_id` when the drop lands inside one; drag to move, Konva `Transformer` for scale/rotate; a side panel lets the writer link the symbol to any wiki entity *or another map* via `entity_id` (client-filtered search over `api.wiki.listEntities` + `api.maps.list`)
- Save is an explicit button (`PUT` the full `{regions, symbols}` layout) — matches the wiki entity edit→save pattern, not scene autosave, so a mid-draw layout never persists accidentally
- **Scope boundary, deliberately deferred**: no post-creation vertex editing (drag a single polygon corner — only whole-shape drag), no undo/redo, no multi-select, no freehand/non-polygon brush strokes, no pan/zoom (fixed 900×600 stage)

**M3 — AI layout tool calls** `[Heavy]`
- Extends the existing `ManuscriptTools` agent tool-call pattern (`internal/ai/tools.go`) with `MapTools`, mirroring exactly what M2's UI can do so AI and user edit the same draft state interchangeably: `add_region`/`update_region`/`remove_region` (shapes) and `add_symbol`/`update_symbol`/`remove_symbol` (icon type, position, scale, rotation, optional region/entity link)
- New agent chat surface scoped to the active map, reusing the `WorkshopPanel` chat UI and the planning-mandate/`AgentPhase` pattern from C2.5
- A user can hand-draw a coastline in M2, then ask the AI to "add a mountain range along the north border and drop three city icons along the coast" and see the same symbols appear, fully editable afterward

**M4 — Multi-provider image generation adapter** `[Heavy]`
- New `ImageAdapter` interface, parallel to the existing `Adapter`/`ToolAdapter`: `GenerateImage(ctx, prompt, referenceImage []byte) ([]byte, error)` — takes a reference image (the rendered layout, see M5) plus a text prompt, so any provider wired in must support image-conditioned generation (image-to-image / edits / references), not text-only
- Versatility is a hard requirement, not a "start with one and expand later" — build the `AdapterFactory` for images the same way `internal/ai/adapters` already does for chat (OpenAI/Anthropic/Ollama), so multiple providers are live from the first release: candidates are OpenAI (`gpt-image-1`), Google (Gemini image generation), and a hosted SDXL/Flux provider (Stability img2img or fal.ai) — each verified individually for reference-image support before being wired in (per the earlier scoping note: don't assume a provider supports image input just because it does text)
- Provider + model selectable per-user in Settings (mirrors the existing text-model provider picker and `user_api_keys` pattern); Settings gets an "Image generation" section with per-provider API keys and a default-provider selector, consistent with how text-model routing already works

**M5 — Layout → base image → generation flow** `[Medium–Heavy]`
- "Use as base image": client-side canvas export flattens the current `regions[]` + `symbols[]` into a single flat schematic raster (no AI involved — this is a deterministic render of the layout, done entirely in the browser/canvas) — this schematic is the reference image, not the final art
- "Generate": sends the schematic reference image + a style/mood text prompt to the selected `ImageAdapter`; the model uses the schematic as compositional ground truth (where the coastline, mountains, cities are) and produces polished map artwork from it; result renders inline in Map Studio, fully ephemeral until committed
- Iteration loop: further edits happen back on the layout (M2 UI or M3 chat), not on the generated art directly; each subsequent "Generate" re-flattens the *current* layout into a fresh schematic and re-runs generation, so the art always reflects the latest layout rather than drifting from repeated image-to-image passes
- "Commit": writes the final layout JSON and the latest generated artwork to git in one commit via `GitService`, the same path scene edits take through Chronicle — the map then participates in project history and branches like any other versioned file

**M6 — Polish / integration** `[Light–Medium]`
- Map thumbnail surfaced in `ProjectExplorer` / `ProjectHome`, and maps appear in the Wiki hub entity list (filterable by `map_type`) since they're `wiki_entities` rows
- Map version history reuses the existing Chronicle timeline UI (a map is just another versioned git file)
- Map-to-map and map-to-entity relationships (world → region → city, galaxy → planet, "region map depicts location X") are visible/editable through the existing wiki relationship UI with no new UI surface required — M1 already put maps in that graph
- Optional: map diff view (structural JSON diff + side-by-side image compare), analogous to `ProseDiffViewer`

---

### D-Portraits — AI-assisted entity artwork

> Scoped 2026-07-17. Lighter-weight sibling of D-Map: same `ImageAdapter` (reference image + prompt) and ephemeral-draft-until-saved philosophy from M4/M5 above, but no git layout step — wiki entities already have a DB-backed `image_key`/MinIO upload path from C1 (`POST/DELETE /wiki/entities/:eid/image`), so "saving" a generated portrait is just calling that existing endpoint with the generated bytes instead of a file picker. Applies to any wiki entity type (character, location, faction, item, concept, lore), not characters only. Because this only needs `ImageAdapter` and not the full map layout/canvas stack, **it's reasonable to build this before D-Map** and let D-Map's M4 reuse the adapter foundation this establishes, rather than the other way around.

**P1 — Generate flow** `[Medium]` ✅ complete (2026-07-17)
- "Generate portrait" action on `EntityDetail` for any entity type
- Backend builds a base prompt from the entity's existing data via new `buildPortraitPrompt` in `internal/ai/portrait.go` (kept separate from the prose-context helpers in `context.go` — those are tuned for narrative injection, not visual description), plus an optional free-text addition from the user ("wearing ceremonial armor", "cyberpunk lighting")
- `Service.GenerateEntityPortrait` calls `ImageAdapter.GenerateImage(prompt, referenceImage=nil)` for the first draft; result renders inline as an ephemeral draft in a new `EntityDetail` panel, matching D-Map's "nothing persists until saved" rule
- New `internal/ai/adapters.ImageAdapter` interface + `OpenAIImageAdapter` (`gpt-image-1`); route `POST /projects/:id/ai/entities/:eid/portrait`

**P2 — Revision loop** `[Medium]` ✅ complete (2026-07-17)
- "Request changes" field under the draft ("make the hair shorter", "darker armor") re-calls the same endpoint with `reference_image_base64` set to the current draft — same `GenerateImage(prompt, referenceImage)` signature D-Map M4 will reuse, no new adapter method needed
- Draft state lives entirely in frontend React state (`WikiHub.tsx` `EntityDetail`); nothing persisted server-side until save
- No git involved anywhere in this flow — wiki entities aren't git-versioned, so this is strictly simpler than maps

**P3 — Save / replace portrait** `[Light]` ✅ complete (2026-07-17)
- "Use this image" converts the base64 draft to a `File` client-side and pipes it through the existing C1 upload path (`POST /wiki/entities/:eid/image`) — zero new backend storage code
- Regenerating an entity that already has a manually-uploaded portrait simply replaces it, same lifecycle as today's upload/remove flow

**P4 — Prompt quality / style consistency** `[Light–Medium]` ✅ complete (2026-07-17)
- Project-level art style guidance: `GenerateEntityPortrait` fetches the project's `ai_instructions` (AI Bible, C0.5) and folds a truncated excerpt (`artStyleExcerptRunes = 300`, matching `truncateRunes`'s existing truncation-marker convention) into the prompt as a labeled "Visual style guidance" section — reuses the existing Bible rather than inventing a new setting
- `buildPortraitPrompt` now also includes character `attributes.capability_notes` (skills/knowledge — the most visually suggestive of the existing structured character fields) ahead of the generic `summary` text; no dedicated "appearance" field exists in the schema, so this uses what's already there rather than adding one
- Applied only on first-draft generation (`referenceImage == nil`) — revisions treat `prompt` as a pure edit instruction against the reference image, unchanged from P2

---

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

*Last updated 2026-07-14: C8 admin area, C9 manuscript import, C9.5 entity rename cascade, C9-P1–P7 AI quality improvements (behavioral prompts, context efficiency, summarization overhaul, task-tier model routing, prose fingerprinting, chat mode specialization, semantic RAG via pgvector), OpenRouter background model, Groq tier routing, and P2 hardening (DFS depth cap, rate limiting confirmed, localStorage token, api.ts URL assertion, context.Background() audit) all complete. C-series and all P0–P2 hardening items are fully done. Next: Phase D (map builder, image generation, Scrivener/Fountain/PDF export, esbuild/vite audit fixes, customizable workspaces).*
