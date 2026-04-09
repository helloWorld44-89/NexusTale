# NexusTale

A sci-fi/fantasy novel-writing platform: structured manuscripts (projects → chapters → scenes), Git-backed narrative history, a world wiki (entities, relationships, timeline, magic rules), and AI-assisted drafting (Phase B).

![CI](https://github.com/helloWorld44-89/NexusTale/actions/workflows/dev.yml/badge.svg?branch=dev)

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
# Edit backend/.env — at minimum change NEXUSTALE_AUTH_JWTSECRET

# 3. Start infrastructure (Postgres, Redis, MinIO)
make dev

# 4. Run the API (port 8080)
make run

# 5. Run the frontend (port 5173)
cd frontend
npm install
npm run dev
```

Open `http://localhost:5173` in your browser.

### Smoke test

```bash
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
| `NEXUSTALE_AUTH_JWTSECRET` | `change-me-in-production` | **Must change for any real deploy** |
| `NEXUSTALE_AUTH_ACCESSTOKENEXPIRY` | `15m` | |
| `NEXUSTALE_AUTH_REFRESHTOKENEXPIRY` | `168h` | 7 days |
| `NEXUSTALE_REDIS_URL` | `redis://localhost:6379` | Provisioned but not yet consumed (see below) |
| `NEXUSTALE_MINIO_ENDPOINT` | `localhost:9000` | Provisioned but not yet consumed (see below) |
| `NEXUSTALE_GIT_REPOSPATH` | `./data/repos` | Where per-project Git repos live on disk |

See [`backend/.env.example`](backend/.env.example) for the full list.

---

## Note on Redis and MinIO

Both services are started by `make dev` but are **not yet consumed by the API**.

- **Redis** will be used for collaboration pub/sub and rate limiting in Phase B.
- **MinIO** will store exports and wiki image uploads in Phase B.

You can safely ignore them during Phase A development.

---

## Available commands (repo root)

| Command | Description |
|---------|-------------|
| `make dev` | Start Postgres, Redis, MinIO via Docker Compose |
| `make dev-down` | Stop the infrastructure containers |
| `make run` | Run the Go API (`cmd/api`) |
| `make build` | Compile the API binary to `backend/bin/api` |
| `make test` | Run backend integration tests (serial, `-p 1`) |
| `make sqlc` | Regenerate `pkg/db/sqlcgen` from SQL queries |
| `make tidy` | `go mod tidy` in `backend/` |

Frontend commands (run from `frontend/`):

| Command | Description |
|---------|-------------|
| `npm run dev` | Vite dev server on port 5173 |
| `npm run build` | Production build |
| `npm run gen:api` | Regenerate `src/services/api-types.ts` from `docs/openapi.yaml` |

---

## Full-stack Docker (optional)

To run the complete stack (API + frontend + infrastructure) in Docker:

```bash
docker compose -f infra/docker/docker-compose.yml up
```

The nginx container proxies `/api/` to the Go backend and serves the React SPA on port 80.

---

## API exploration (Bruno)

A [Bruno](https://www.usebruno.com/) collection lives in `bruno/` with all 40 API routes pre-configured:

```
bruno/
  01-auth/       register, login, refresh, logout
  02-health/     healthz
  03-projects/   CRUD
  04-chapters/   CRUD
  05-scenes/     CRUD + update content
  06-wiki/       entities, relationships, magic rules, timeline events (incl. anchor tests)
  07-git/        chronicle, lore, echo, timelines, diverge, travel, canonize
  environments/
    local.bru    # points to localhost:8080
```

---

## Project structure

```
NexusTale/
  backend/
    cmd/api/              # Entry point
    internal/
      auth/               # JWT register/login/refresh/logout + middleware
      project/            # Projects, chapters, scenes, git service
      wiki/               # Entities, relationships, magic rules, timeline
      ai/                 # AI adapter stubs (Phase B)
      collaboration/      # CRDT/WebSocket stubs (Phase C)
      export/             # Export pipeline stubs (Phase B)
    pkg/
      db/
        migrations/       # golang-migrate SQL files
        queries/          # sqlc source files (edit these)
        sqlcgen/          # Generated — do not hand-edit
      cache/              # Interface stub
      storage/            # MinIO/S3 client stub
  frontend/
    src/
      app/                # Router, providers
      pages/              # Dashboard, Editor, Login, Register, WikiHub
      components/         # Editor, wiki, git, layout components
      services/           # api.ts (fetch layer) + api-types.ts (generated)
      store/              # Zustand auth store
  docs/
    openapi.yaml          # OpenAPI 3.1.0 spec — source of truth for types
    PROJECT_PLAN.md       # Full architecture + phased delivery
    specs/
      phase-a-mvp.md      # Phase A task checklist with acceptance criteria
  infra/
    docker/               # docker-compose files + Dockerfiles
    ansible/              # Deployment playbooks
  bruno/                  # API test collection
  Makefile
```

---

## Architecture overview

See [`docs/PROJECT_PLAN.md`](docs/PROJECT_PLAN.md) for the full design. In brief:

- **PostgreSQL** — source of truth for all structured data (users, projects, wiki, git refs)
- **go-git** — per-project bare repos on disk; Chronicle = commit, Diverge = branch, Canonize = fast-forward merge
- **Redis** — provisioned for Phase B (collaboration pub/sub, rate limiting)
- **MinIO** — provisioned for Phase B (exports, wiki image uploads)

---

## Running tests

```bash
make test
# or equivalently:
cd backend && go test ./... -v -count=1 -p 1
```

Tests require a running Postgres instance (`make dev`). They skip gracefully when no DB is available.

---

## Contributing

1. Branch off `dev` for features; open PRs against `dev`.
2. CI runs on push to `dev`: Go tests → frontend typecheck → API types drift check → Docker build + push → Ansible deploy.
3. After changing `pkg/db/queries/*.sql` or schema, run `make sqlc` and commit the regenerated files.
4. After changing `docs/openapi.yaml`, run `cd frontend && npm run gen:api` and commit `src/services/api-types.ts`.
