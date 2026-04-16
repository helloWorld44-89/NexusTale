# NexusTale

A sci-fi/fantasy novel-writing platform: structured manuscripts (projects → acts → chapters → scenes), Git-backed narrative history, a world wiki, and AI-assisted drafting powered by Anthropic, OpenAI, or a local Ollama instance.

![CI](https://github.com/helloWorld44-89/NexusTale/actions/workflows/dev.yml/badge.svg?branch=dev)

---

## What's working

| Area | Status |
|------|--------|
| **Auth** | Register / login / refresh / logout; JWT access + refresh tokens; AES-256-GCM API key storage |
| **Manuscript** | Projects → Acts → Chapters → Scenes; word count; tags; per-scene POV / tense / summary |
| **Git (Chronicle)** | Snapshot, history, diff, branch (Diverge), switch (TravelTo), merge (Canonize), paradox detection |
| **World wiki** | Entities, relationships, magic rules, timeline events; d3 relationship graph; entity image upload (MinIO) |
| **AI** | Beat expansion + scene continuation (SSE); Nexus chat (SSE); chapter auto-summarize (debounced); project AI Bible; `@[Entity]` inline refs in context; Anthropic / OpenAI / Ollama adapters; token + cost tracking |
| **AI context pins** | Writer-curated entities / chapters / scenes pinned into every AI call; summary or full mode |
| **Writing styles** | Named prose presets per project; import / export as JSON |
| **Novel guide** | 5-step wizard (Premise → Characters → World → Outline → First Scene) that scaffolds wiki + manuscript |
| **Story structures** | 12 seeded templates + scoring questionnaire; structure phases injected into AI context |
| **Export** | Markdown (zip, sync); EPUB + DOCX (async jobs, MinIO, presigned download URL) |
| **Frontend** | React 18 + Vite + TypeScript + Tailwind; VSCode-style editor, wiki hub, git panel, Nexus AI chat, guide wizard, export panel, settings |
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
| `NEXUSTALE_REDIS_URL` | `redis://localhost:6379` | Used for future collaboration pub/sub |
| `NEXUSTALE_MINIO_ENDPOINT` | `localhost:9000` | Used for EPUB/DOCX exports and wiki image uploads |
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
      project/            # Projects, acts, chapters, scenes; git service
      wiki/               # Entities, relationships, magic rules, timeline; image upload
      ai/                 # Adapters (Anthropic/OpenAI/Ollama), context window, chat/complete/summarize
      prompts/            # Writing style presets CRUD; import/export
      export/             # Markdown zip, EPUB (go-epub), DOCX (raw OOXML); async job pool
      guide/              # Novel guide wizard steps + story structure scoring
      collaboration/      # WebSocket/CRDT hub (C3 — not yet wired)
    pkg/
      db/
        migrations/       # golang-migrate SQL files (000001 … 000018)
        queries/          # sqlc source files — edit these, then run make sqlc
        sqlcgen/          # Generated — do not hand-edit
      storage/            # MinIO/S3 client (PutObject, GetObject, PresignedGetURL, DeleteObject)
      cache/              # Cache interface (Redis — reserved for collaboration)
  frontend/
    src/
      app/                # Router, providers, auth store
      pages/              # Dashboard, Editor, ProjectHome, WikiHub, Guide, Settings
      components/
        ai/               # ChatBar (Nexus), BeatInput, ContextPanel, PromptLibrary
        editor/           # ScribeEditor, SceneMetadataPanel, GitPanel, BeatInput
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
- **go-git** — per-project repos on disk; Chronicle = commit, Diverge = branch, Canonize = fast-forward merge
- **MinIO** — EPUB/DOCX export jobs + wiki entity image uploads; presigned URLs for downloads
- **Redis** — provisioned; reserved for real-time collaboration (Phase C3)
- **AI adapters** — Anthropic (`claude-haiku-4-5`), OpenAI (`gpt-4o-mini`), Ollama (any model); API keys stored encrypted per user; automatic Ollama fallback

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
