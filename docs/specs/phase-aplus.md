# Phase A+ — Pre-Phase B polish

**Goal:** Close the remaining UX gaps and foundational items before wiring AI (Phase B). All tasks here are no-AI-dependency — they make the writing experience more complete and lay the groundwork Phase B expects to find.

**Exit criteria (Phase A+ done):**

- Scene editor surfaces all scene metadata (POV, tense, tags, summary, word count) — not just raw text.
- AI provider keys are securely stored and ready for the AI adapter layer to consume.
- Wiki autolink works in the editor — entities in the current scene are highlighted.
- Writer can go full-screen without UI chrome distracting from the prose.
- Project home page gives a meaningful overview before entering the editor.
- Users can delete their own account (GDPR).
- Both light and dark themes available.
- Relationship graph is visible in the wiki hub.

**Companion docs:** [ROADMAP.md](../../ROADMAP.md) · [PROJECT_PLAN.md](../PROJECT_PLAN.md)

---

## Completed

| # | Task | Status |
|---|------|--------|
| A+1 | Word count + scene metadata in editor | ✅ done |
| A+2 | Secure AI key storage | ✅ done |
| A+3 | Autolink wired in editor | ✅ done |

---

## A+4 — Focus / distraction-free mode

**Effort:** Low · **Scope:** Frontend only · **Blocking for Phase B:** No

| # | Task | Status | Acceptance criteria |
|---|------|--------|---------------------|
| A+4.1 | Toggle trigger | ⬜ todo | A button in the editor toolbar (or `F11`) enters focus mode. State stored in React (`useState`). |
| A+4.2 | Focus layout | ⬜ todo | In focus mode: `ActivityBar`, left panel, `ProjectExplorer`, `TopBar`, `StatusBar`, and `SceneMetadataPanel` are all hidden. `ScribeEditor` fills the full viewport. |
| A+4.3 | Exit | ⬜ todo | `Esc` key or a floating exit button (top-right, appears on mouse movement) exits focus mode and restores the previous layout. |

**Implementation notes:**

- Add `focusMode: boolean` state to `Editor.tsx`. Conditionally render panels.
- A thin floating bar (opacity 0 → 1 on hover) in the top-right corner shows the exit button — keeps it accessible without cluttering the prose view.
- No backend changes.

---

## A+5 — Project home / stats page

**Effort:** Medium · **Scope:** Backend + Frontend · **Blocking for Phase B:** No

| # | Task | Status | Acceptance criteria |
|---|------|--------|---------------------|
| A+5.1 | Backend endpoint | ⬜ todo | `GET /projects/:id/stats` returns `ProjectStats`: `total_word_count`, `chapter_count`, `scene_count`, `last_updated`. Aggregated in SQL — no N+1. |
| A+5.2 | OpenAPI + codegen | ⬜ todo | `ProjectStats` schema documented; `npm run gen:api` clean; CI drift check passes. |
| A+5.3 | Project home page | ⬜ todo | `/projects/:id` (before the editor) is a dedicated overview page, not a direct jump to the editor. Shows stats card, chapter list with scene counts, last-edited timestamps, wiki entity count, "Open Editor" and "Open Wiki" buttons. |
| A+5.4 | Editor entry | ⬜ todo | "Open Editor" on the project home page navigates to `/projects/:id/editor` (rename current editor route) or passes a query param. Alternatively keep `/projects/:id` as the home and add `/projects/:id/write` for the editor. |

**Implementation notes:**

- SQL for stats: `SELECT COUNT(*) scenes, SUM(word_count) words FROM scenes JOIN chapters ON ...` — single query.
- The project home is the natural place to add a "novel guide" CTA later in Phase B.
- Router change: current `<Route path="/projects/:id">` becomes the home page; editor moves to `/projects/:id/write`. Update all `navigate('/projects/${id}')` calls in Dashboard.

---

## A+6 — User account deletion

**Effort:** Low–Medium · **Scope:** Backend + Frontend · **Blocking for Phase B:** No

| # | Task | Status | Acceptance criteria |
|---|------|--------|---------------------|
| A+6.1 | Backend endpoint | ⬜ todo | `DELETE /users/me` behind `RequireAuth`. Deletes user row — FK cascades remove projects, chapters, scenes, wiki, refresh tokens, API keys. Also removes git repo directories from disk for all owned projects. Returns `204 No Content`. |
| A+6.2 | OpenAPI + codegen | ⬜ todo | Endpoint documented; `npm run gen:api` clean. |
| A+6.3 | Settings danger zone | ⬜ todo | New "Danger Zone" section at bottom of `/settings`. Button opens a confirm dialog requiring the user to type their email address. On match, calls `DELETE /users/me`, clears auth store, redirects to `/login`. |

