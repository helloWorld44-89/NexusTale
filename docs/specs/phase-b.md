# Phase B — AI + Export Core

**Goal:** Turn NexusTale from a structured writing tool into an AI-assisted one. Writers can chat with an AI about their story, get scene continuations, have summaries auto-generated, export their manuscript, and be guided through starting a new project.

**Entry criteria (Phase A+ done):**
- API keys stored and encrypted per user (`/users/me/api-keys`)
- Project → Act → Chapter → Scene hierarchy fully implemented
- Wiki (entities, relationships, timeline) fully implemented
- ProjectHome shows stats; editor at `/projects/:id/editor`

**Exit criteria (Phase B done):**
- AI chat and scene continuation work with at least one cloud provider + Ollama
- Chapter summaries generated automatically; stale flag surfaced in UI
- Token usage tracked and displayed on ProjectHome
- Markdown export downloads immediately; EPUB export queues, polls, downloads
- Novel guide wizard scaffolds a new project in ≤ 5 steps

**Sub-specs:**
- [phase-b-ai.md](./phase-b-ai.md) — adapters, routes, context window, token tracking
- [phase-b-export.md](./phase-b-export.md) — Markdown zip, EPUB async job, MinIO
- [phase-b-guide.md](./phase-b-guide.md) — novel guide wizard
- [phase-b-structures.md](./phase-b-structures.md) — optional story structure templates + recommendation wizard

**Companion docs:** [ROADMAP.md](../../ROADMAP.md) · [PROJECT_PLAN.md](../PROJECT_PLAN.md)

---

## Milestone map

| Step | What ships | Depends on |
|------|-----------|------------|
| B1 — AI proxy | `/ai/complete` (continue + beat modes), `/ai/chat`, `/ai/summarize`; ChatBar live; thinking model detection | User API keys (done) |
| B1.5 — Writing styles | `project_prompts` table; CRUD routes; style selector in SceneMetadataPanel; beat UI in editor | B1 |
| B2 — AI memory | Auto-summarize; context window with `@[entity]` resolution; stale indicator | B1 |
| B3 — Token tracking | `ai_usage` table; ProjectHome usage stats | B1 |
| B4 — Export | Markdown zip; EPUB async + MinIO | Nothing (independent) |
| B5 — Novel guide | 5-step wizard; scaffolds wiki + scenes | B1 (uses AI for suggestions, optional) |
| B5.5 — Story structure | Optional template library + recommendation wizard; guide Step 3.5 | B5 |

B4 can run in parallel with B1–B3. B1.5 can start immediately after B1. B5 depends on B1 being done. B5.5 is additive — skipping it never breaks anything.

---

## Migrations needed

| # | Table | Purpose |
|---|-------|---------|
| 010 | `chapter_summaries (chapter_id, branch_name, …)`, `project_active_branch` | Branch-isolated chapter summaries; active branch tracking per user |
| 011 | `ai_usage` | Per-call token + cost tracking |
| 012 | `export_jobs` | Async export job state + MinIO key |
| 013 | `guide_steps` | Novel guide wizard progress per project |
| 014 | `project_prompts`, `user_api_keys.force_non_streaming` | Writing style presets per project; non-streaming override per key |
| 015 | `novel_structures` (seeded), `projects.structure_id`, `projects.structure_custom` | Optional story structure templates; nullable — freeform is always valid |

---

## Key architectural decisions

### Streaming vs batch AI responses
- Chat and completion use **SSE (Server-Sent Events)** — Gin supports `c.Stream()`; client uses `EventSource` or manual `fetch` + `ReadableStream`
- Summarize is fire-and-forget batch — returns plain JSON once complete

### Context window strategy (B2)
```
[system prompt]
  World: {project title}, {genres}
  Wiki entities: {autolinked entities from current scene, with summaries}
  
[recent chapter summaries — last 3 chapters]

[recent scenes — last 2 scenes, full content]

[user message or current scene content]
```
Cap at ~8k tokens (configurable). Summaries compress older context; recent scenes stay verbatim.

### EPUB async job (B4)
1. `POST /export/epub` → creates `export_jobs` row with `status=queued`, returns `jobId`
2. Background goroutine picks it up, generates EPUB, uploads to MinIO
3. `GET /export/jobs/:jobId` → `{status, download_url}` where `download_url` is a signed MinIO URL (1h TTL)
4. Frontend polls every 3s until `status=done`

### Novel guide scaffolding (B5)
Each completed step writes real data so the guide isn't throwaway:
- Step 1 (Premise) → updates `project.description`
- Step 2 (Characters) → creates `wiki_entities` of type `character`
- Step 3 (World) → creates `wiki_entities` of type `location` + `faction`; optionally `wiki_magic_rules`
- Step 4 (Outline) → creates chapters with titles and summaries
- Step 5 (First scene) → creates first scene with AI-assisted opening paragraph

