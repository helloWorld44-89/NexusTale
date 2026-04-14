# Phase B — Story Structure Sub-spec

## Philosophy

Structure is a **tool, not a requirement**.

Writers can:
- Pick a named structure from the template library and let it inform their outline and AI prompts
- Answer the scoring wizard to get a recommendation they may accept, modify, or ignore
- Skip structure entirely and write freeform — NexusTale works the same either way

At no point should the app gate any feature on having a structure selected. The AI context window,
chapter creation, and guide wizard all work identically with or without a structure. When a
structure *is* selected it adds optional context; when it is not, nothing changes.

---

## Migration 015

```sql
-- Seeded template library (read-only from the app's perspective)
CREATE TABLE novel_structures (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    phases      JSONB NOT NULL DEFAULT '[]',  -- [{name, description, hints}]
    strengths   TEXT NOT NULL DEFAULT '',
    risks       TEXT NOT NULL DEFAULT '',
    sort_order  INT  NOT NULL DEFAULT 0
);

-- Per-project selection — both columns nullable; null = freeform
ALTER TABLE projects
    ADD COLUMN structure_id     UUID REFERENCES novel_structures(id) ON DELETE SET NULL,
    ADD COLUMN structure_custom JSONB;  -- freestyle author rules: {acts, midpoint, ending, rules[]}
```

The `novel_structures` table is seeded at migration time from the 12 templates in
`docs/NOVEL_STRUCTURES.md`. It is never modified by user actions.

---

## Routes

```
GET  /novel-structures
→ [{ id, name, description, phases, strengths, risks }]
   — no auth required; public catalog

POST /projects/:id/guide/structure/score
Body: { answers: { [questionKey]: string[] } }
→ { ranked: [{ structure_id, name, score, is_secondary }] }
   — runs the scoring matrix server-side; does NOT set structure_id

PUT  /projects/:id/structure
Body: { structure_id?: uuid | null, structure_custom?: object | null }
→ { structure_id, structure_custom }
   — structure_id: null = freeform; structure_custom: null = clear custom rules

GET  /projects/:id/structure
→ { structure_id, structure_name, phases, structure_custom }
   — null fields when no structure selected
```

`POST /guide/structure/score` is a pure calculation — it returns ranked suggestions without
persisting anything. The writer decides what to apply, if anything.

---

## Scoring matrix implementation

`internal/guide/score.go`

```go
type ScoreRequest struct {
    Answers map[string][]string `json:"answers"`
}

type StructureScore struct {
    StructureID string `json:"structure_id"`
    Name        string `json:"name"`
    Score       int    `json:"score"`
    IsSecondary bool   `json:"is_secondary"`
}

// Score runs the weighted matrix from STRUCTURESELCTION.md.
// Returns structures ranked by score. Structures below the minimum
// threshold (6 pts) are excluded; if none qualify, returns empty slice
// (caller treats this as "freeform recommended").
func Score(answers map[string][]string, structures []Structure) []StructureScore
```

No AI involved. Pure deterministic Go math — fast, testable, free.

Threshold rules:
- Primary: highest score, minimum 6 points
- Secondary ("borrowed"): any structure within 70–80% of the top score
- No qualifiers → result is empty → UI shows "Freeform recommended"

---

## Guide wizard integration (B5 — Step 3.5)

The structure step is **optional and skippable** inside the novel guide. It sits between
World Basics (Step 3) and Chapter Outline (Step 4).

### Step 3.5 — Story Structure (optional)

```
┌─────────────────────────────────────────────────────────────┐
│  Story Structure                                 (optional)  │
│                                                             │
│  You can pick a story template, answer a few questions to   │
│  get a suggestion, or skip this entirely and write freeform.│
│                                                             │
│  ○  Answer questions → get a recommendation                 │
│  ○  Browse and pick a template                              │
│  ○  Freeform — I'll define my own structure                 │
│  ○  Skip — no structure                                     │
│                                                             │
│  [Skip this step]           [Continue →]                    │
└─────────────────────────────────────────────────────────────┘
```

**"Skip — no structure"** and **"Skip this step"** both advance without calling any API.
`guide_steps` gets a row with `step_key: 'structure'`, `completed_at: null`, `data: {}`.

**"Freeform"** path:
- Writer optionally enters: how many acts or phases, what marks the midpoint, what changes by
  the end, any rules the AI should follow or avoid
- On complete: `PUT /projects/:id/structure` with `{ structure_id: null, structure_custom: {...} }`
- The AI uses `structure_custom` as a plain-language system prompt addendum

**"Answer questions"** path:
- 10-question wizard rendered step-by-step (checkboxes, skip each question allowed)
- Writer clicks "Get recommendation" → `POST /guide/structure/score`
- Result card shows primary recommendation + any secondary suggestions
- Writer can: **Use this**, **Choose a different one**, or **Continue without structure**

**"Browse templates"** path:
- Grid of cards, one per structure from `GET /novel-structures`
- Each card shows name, ideal genre, core principle; click to expand strengths/risks
- Writer clicks "Use this structure" → selected, advance to Step 4
- "None of these fit" → falls through to freeform or skip

**On complete with a selection:**
- `PUT /projects/:id/structure` with `{ structure_id: <uuid> }`
- Step 4 (Chapter Outline) pre-populates act count and phase names from the selected structure
  (writer can still edit freely)

---

## AI context window integration (B2 extension)

`BuildContext` checks `projects.structure_id` and `projects.structure_custom` **only when set**.
When neither is set, context is built identically to the no-structure case — no difference.

When a structure is selected, a single optional system message is prepended (after the main
system message, before chapter summaries):

```
Story structure: {structure_name}
Current phase: {phase_name_matching_current_act}
Next expected beat: {next_phase_name}
```