**Implementation notes:**

- Git repo cleanup: the service needs to call `os.RemoveAll` for each `project.git_repo_path` before the DB row is deleted (or do it after with best-effort).
- Cascade in DB is already set up (`ON DELETE CASCADE` on all child tables).
- The confirm-by-email pattern prevents accidental deletion; no second API call needed.

---

## A+7 — Light theme

**Effort:** Low · **Scope:** Frontend only · **Blocking for Phase B:** No

| # | Task | Status | Acceptance criteria |
|---|------|--------|---------------------|
| A+7.1 | CSS variable system | ⬜ todo | All `brand-*` color tokens in `tailwind.config.js` / `index.css` are defined as CSS variables on `:root`. A `.light` class on `<html>` overrides them with light-mode values. |
| A+7.2 | Theme store | ⬜ todo | `useThemeStore` (Zustand, persisted to `localStorage`) tracks `'dark' | 'light'`. On mount, applies the correct class to `<html>`. |
| A+7.3 | Theme toggle | ⬜ todo | Toggle button in `/settings` switches between dark and light. Change is immediate and persists across sessions. |
| A+7.4 | Light palette | ⬜ todo | Light mode overrides: white/off-white backgrounds, dark text, muted borders, same accent colors (cyan, gold, purple) slightly deepened for contrast. All existing pages readable in both themes. |

**Implementation notes:**

- Tailwind `darkMode: 'class'` is the right config. Existing `bg-brand-*` classes stay unchanged — only the CSS variable values swap.
- Test each major page (Login, Dashboard, Editor, WikiHub, Settings) in light mode before marking done.
- `prefers-color-scheme` media query can set the default if no preference is stored.

---

## A+8 — Relationship graph visualization

**Effort:** Medium · **Scope:** Frontend only · **Blocking for Phase B:** No

| # | Task | Status | Acceptance criteria |
|---|------|--------|---------------------|
| A+8.1 | Install graph lib | ⬜ todo | Add `d3` (or `@visx/network`) to `frontend/package.json`. Types included. Bundle size impact reviewed — consider dynamic import if >50 kB. |
| A+8.2 | `RelationshipGraph` component | ⬜ todo | Force-directed SVG graph. Nodes are wiki entities, colored by type (use existing `TYPE_COLORS`). Edges are relationships, labeled with `type`. Node radius scales with connection count. |
| A+8.3 | Interaction | ⬜ todo | Clicking a node opens entity detail (reuse `WikiHub`'s entity detail or navigate to it). Hovering a node highlights its edges. Graph is zoomable/pannable. |
| A+8.4 | WikiHub integration | ⬜ todo | `WikiHub` gets a third "Graph" tab alongside "Entities" and "Timeline". Tab calls `GET /wiki/graph` (endpoint already exists); passes data to `RelationshipGraph`. Empty state: "Add entities and relationships to see the graph." |

**Implementation notes:**

- `GET /wiki/graph` returns `{ entities: [...], relationships: [...] }` — already implemented and tested.
- d3 force simulation: `forceLink` + `forceManyBody` + `forceCenter`. SVG with `<g>` transform for pan/zoom.
- Consider `@visx/network` as a higher-level alternative to raw d3 if the team prefers React-idiomatic code.
- Dynamic import (`const d3 = await import('d3')`) inside `RelationshipGraph` avoids bloating the initial bundle.

---

## Suggested implementation order

| Priority | Task | Why |
|----------|------|-----|
| 1 | **A+4** Focus mode | Pure frontend, 1–2 hours, immediately improves writing UX |
| 2 | **A+7** Light theme | Frontend only, sets up the CSS variable system before more pages are added |
| 3 | **A+5** Project home | Requires small backend addition; natural Phase B entry point (novel guide CTA) |
| 4 | **A+6** Account deletion | Low backend effort; important for self-hosted trust |
| 5 | **A+8** Relationship graph | Medium frontend effort; makes wiki feel complete |

---

## Out of scope for Phase A+

- AI-dependent features (plot hole detection, consistency checks) — Phase B
- Async worker / job queue — Phase B
- Vector memory / RAG / embeddings — Phase B+
- Admin dashboard — Phase C+

---

*Linked from [ROADMAP.md](../../ROADMAP.md) and [PROJECT_PLAN.md](../PROJECT_PLAN.md).*