---

## Checklist (all phases)

### B1 — AI proxy + adapters
- [x] **B1.1** Define `Adapter` interface (`adapters/interface.go`): `Complete`, `Chat`, `Summarize`, `StreamComplete`, `StreamChat`, `IsThinkingModel`, `Provider`; `CompleteMode` type (`continue` | `beat`)
- [x] **B1.2** `OpenAIAdapter` (`adapters/openai.go`): gpt-4o-mini default; thinking model detection; `simulateStream` for non-streaming models
- [x] **B1.3** `AnthropicAdapter` (`adapters/anthropic.go`): claude-haiku-4-5-20251001 default; Anthropic SSE event parsing; system prompt in `System` field
- [x] **B1.4** `OllamaAdapter` (`adapters/ollama.go`): `/api/chat` NDJSON streaming; no key needed
- [x] **B1.5** `AdapterFactory` (`adapters/factory.go`): provider preference order `[anthropic, openai]` → Ollama fallback; thinking model detection (`o1/o3/o4/deepseek-reasoner/qwq/r1`)
- [x] **B1.6** AI service (`ai/service.go`): beat/continue system prompt builders with project/scene metadata; `resolveContext` fetches scene+project non-fatally
- [x] **B1.7** HTTP routes (`ai/handler.go`): `POST /projects/:id/ai/complete`, `/ai/chat` (SSE via `io.Pipe`), `/ai/summarize` (JSON); all behind `RequireAuth`; `config.AIConfig` wired in `cmd/api/main.go`
- [x] **B1.8** OpenAPI: deferred to after B1.9 (types added inline to `api.ts` for now)
- [x] **B1.9** Frontend: wire `ChatBar` to `POST /ai/chat`; display streaming response with SSE; pass `projectId` + `sceneId` as context; typing indicator + Stop button

### B1.5 — Writing styles (prose prompts)
- [ ] **B1.5.1** Migration 014: `project_prompts` table (`id, project_id, name, category, content, system_content, sort_order`); `ALTER TABLE user_api_keys ADD COLUMN force_non_streaming BOOL NOT NULL DEFAULT false`
- [ ] **B1.5.2** sqlc: `ListProjectPrompts`, `CreateProjectPrompt`, `UpdateProjectPrompt`, `DeleteProjectPrompt`; regenerate
- [ ] **B1.5.3** HTTP routes: `GET/POST /projects/:id/prompts`, `PUT/DELETE /projects/:id/prompts/:promptId`; behind `RequireAuth`
- [ ] **B1.5.4** AI service: `ApplyPromptPreset(req CompleteRequest, preset ProjectPrompt)` — merges `system_content` into system prompt, appends `content` as style block to user turn
- [ ] **B1.5.5** OpenAPI: `ProjectPromptResponse`, `CreateProjectPromptRequest`, `UpdateProjectPromptRequest`; regenerate types
- [ ] **B1.5.6** Frontend: writing style dropdown in `SceneMetadataPanel`; "Manage styles" link; selected `prompt_id` sent with every AI call
- [ ] **B1.5.7** Frontend: Beat input field in `ScribeEditor` toolbar; send with `mode: "beat"`; streamed response appended with highlight + Accept/Retry/Discard actions

### B2 — AI memory + context
- [ ] **B2.1** Migration 010: create `chapter_summaries (chapter_id, branch_name, ai_summary, stale, updated_at)` with PK `(chapter_id, branch_name)`; create `project_active_branch (project_id, user_id, branch_name, updated_at)` with PK `(project_id, user_id)` — **no column added to `chapters`**
- [ ] **B2.2** sqlc: `UpsertChapterSummary`, `GetChapterSummary(chapterID, branchName)`, `MarkChapterSummaryStale`, `DeleteBranchSummaries(projectID, branchName)`, `UpsertProjectActiveBranch`, `GetProjectActiveBranch`, `ClearActiveBranch`; regenerate
- [ ] **B2.3** Branch resolution helper: `ResolveBranch(ctx, projectID, userID, headerBranch)` → checks header → `project_active_branch` → defaults `"canon"`
- [ ] **B2.4** `TravelTo` and `Diverge` handlers: upsert `project_active_branch` after git HEAD switch
- [ ] **B2.5** Auto-summarize goroutine: debounce key is `(chapter_id, branch_name)`; on fire, upsert `chapter_summaries` for that branch; on scene save, mark only active branch row stale
- [ ] **B2.6** `BuildContext(ctx, db, projectID, sceneID, branchName, userText)`: chapter summaries queried by active branch, falling back to `"canon"` if no branch-specific row; inline `@[entity]` parsing (up to 5 additional wiki entries injected before user turn)
- [ ] **B2.7** Wire context builder into `Complete` and `Chat` routes; pass `X-NexusTale-Branch` header through to `ResolveBranch`
- [ ] **B2.8** `ChapterResponse` gains `ai_summary` and `ai_summary_stale` sourced from `chapter_summaries` for the requesting user's active branch; update OpenAPI; regenerate types
- [ ] **B2.9** `Canonize` handler: call `DeleteBranchSummaries` + `ClearActiveBranch` for merged branch after merge completes
- [ ] **B2.10** Frontend: stale indicator badge on chapter in ProjectExplorer; "Regenerate" button in SceneMetadataPanel; send `X-NexusTale-Branch` header on all AI and scene-save requests

