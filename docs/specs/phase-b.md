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

**Companion docs:** [ROADMAP.md](../../ROADMAP.md) · [PROJECT_PLAN.md](../PROJECT_PLAN.md)

---

## Milestone map

| Step | What ships | Depends on |
|------|-----------|------------|
| B1 — AI proxy | `/ai/complete`, `/ai/chat`, `/ai/summarize` routes; ChatBar live | User API keys (done) |
| B2 — AI memory | Auto-summarize; context window; stale indicator | B1 |
| B3 — Token tracking | `ai_usage` table; ProjectHome usage stats | B1 |
| B4 — Export | Markdown zip; EPUB async + MinIO | Nothing (independent) |
| B5 — Novel guide | 5-step wizard; scaffolds wiki + scenes | B1 (uses AI for suggestions, optional) |

B4 can run in parallel with B1–B3. B5 depends on B1 being done (AI suggestions are optional but expected).

---

## Migrations needed

| # | Table | Purpose |
|---|-------|---------|
| 010 | `chapters.ai_summary`, `chapters.ai_summary_stale` | Store auto-generated chapter summaries |
| 011 | `ai_usage` | Per-call token + cost tracking |
| 012 | `export_jobs` | Async export job state + MinIO key |
| 013 | `guide_steps` | Novel guide wizard progress per project |

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
- [ ] **B1.1** Define `Adapter` interface in `internal/ai/adapter.go`: `Complete`, `Chat`, `Summarize`, `StreamComplete`, `StreamChat`
- [ ] **B1.2** Implement `OpenAIAdapter` (gpt-4o default); reads key from `DecryptAPIKey`
- [ ] **B1.3** Implement `AnthropicAdapter` (claude-3-5-haiku default)
- [ ] **B1.4** Implement `OllamaAdapter` (local HTTP; model from config; no key needed)
- [ ] **B1.5** `AdapterFactory` — selects adapter by provider name from stored key; falls back to Ollama if no key
- [ ] **B1.6** AI service: `Complete(ctx, projectID, userID, content)`, `Chat(ctx, projectID, userID, messages)`, `Summarize(ctx, text)`
- [ ] **B1.7** HTTP routes: `POST /projects/:id/ai/complete`, `POST /projects/:id/ai/chat`, `POST /projects/:id/ai/summarize`; all behind `RequireAuth`
- [ ] **B1.8** OpenAPI: `AICompleteRequest`, `AIChatRequest`, `AIChatMessage`, `AICompleteResponse`, `AISummarizeResponse`; regenerate types
- [ ] **B1.9** Frontend: wire `ChatBar` to `POST /ai/chat`; display streaming response with SSE; show model name in header

### B2 — AI memory + context
- [ ] **B2.1** Migration 010: `ALTER TABLE chapters ADD COLUMN ai_summary TEXT NOT NULL DEFAULT '', ADD COLUMN ai_summary_stale BOOL NOT NULL DEFAULT false`
- [ ] **B2.2** sqlc: `UpdateChapterAISummary` query; regenerate
- [ ] **B2.3** Auto-summarize goroutine: triggered on scene save; debounced 30s per chapter; calls `Summarize`; sets `ai_summary_stale = false`
- [ ] **B2.4** Mark `ai_summary_stale = true` on any scene update in a chapter
- [ ] **B2.5** Context window builder: `BuildContext(ctx, projectID, sceneID)` → structured prompt prefix
- [ ] **B2.6** Wire context builder into `Complete` and `Chat` routes
- [ ] **B2.7** Expose `ai_summary` and `ai_summary_stale` in `ChapterResponse`; update OpenAPI
- [ ] **B2.8** Frontend: stale indicator badge on chapter in ProjectExplorer; "Regenerate" button in SceneMetadataPanel

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