Phase is matched by `acts.sort_order` → `novel_structures.phases` array index.
If the match fails (writer has more acts than phases, or modified act names), this block is
silently omitted — no error surfaced to the user.

For freeform (`structure_custom`):
```
Story rules: {structure_custom.rules joined as bullet points}
```

Only injected if `structure_custom.rules` is non-empty. Writer controls whether the AI knows
about their custom rules at all.

---

## Frontend: structure badge on ProjectHome

When `structure_id` is set: a small badge "Hero's Journey" (or chosen structure name) appears
on ProjectHome below the project title. Clicking it opens the structure step from the guide,
allowing the writer to change or clear the selection at any time.

When no structure is set: no badge, no prompt — the UI is silent about it.

---

## Timeline view integration

The wiki Timeline tab already shows `wiki_timeline_events` grouped by era. When a structure is
selected, phase labels are overlaid as section banners between act groupings — letting the writer
see how their chronological world events map onto their story's structural shape.

### Mapping logic

Acts have `sort_order` (1, 2, 3…). Structure phases are an ordered array on
`novel_structures.phases[0]`, `[1]`, `[2]`… The join is index-based:

```
Act sort_order 1  →  phases[0]  (e.g. "Act I — Setup")
Act sort_order 2  →  phases[1]  (e.g. "Act II — Confrontation: Trials, Allies, Enemies")
Act sort_order 3  →  phases[2]  (e.g. "Act III — Resolution")
```

Timeline events are already associated with chapters → acts via the hierarchy. The timeline
view groups events under their act, then shows the structure phase name as a sub-header above
that act's events.

### Behaviour rules

- **No structure selected:** timeline renders exactly as today — act names only, no phase labels.
  The UI adds nothing and removes nothing.
- **Freeform structure:** `structure_custom` may contain phase/act names defined by the writer.
  If present, those names appear instead of the act's default title. If absent, act titles render
  as-is.
- **More acts than phases:** extra acts show no phase label — silently ignored.
- **Phase label is non-interactive:** it is display-only. Writers do not drag events between
  phases; events stay attached to their chapters/acts as normal.

### Visual treatment

```
Timeline
│
├── ── Act I ─────────────────────────────────────────────
│   ╔ Hero's Journey — Call to Adventure / Crossing the Threshold ╗   ← phase banner (muted, italic)
│   │
│   ├─ [Year 412 AE] The Shattered Accord signed
│   ├─ [Year 415 AE] Navigator Kael born
│
├── ── Act II ────────────────────────────────────────────
│   ╔ Hero's Journey — Trials, Allies, Enemies                    ╗
│   │
│   ├─ [Year 433 AE] The Dying Stars first observed
│   ├─ [Year 434 AE] Kael expelled from the Academy
```

Phase banner: small, muted, italic text — clearly secondary to the timeline events themselves.
Does not appear at all when no structure is set.

### `GET /projects/:id/structure` response extension

The existing `ProjectStructureResponse` already returns `phases`. The frontend timeline view
fetches this alongside `GET /projects/:id/wiki/timeline` and performs the index join client-side.
No new backend route needed.

---

## OpenAPI schemas

```yaml
NovelStructureResponse:
  type: object
  required: [id, name, description, phases, strengths, risks]
  properties:
    id:          { type: string, format: uuid }
    name:        { type: string }
    description: { type: string }
    phases:      { type: array, items: { type: object } }
    strengths:   { type: string }
    risks:       { type: string }

StructureScoreRequest:
  type: object
  required: [answers]
  properties:
    answers: { type: object, additionalProperties: { type: array, items: { type: string } } }

StructureScoreResponse:
  type: object
  required: [ranked]
  properties:
    ranked:
      type: array
      items:
        type: object
        required: [structure_id, name, score, is_secondary]
        properties:
          structure_id: { type: string, format: uuid }
          name:         { type: string }
          score:        { type: integer }
          is_secondary: { type: boolean }

ProjectStructureResponse:
  type: object
  properties:
    structure_id:   { type: string, format: uuid, nullable: true }
    structure_name: { type: string, nullable: true }
    phases:         { type: array, items: { type: object }, nullable: true }
    structure_custom: { type: object, nullable: true }
```

---

## Checklist

- [x] **B5.5.1** Migration 015: `novel_structures` table + seed with 12 templates; `projects.structure_id` (nullable FK) + `projects.structure_custom` (nullable JSONB)
- [x] **B5.5.2** sqlc: `ListNovelStructures`, `GetNovelStructure`, `GetProjectStructure`, `UpdateProjectStructure`; regenerate
- [x] **B5.5.3** Scoring function: `internal/guide/score.go` — deterministic matrix; unit tested; returns empty slice when no structure clears threshold
- [x] **B5.5.4** Routes: `GET /novel-structures`, `POST /projects/:id/guide/structure/score`, `GET/PUT /projects/:id/structure`; all behind `RequireAuth` except `GET /novel-structures`
- [x] **B5.5.5** Guide Step 3.5 frontend: 4-path chooser (questionnaire / browse / freeform / skip); questionnaire → score call → result card with "Use this / Choose different / Continue without"; browse grid with expand cards
- [x] **B5.5.6** `BuildContext` (B2 extension): inject structure phase context when set; silently omit when structure absent or phase match fails
- [x] **B5.5.7** OpenAPI: all structure schemas; regenerate types
- [x] **B5.5.8** Frontend: structure badge on ProjectHome (only when selected); clicking re-opens structure step
- [x] **B5.5.9** Frontend: Timeline tab in WikiHub — when a structure is selected, render phase banners (muted, italic) above each era group using index-based mapping (`era group index → phases[n]`); no banner when no structure set; display-only; client-side join using `GET /projects/:id/structure` phases + existing timeline data