### B3 — Token usage tracking
- [ ] **B3.1** Migration 011: `ai_usage` table (id, user_id, project_id, model, prompt_tokens, completion_tokens, cost_usd, created_at)
- [ ] **B3.2** sqlc: `InsertUsage`, `GetProjectUsageSummary` (total tokens, total cost, this month)
- [ ] **B3.3** Record usage after every AI call (non-blocking; log on error)
- [ ] **B3.4** `GET /projects/:id/ai/usage` → `{total_tokens, total_cost_usd, monthly_tokens, monthly_cost_usd}`
- [ ] **B3.5** OpenAPI: `AIUsageResponse` schema; regenerate types
- [ ] **B3.6** Frontend: usage stat cards on ProjectHome alongside word/scene/chapter counts

### B4 — Export
- [ ] **B4.1** Migration 012: `export_jobs` table (id, project_id, user_id, format, status, minio_key, download_url, error, created_at, expires_at)
- [ ] **B4.2** `GET /projects/:id/export/markdown` — walk acts→chapters→scenes; build zip in memory; stream as `application/zip`
- [ ] **B4.3** `POST /projects/:id/export/epub` — create `export_jobs` row; enqueue to goroutine pool; return `{job_id}`
- [ ] **B4.4** EPUB goroutine: `internal/export` epub builder; upload to MinIO; update job row with signed URL
- [ ] **B4.5** `GET /projects/:id/export/jobs/:jobId` — return `{status, download_url, error}`
- [ ] **B4.6** OpenAPI: export endpoints + `ExportJobResponse`; regenerate types
- [ ] **B4.7** Frontend: Export card on ProjectHome; Markdown download (direct link); EPUB "Generate" button → poll → download

### B5 — Novel guide
- [ ] **B5.1** Migration 013: `guide_steps` table (id, project_id, step_key TEXT, data JSONB, completed_at)
- [ ] **B5.2** sqlc: `UpsertGuideStep`, `ListGuideSteps`
- [ ] **B5.3** `GET /projects/:id/guide` — returns all step states (key, data, completed)
- [ ] **B5.4** `POST /projects/:id/guide/:stepKey` — saves step data; triggers side effects (create entities, chapters, etc.)
- [ ] **B5.5** Step side-effect handlers: Premise → update project; Characters → create entities; World → create entities; Outline → create chapters; First scene → create scene + optional AI opening
- [ ] **B5.6** OpenAPI: guide endpoints + `GuideStepResponse`; regenerate types
- [ ] **B5.7** Frontend: `/projects/:id/guide` — linear wizard with step sidebar; progress indicator; each step has a form + preview of what will be created; "Skip" allowed; "Finish guide" exits to ProjectHome

### B5.5 — Story structure (optional templates)
> Structure is never required. Freeform is always a valid first-class choice.
> See full spec: [phase-b-structures.md](./phase-b-structures.md)

- [ ] **B5.5.1** Migration 015: `novel_structures` table seeded with 12 templates; `projects.structure_id UUID NULL REFERENCES novel_structures`; `projects.structure_custom JSONB NULL`
- [ ] **B5.5.2** sqlc: `ListNovelStructures`, `GetProjectStructure`, `UpdateProjectStructure`; regenerate
- [ ] **B5.5.3** Scoring function: `internal/guide/score.go` — deterministic weighted matrix; returns empty slice when no structure clears threshold (→ freeform recommended)
- [ ] **B5.5.4** Routes: `GET /novel-structures` (no auth), `POST /projects/:id/guide/structure/score` (pure calculation, no persistence), `GET/PUT /projects/:id/structure`
- [ ] **B5.5.5** Guide Step 3.5 frontend — 4-path chooser: questionnaire / browse templates / freeform custom rules / skip; all paths clearly labeled as optional; "Continue without structure" always visible
- [ ] **B5.5.6** `BuildContext` extension (B2): inject structure phase context only when set; silently omit when absent or phase match fails
- [ ] **B5.5.7** OpenAPI: `NovelStructureResponse`, `StructureScoreRequest/Response`, `ProjectStructureResponse`; regenerate types
- [ ] **B5.5.8** Frontend: structure badge on ProjectHome shown only when a structure is selected; clicking reopens the selection step
