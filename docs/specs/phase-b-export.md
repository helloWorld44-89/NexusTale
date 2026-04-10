# Phase B — Export Sub-spec: Markdown & EPUB

## Overview

Two export modes serve different writer needs:
- **Markdown zip** — instant, for backup and portability. No queue, no MinIO, just stream a zip.
- **EPUB** — requires pandoc or a Go EPUB library; can take several seconds for large projects; runs async with a polling endpoint and a MinIO download URL.

---

## Markdown export (sync)

### Route
```
GET /projects/:id/export/markdown
Content-Disposition: attachment; filename="<project-title>.zip"
Content-Type: application/zip
```

### Structure inside the zip
```
<project-title>/
  Act 1/
    01 - Chapter Title/
      01 - Scene Title.md
      02 - Scene Title.md
  Act 2/
    ...
```

### File format (each scene)
```markdown
---
title: Scene Title
pov: Character Name
tense: past
tags: [action, revelation]
word_count: 847
---

Scene content here...
```

### Implementation notes
- Use `archive/zip` from stdlib — no dependencies
- Stream directly to `c.Writer` using `c.Stream()` so large projects don't buffer in memory
- Scene sort order: act.sort_order → chapter.sort_order → scene.sort_order
- Sanitise filenames: replace `/`, `\`, `:`, `*`, `?`, `"`, `<`, `>`, `|` with `-`

---

## EPUB export (async)

### Migration 012
```sql
CREATE TABLE export_jobs (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    format      TEXT NOT NULL DEFAULT 'epub',
    status      TEXT NOT NULL DEFAULT 'queued',  -- queued | processing | done | failed
    minio_key   TEXT NOT NULL DEFAULT '',
    download_url TEXT NOT NULL DEFAULT '',
    error       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL DEFAULT now() + INTERVAL '1 hour'
);
```

### Routes
```
POST /projects/:id/export/epub
→ 202 Accepted { "job_id": "uuid" }

GET /projects/:id/export/jobs/:jobId
→ 200 { "status": "done", "download_url": "https://...", "error": "" }
```

### Worker goroutine
- Simple channel-based pool (e.g. 3 workers, buffered channel of 50)
- On receipt: set `status=processing`
- Build EPUB using `github.com/bmaupin/go-epub` (pure Go, no pandoc)
- Upload to MinIO with key `exports/{projectID}/{jobID}.epub`
- Generate presigned URL (1h TTL)
- Update job row: `status=done`, `minio_key`, `download_url`
- On error: `status=failed`, `error=<message>`

### EPUB structure
```
content.opf    — manifest
toc.ncx        — table of contents
chapter-01.xhtml
chapter-02.xhtml
...
```
Each chapter = one XHTML file. Scenes within a chapter are separated by a horizontal rule and scene title heading. Act titles as `<h1>`, chapter titles as `<h2>`, scene titles as `<h3>`.

### Frontend flow
1. User clicks "Export EPUB" on ProjectHome
2. `POST /export/epub` → receive `job_id`; show spinner
3. Poll `GET /export/jobs/:jobId` every 3s
4. On `status=done`: show download button with `download_url`
5. On `status=failed`: show error message with retry button
6. Download URL expires after 1h; if expired (`expires_at` in response), user must re-generate

---

## MinIO setup for exports

Bucket: `nexustale-exports` (separate from any future user media bucket)

```go
// On startup, ensure bucket exists
minioClient.MakeBucket(ctx, "nexustale-exports", minio.MakeBucketOptions{})
```

Object lifecycle: set MinIO lifecycle rule to auto-delete objects after 24h (belt-and-suspenders on top of the 1h presigned URL TTL).

---

## OpenAPI schemas

```yaml
ExportJobResponse:
  type: object
  required: [job_id, status]
  properties:
    job_id:       { type: string, format: uuid }
    status:       { type: string, enum: [queued, processing, done, failed] }
    download_url: { type: string }
    error:        { type: string }
    expires_at:   { type: string, format: date-time }
```

---

## Future: DOCX export (Phase C)

Same async pattern, different renderer. The `export_jobs.format` column is TEXT so it's already flexible. Phase C adds `format=docx` with a different goroutine handler using `github.com/unidoc/unioffice` or similar.
