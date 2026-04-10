# Phase B — Novel Guide Sub-spec: 5-Step Wizard

## Overview

The novel guide is a one-time onboarding wizard for new projects. It walks the writer through the five decisions every novel needs before the first sentence: premise, characters, world, structure, and opening. Each completed step writes real data into the project — no throwaway answers.

The guide is **skippable at any step** and **resumes** from where the writer left off. Writers who skip the guide entirely can access it later from ProjectHome.

---

## Migration 013

```sql
CREATE TABLE guide_steps (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id   UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    step_key     TEXT NOT NULL,              -- 'premise' | 'characters' | 'world' | 'outline' | 'first_scene'
    data         JSONB NOT NULL DEFAULT '{}',
    completed_at TIMESTAMPTZ,
    UNIQUE (project_id, step_key)
);
```

---

## Steps

### Step 1 — Premise
**Form fields:**
- Logline (1–2 sentences, required)
- Genre(s) — pre-filled from project genres
- Themes (free text, optional)
- Tone: dark / hopeful / action / literary / humorous

**Side effects on complete:**
- `UPDATE projects SET description = logline WHERE id = $1`
- No new entities created

**Example logline:** *"A disgraced navigator discovers the stars are dying — and she's the only one who knows why."*

---

### Step 2 — Core Characters
**Form fields:**
- Character name + role (protagonist / antagonist / ally / neutral) — add up to 5
- One-line description per character

**Side effects on complete:**
- `INSERT INTO wiki_entities (project_id, type='character', name, summary)` for each character
- Optionally: create a `wiki_relationship` between protagonist and antagonist ("opposes")

---

### Step 3 — World Basics
**Form fields:**
- Setting name (e.g. "The Shattered Reach")
- Setting description (1 paragraph)
- 1–3 factions or organisations (name + one-liner)
- 1–3 key locations (name + one-liner)
- One core rule of the world (magic / technology / politics — optional)

**Side effects on complete:**
- Create `wiki_entity` of type `location` for setting + each location
- Create `wiki_entity` of type `faction` for each faction
- Create `wiki_magic_rule` if rule provided

---

### Step 4 — Chapter Outline
**Form fields:**
- Number of acts: 1 / 2 / 3 (default 3)
- Chapter titles + one-line summary for each (dynamic list; minimum 1)
- Optional: AI "suggest structure" button → calls `/ai/complete` with premise + characters to generate a suggested chapter list

**Side effects on complete:**
- Create acts if >1 selected (rename default "Act 1" if needed; add Act 2, Act 3)
- Create chapters with titles and summaries in order

---

### Step 5 — First Scene
**Form fields:**
- First scene title
- Opening line (the writer types it, or leaves blank for AI assist)
- "Write my opening paragraph" toggle — if enabled, calls `/ai/complete` with premise + opening line as seed

**Side effects on complete:**
- Create scene in first chapter
- If AI assist: pre-fill scene content with AI-generated opening paragraph
- Mark all 5 guide steps as complete; set `completed_at`

---

## Backend routes

```
GET  /projects/:id/guide
→ { steps: [{ step_key, data, completed_at }] }

POST /projects/:id/guide/:stepKey
Body: { data: { ...step-specific fields } }
→ { step_key, data, completed_at }
  (triggers side effects synchronously)
```

---

## OpenAPI schemas

```yaml
GuideStepResponse:
  type: object
  required: [step_key, data, completed]
  properties:
    step_key:     { type: string }
    data:         { type: object }
    completed:    { type: boolean }
    completed_at: { type: string, format: date-time }

GuideStateResponse:
  type: object
  required: [steps, all_complete]
  properties:
    steps:        { type: array, items: { $ref: '#/components/schemas/GuideStepResponse' } }
    all_complete: { type: boolean }
```

---

## Frontend: wizard UI

### Route
`/projects/:id/guide`

### Layout
```
┌─────────────────────────────────────────────────────┐
│  ← Back to project        NexusTale — Novel Guide   │
├──────────────┬──────────────────────────────────────┤
│              │                                      │
│  Step list   │   Step form                          │
│  (sidebar)   │                                      │
│              │   [Title]                            │
│  1 Premise ✓ │   [Description]                      │
│  2 Characters│                                      │
│  3 World     │   [Field 1]                          │
│  4 Outline   │   [Field 2]                          │
│  5 Scene     │                                      │
│              │   [Skip step]    [Complete step →]   │
└──────────────┴──────────────────────────────────────┘
```

### Behaviour
- Sidebar shows which steps are complete (✓), current (highlighted), and upcoming (muted)
- "Complete step" → `POST /guide/:stepKey` → on success, advance to next step
- "Skip step" → advance without calling the API (step stays incomplete)
- "Finish guide" appears after step 5 (or any step if all previous skipped) → navigate to `/projects/:id`
- If guide already complete, redirect immediately to `/projects/:id` with a "You already finished the guide" toast
- Resumable: `GET /guide` on mount → find first incomplete step → start there

### AI assist (step 4 + 5)
- "Suggest structure" / "Write opening paragraph" buttons call AI endpoints
- Show spinner while waiting; result populates form field (editable before confirming)
- AI assist is best-effort: if no key stored, show "Add an API key in Settings to use AI assist" inline

---

## Entry point from ProjectHome

Add a "Start guide" card on ProjectHome when `guide.all_complete === false`:

```
┌─────────────────────────────────────────┐
│  📖  Novel Guide                        │
│  Walk through 5 steps to scaffold your  │
│  story structure, characters, and world.│
│  [Continue guide →]                     │
└─────────────────────────────────────────┘
```

Card disappears once all steps complete. Can always be re-accessed via `/projects/:id/guide`.
