# NexusTale

A sci-fi/fantasy novel-writing platform: structured manuscripts (projects → acts → chapters → scenes), Git-backed narrative history, a world wiki, AI-assisted drafting, and collaborative editing — built for writers who think in worlds.

![CI](https://github.com/helloWorld44-89/NexusTale/actions/workflows/dev.yml/badge.svg?branch=dev)

---

## What's working

| Area | Status |
|------|--------|
| **Auth** | Register / login / refresh / logout; JWT access + refresh tokens; AES-256-GCM API key storage |
| **Manuscript** | Projects → Acts → Chapters → Scenes; word count; per-scene POV / tense / summary / role / goal / conflict |
| **Chronicle (Git)** | Snapshot, history, diff, branch (Diverge), switch (TravelTo), merge (Canonize), paradox detection; per-repo concurrency locking |
| **World wiki** | Entities (with portrait upload), relationships, magic rules (with structured attributes), timeline events; d3 relationship graph |
| **Entity mentions** | Auto-tagged entity names highlighted inline in the editor (dotted underline, type-colored); hover card with portrait + summary; right-click suppress; "Appears In" scene list per entity |
| **AI — drafting** | Beat expansion + scene continuation (SSE streaming); chapter auto-summarize (debounced 30s); project AI Bible; `@[Entity]` inline refs resolved in context; Anthropic / OpenAI / Ollama adapters; token + cost tracking |
| **AI — Workshop** | Per-project chat sessions with full manuscript tool use (append/replace scene, create scene/chapter/act); Agent mode with undo per action; phase-aware system prompts |
| **AI context** | Writer-curated context pins (entities / chapters / scenes / research notes); summary or full mode; pinned block injected into every AI call |
| **Writing styles** | Named prose presets per project; import / export as JSON |
| **Revision phases** | Project phase (Drafting → Revision → Language Pass → etc.); phase badge in TopBar; Workshop system prompt adapts per phase |
| **Research notes** | Free-form notes with tags and source URLs; pinnable into AI context |
| **Beat history** | Recent beat prompts surfaced in BeatInput for one-click reuse |
| **Novel guide** | 5-step wizard (Premise → Characters → World → Outline → First Scene) that scaffolds wiki + manuscript |
| **Story structures** | 12 seeded templates + scoring questionnaire; structure phases injected into AI context |
| **Story threads** | Named open/closed narrative threads tracked per project; injected into AI context |
| **Export** | Markdown (zip, sync); EPUB + DOCX (async jobs, MinIO, presigned download URL) |
| **Collaboration** | Project invites; clone-based reviewer access; merge requests with line-level diffs; manuscript annotations (note / suggestion / question); in-app notifications |
| **Frontend** | React 18 + Vite + TypeScript + Tailwind; VSCode-style editor, wiki hub, git panel, Nexus AI chat, Workshop, guide wizard, export panel, settings, public landing page + waitlist |
| **CI/CD** | GitHub Actions → GHCR → Ansible → dev VM |

---

## Prerequisites

| Tool | Version |
|------|---------|
| Go | 1.23+ |
| Node | 20+ |
| Docker + Compose | v2 (the `docker compose` plugin) |

---

## Quick start (local dev)

```bash
# 1. Clone
git clone https://github.com/helloWorld44-89/NexusTale.git
cd NexusTale

# 2. Configure the backend
cp backend/.env.example backend/.env
# Edit backend/.env — at minimum set NEXUSTALE_AUTH_JWTSECRET and NEXUSTALE_ENCRYPTION_KEY

# 3. Start infrastructure (Postgres, Redis, MinIO)
make dev

# 4. Run the API (port 8080)
make run

# 5. Run the frontend (port 5173)
cd frontend && npm install && npm run dev
```

Open `http://localhost:5173` in your browser.

```bash
# Smoke test
curl http://localhost:8080/api/v1/healthz
# {"status":"ok"}
```

---

## Environment variables

All variables use the `NEXUSTALE_` prefix. Copy `backend/.env.example` and fill in:

| Variable | Default | Notes |
|----------|---------|-------|
| `NEXUSTALE_SERVER_PORT` | `8080` | API listen port |
| `NEXUSTALE_DB_URL` | `postgres://nexustale:nexustale@localhost:5432/nexustale?sslmode=disable` | PostgreSQL DSN |
| `NEXUSTALE_AUTH_JWTSECRET` | — | **Required.** Sign JWT access tokens |
| `NEXUSTALE_AUTH_ACCESSTOKENEXPIRY` | `15m` | |
| `NEXUSTALE_AUTH_REFRESHTOKENEXPIRY` | `168h` | 7 days |
| `NEXUSTALE_ENCRYPTION_KEY` | — | **Required.** 32-byte hex key for AES-256-GCM API key storage |
| `NEXUSTALE_REDIS_URL` | `redis://localhost:6379` | Used for collaboration pub/sub |
| `NEXUSTALE_MINIO_ENDPOINT` | `localhost:9000` | EPUB/DOCX exports and wiki image uploads |
| `NEXUSTALE_MINIO_ACCESSKEY` | `minioadmin` | |
| `NEXUSTALE_MINIO_SECRETKEY` | `minioadmin` | |
| `NEXUSTALE_GIT_REPOSPATH` | `./data/repos` | Where per-project Git repos live on disk |
| `NEXUSTALE_AI_OLLAMAURL` | `http://localhost:11434` | Ollama base URL (fallback when no cloud key) |
| `NEXUSTALE_AI_OLLAMAMODEL` | `llama3` | Default Ollama model |

See [`backend/.env.example`](backend/.env.example) for the full list.

---

## Available commands

From the repo root:

| Command | Description |
|---------|-------------|
| `make dev` | Start Postgres, Redis, MinIO via Docker Compose |
| `make dev-down` | Stop infrastructure containers |
| `make run` | Run the Go API (`cmd/api`) |
| `make build` | Compile the API binary to `backend/bin/api` |
| `make test` | Run backend tests (serial, `-p 1`) |
| `make sqlc` | Regenerate `pkg/db/sqlcgen` from SQL queries |
| `make tidy` | `go mod tidy` in `backend/` |

From `frontend/`:

| Command | Description |
|---------|-------------|
| `npm run dev` | Vite dev server on port 5173 |
| `npm run build` | Production build |
| `npm run lint` | ESLint |
| `npm run gen:api` | Regenerate `src/services/api-types.ts` from `docs/openapi.yaml` |

---

## Project structure

```
NexusTale/
  backend/
    cmd/api/              # Entry point — wires services, runs migrations, starts Gin
    internal/
      auth/               # JWT register/login/refresh/logout, middleware, API key encryption
      project/            # Projects, acts, chapters, scenes; git service; mention notifier
      wiki/               # Entities, relationships, magic rules, timeline; image upload; entity tagger
      ai/                 # Adapters (Anthropic/OpenAI/Ollama), context window, chat/complete/summarize/workshop
      prompts/            # Writing style presets CRUD; import/export
      export/             # Markdown zip, EPUB (go-epub), DOCX (raw OOXML); async job pool
      guide/              # Novel guide wizard steps + story structure scoring
      research/           # Research notes CRUD
      collaboration/      # Invites, clones, merge requests, annotations, notifications
      admin/              # (planned C8) User management, AI usage view
    pkg/
      db/
        migrations/       # golang-migrate SQL files (000001 … 000035)
        queries/          # sqlc source files — edit these, then run make sqlc
        sqlcgen/          # Generated — do not hand-edit
      storage/            # MinIO/S3 client (PutObject, GetObject, PresignedGetURL, DeleteObject)
      cache/              # Cache interface (Redis)
  frontend/
    src/
      app/                # Router, providers, auth store
      pages/              # Dashboard, Editor, ProjectHome, WikiHub, Guide, Settings, Landing
      components/
        ai/               # ChatBar (Nexus), BeatInput, WorkshopPanel, ContextPanel, PromptLibrary
        editor/           # ScribeEditor (TipTap), SceneMetadataPanel, GitPanel, AnnotationSidebar
          extensions/     # EntityMentionExtension (ProseMirror decoration plugin)
          utils/          # editorUtils (plainToHTML, buildCharToPos, …)
        wiki/             # WikiPanel, EntityDetail, RelationshipGraph, TimelineView
        layout/           # TopBar, ActivityBar, StatusBar
        guide/            # StructureStep
      services/           # api.ts (hand-written fetch layer) + api-types.ts (generated)
  docs/
    openapi.yaml          # OpenAPI 3.1.0 spec — source of truth for generated types
    PROJECT_PLAN.md       # Full architecture + phased delivery plan
  infra/
    docker/               # docker-compose files + Dockerfiles (API + frontend)
    ansible/              # Deployment playbooks (dev VM)
  bruno/                  # API test collection
  Makefile
```

---

## Architecture

- **PostgreSQL** — source of truth for all structured data; sqlc for type-safe queries
- **go-git** — per-project repos on disk; Chronicle = commit, Diverge = branch, Canonize = fast-forward merge; per-repo mutex prevents concurrent tree mutations
- **MinIO** — EPUB/DOCX export jobs + wiki entity image uploads; presigned URLs for client downloads
- **Redis** — provisioned; used for collaboration pub/sub
- **AI adapters** — Anthropic, OpenAI, Ollama; model selectable per user; API keys stored AES-256-GCM encrypted; automatic Ollama fallback when no cloud key is set
- **TipTap (ProseMirror)** — rich text editor in the frontend; entity mentions rendered as ProseMirror Decorations (no stored markup); plain-text round-trip preserved for backend storage

---

## Running tests

```bash
make test
```

Integration tests require a running Postgres instance (`make dev`). They skip gracefully when no DB is reachable.

---

## Contributing

1. Branch off `dev` for features; open PRs against `dev`.
2. CI runs on push to `dev`: Go tests → `tsc --noEmit` → ESLint → API types drift check → `sqlc diff` → Docker build + push → Ansible deploy.
3. After editing `pkg/db/queries/*.sql` or migrations, run `make sqlc` and commit the regenerated files.
4. After editing `docs/openapi.yaml`, run `cd frontend && npm run gen:api` and commit `src/services/api-types.ts`.
