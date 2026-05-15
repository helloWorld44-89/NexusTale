# NexusTale — AI & RAG Improvement Plan

> **Source:** Synthesized from three independent model reviews (o3, GPT-4o, Gemini 2.5 Pro) of `AI_PROMPT_ENGINEERING_ANALYSIS.md`. Each recommendation below is supported by at least two of three graders unless marked `[unique]`. Items are ordered by impact-to-effort ratio within each phase.

---

## Consensus Scores (averaged across graders, normalized to 1–5)

| Call | Prompt Clarity | Task Specificity | Context Relevance | Context Efficiency | Output Constraints | Edge Handling | **Avg** |
|------|---------------|-----------------|------------------|-------------------|------------------|--------------|---------|
| Beat | 3.3 | 2.3 | 3.5 | 2.2 | 2.2 | 2.2 | **2.6** |
| Continue | 2.8 | 2.3 | 3.5 | 2.4 | 1.8 | 2.2 | **2.5** |
| Summarize | 3.2 | 2.6 | 2.0 | 2.2 | 3.2 | 1.6 | **2.5** |
| Regular Chat | 3.5 | 3.0 | 2.8 | 2.2 | 2.2 | 2.4 | **2.7** |
| Workshop Chat | 4.6 | 4.6 | 3.4 | 3.2 | 3.6 | 3.2 | **3.8** |
| Workshop Agent | 3.8 | 3.2 | 3.5 | 2.2 | 2.2 | 2.8 | **3.0** |
| RAG (cross-cutting) | — | — | 2.4 | 1.8 | — | 2.0 | **2.1** |
| Model Routing | — | — | — | 2.6 | — | 2.6 | **2.6** |

**Overall system grade: ~2.9 / 5** (o3 expressed this as 8.1/10 on a more generous scale, but is grading relative to commercial writing apps, not absolute best practice.)

### Grader agreements

All three graders independently flagged the same five root issues, in this priority order:

1. **Brute-force RAG** — all context injected regardless of relevance; no pruning policy
2. **Declarative craft prompting** — prompts say *what* to do, not *how*; no behavioral constraints or anti-patterns
3. **Summarization architecture is structurally broken** — no scene separators, no position context, 200-token cap too tight, no retry
4. **No task-specific model routing** — haiku is the default for prose generation tasks
5. **Agent has no planning phase** — jumps to tool calls without a plan

---

## Phase 1 — Prompt Text Fixes (no DB migrations, no new dependencies)

These are edits to `service.go`, `workshop_handler.go`, and `context.go`. They change only string construction — the fastest possible improvement.

**Expected outcome:** Immediate improvement to Beat/Continue output quality; elimination of most repetition artifacts and meta-narration.

---

### 1.1 Beat — Behavioral craft constraints + user-turn framing

**Current system prompt craft line:**
```
Match the author's tone and style. Use sensory details. Show, don't tell.
```

**Replace with behavioral constraints** (o3: "declarative → behavioral prompting"):
```
Write immersive prose in the scene's established voice. Strict rules:
- Do NOT repeat, rephrase, or echo any sentence from ## Scene ending
- Do NOT produce meta-narration ("In this scene, Kira will...")
- Do NOT summarize forward ("Over the next few days...")
- Render emotion through physical sensation, gesture, interrupted thought, or dialogue — not abstract statement ("she felt nervous")
- Match sentence rhythm of the ## Scene ending excerpt
- Avoid: adverbs modifying weak verbs, "suddenly", "realized that", "could see/hear/feel"
```

**Replace paragraph count:**

From: `"write 2–3 paragraphs"`

To: `"write approximately 2–3 paragraphs — expand if the beat requires it, compress if it resolves in one"`

**Add explicit user-turn framing** (all three graders agree):

Change `adapterReq.Content = req.BeatText` to:
```
Expand the following story beat into prose, continuing directly from ## Scene ending:

"<BEAT TEXT>"
```

**File:** `backend/internal/ai/service.go` — `beatSystemPrompt()` function + user-turn assembly in `StreamComplete`

---

### 1.2 Continue — Narrative phase awareness + user-turn framing

**Current system prompt close:**
```
Continue the story naturally from where it left off.
```

**Replace with:**
```
Continue the story from where ## Earlier in this scene ends.

Rules:
- Do NOT repeat or rephrase any text from the user turn
- Do NOT summarize forward ("Later that night...")
- Match sentence length and rhythm of the preceding prose
- Stay strictly in established tense and POV
- Write the present moment — do not skip ahead
```

**Add narrative phase field** `[unique: o3]` — extend `CompleteRequest` with optional `NarrativePhase string` enum (`escalation | reflection | confrontation | discovery | aftermath | transition`). When set, append to system prompt:

```
## Narrative function
This continuation is: <PHASE>. Write accordingly — [one-sentence elaboration per phase].
```

This is a frontend dropdown in the Continue toolbar (no migration needed — request-only field).

**Add user-turn framing:**

Change so the user turn begins with:
```
Continue the story from the following text:

<LAST ~800 TOKENS>

[Instruction: <OPTIONAL>]
```

**File:** `backend/internal/ai/service.go` — `continueSystemPrompt()` + user-turn assembly

---

### 1.3 Workshop — Swap prompt order + increase digest limit

**Current order:** `PHASE_DIRECTIVE + "\n\n" + BASE_IDENTITY`

**Correct order** (all three graders agree; o3: "identity before specialization"):
```go
return base + "\n\n" + directive
```

This means the model reads "You are Nexus in Workshop mode..." first, then the phase-specific lens. Less persona confusion.

**Increase digest per-turn limit** from 200 runes to 600 runes:

```go
const workshopDigestMaxRunes = 600  // was 200
```

At 200 runes (~50 tokens), a single paragraph of craft feedback is truncated. At 600 runes (~150 tokens), full multi-sentence feedback survives.

**File:** `backend/internal/ai/workshop_handler.go` — `workshopSystemPrompt()` + `applyWorkshopHistoryWindow()`

---

### 1.4 Workshop phases — PRIMARY / SECONDARY / FAILURE structure

**Current `story_pass` directive opens with:**
```
You are a developmental editor focused on structural integrity. For any scene or chapter discussed: (1)...
```

**Upgrade to structured format** `[unique: o3]` for all four phases:

```
PRIMARY OBJECTIVE: <one sharp sentence on the main editorial lens>
SECONDARY OBJECTIVE: <what to watch for beyond the primary>
FAILURE CONDITIONS: <output that would be wrong — e.g. "do not give generic encouragement", "do not rewrite prose unless asked">
```

Example for `story_pass`:
```
PRIMARY OBJECTIVE: Identify scenes that do not advance character, plot, or world — and explain why.
SECONDARY OBJECTIVE: Surface unfulfilled reader promises and pacing failures (scenes that rush or linger past their landing point).
FAILURE CONDITIONS: Do not offer general praise. Do not suggest rewrites. Reference specific open story threads from the context when relevant.
```

**File:** `backend/internal/ai/workshop_handler.go` — `workshopSystemForPhase()`

---

### 1.5 Agent — Planning phase + append-vs-replace guidance

**Add to agent system prompt** (all three graders agree):

```
BEFORE calling any tools:
1. State your plan in plain text: what you will create or modify, in what order, and why.
2. List the specific scene/chapter IDs you will target (call list_project_structure first if unsure).
3. Only then call tools.

Tool selection rules:
- Use append_to_scene when adding new content after existing prose.
- Use replace_scene_content ONLY when the writer explicitly asks to rewrite or replace. Never shorten a scene when replacing — reproduce existing content if it should be kept.
```

**File:** `backend/internal/ai/service.go` — agent system prompt in `StreamChatWithTools()`

---

## Phase 2 — Context Efficiency (code changes, no DB migrations)

These changes are to `context.go` and `service.go`. They reduce token overhead significantly without changing any stored data.

**Expected outcome:** 40–60% reduction in context tokens for mid-to-large projects; faster calls; lower cost.

---

### 2.1 Hard context budget with priority drop policy

Replace the warn-only `contextBudgetWarnChars = 20_000` with an enforced budget.

**Budget:** `maxContextChars = 32_000` (~8,000 tokens — fits comfortably in any provider's context window alongside a full system prompt and output). Adjust per call type:

| Call | Budget |
|------|--------|
| Beat | 24,000 chars (~6,000 tokens) |
| Continue | 24,000 chars |
| Chat | 32,000 chars |
| Workshop | 32,000 chars |
| Summarize | (no context budget — uses raw content only) |

**Drop order when budget is exceeded** (all three graders agree on this hierarchy):

```
ALWAYS KEEP (never drop):
  1. Current scene tail / head excerpt
  2. Scene directive (## This scene)
  3. Entities directly mentioned in current scene

DROP FIRST (least relevant):
  4. Chapter summaries older than N chapters back (keep last 5 + first 1 as anchor)
  5. Open story threads not linked to current scene entities
  6. Low-priority entity mentions (items, concepts — not characters/locations)
  7. Stale pinned context (pins older than current session)
```

**Implementation:** `BuildContext` returns a `[]contextSection` slice with a `priority int` and `chars int` per section. A `pruneTobudget(sections []contextSection, budget int)` function drops from lowest priority until under budget, then assembles the final string.

**File:** `backend/internal/ai/context.go`

---

### 2.2 Cap and rank chapter summaries

**Current behavior:** All chapter summaries on the branch are injected.

**New behavior:**

```go
const (
    summaryRecentWindow = 5   // last N chapters before current
    summaryAnchorCount  = 1   // always include chapter 1 summary
)
```

Inject: first 1 summary (story anchor) + last 5 summaries (recent context) + current chapter summary. For a 30-chapter novel, this reduces summary injection from 30 summaries to ~7. Writers working on chapter 20 don't need summaries of chapters 3–15 for every beat.

If the writer needs earlier context, the pinned context system (pins a chapter) is the right escape hatch — that already works.

**File:** `backend/internal/ai/context.go` — `buildStorysoFarContext()`

---

### 2.3 Cap and rank entity mentions

**Current behavior:** Every indexed entity mention is injected.

**New behavior — type-priority cap:**

```go
const (
    maxCharacterEntities = 5
    maxLocationEntities  = 3
    maxOtherEntities     = 2  // factions, items, concepts combined
)
```

**Ranking within each type:** entities that appear more frequently in the current scene (by match count in `scene_entity_mentions`) rank first. Peripheral mentions of a faction name once are lower priority than a character who appears in every paragraph.

**File:** `backend/internal/ai/context.go` — `buildEntitiesInSceneContext()`

---

### 2.4 Chat/Workshop: inject scene summary instead of full scene text

**Current behavior (section 6):** Full scene text is always injected when `currentSceneID != uuid.Nil`.

**New behavior:**

- If a chapter summary exists for the chapter containing `currentSceneID`, inject the **chapter summary** as `## Current chapter` instead of the full scene text.
- Keep the full scene text injection only when the call mode is `beat` or `continue` (where it is already handled via head/tail split and section 6 is suppressed).
- Add a `## Current scene (summary)` fallback that uses the first 400 runes of the scene if no summary exists yet.

The writer who wants to discuss specific scene prose with Chat/Workshop can paste it into the message — the model doesn't need it pre-loaded in the system prompt.

**File:** `backend/internal/ai/context.go` — `buildCurrentSceneContext()`; `service.go` — pass a `chatMode bool` to `BuildContext`

---

### 2.5 Regular Chat: add digest compression

**Current behavior:** History is silently truncated at 12 turns (oldest messages dropped).

**Fix:** Apply the same `applyWorkshopHistoryWindow` logic to `StreamChat` (the Regular Chat path). Use the same 600-rune-per-turn digest limit from Phase 1.3.

**File:** `backend/internal/ai/service.go` — `StreamChat()` history assembly

---

### 2.6 Agent: pass only relevant tools per intent

**Current behavior:** All 10 tools are passed on every round.

**New behavior — intent classification:**

```go
func classifyToolIntent(userMsg string) toolSet {
    // keyword match — no ML needed
    hasWikiIntent  := containsAny(userMsg, "wiki", "character", "entity", "relationship", "faction")
    hasWriteIntent := containsAny(userMsg, "write", "scene", "chapter", "append", "add", "create", "draft")
    
    switch {
    case hasWikiIntent && !hasWriteIntent:
        return toolSetWikiOnly   // list_wiki_entities, create_wiki_entity, update_wiki_entity, create_wiki_relationship
    case hasWriteIntent && !hasWikiIntent:
        return toolSetWriteOnly  // list_project_structure, append_to_scene, replace_scene_content, create_scene, create_chapter, create_act
    default:
        return toolSetAll
    }
}
```

Tool definitions add ~800–1,000 tokens per round. Filtering to 4–6 tools when the intent is clear saves ~400–500 tokens per round × up to 25 rounds.

**File:** `backend/internal/ai/service.go` — `StreamChatWithTools()`

---

## Phase 3 — Summarization Architecture Overhaul

The summarization pipeline is the weakest component. All three graders flagged it. This phase upgrades it from "prose summary" to "narrative state tracking."

**Expected outcome:** Better BuildContext `## Story so far` quality → better Beat/Continue output because the model has an accurate understanding of where the story is.

---

### 3.1 Multi-layer summary format

**Current prompt produces:** "Kira confronts the warden and learns her brother may be alive."

**New prompt produces structured output** `[unique: o3, validated by all three graders]`:

New system prompt:
```
You are a narrative memory system for a novel-writing tool. Summarize the following chapter content in a structured format.

Chapter: "<CHAPTER TITLE>" (Chapter <N> of <TOTAL>)

Produce exactly this format:
EVENTS: <2–3 bullet points of key plot actions>
CHANGES: <1–2 bullet points of what shifted — character state, relationship, world-state, or revealed information>
PRESSURE: <1 sentence on what narrative tension now exists or intensifies going forward>

Rules:
- Be specific: name characters, not "someone"
- Do not use "this chapter shows..." or "in this chapter..."
- PRESSURE must point forward, not summarize what happened
```

Example output:
```
EVENTS:
- Kira forces her way into the warden's records room and finds a transfer document bearing her brother's name
- The warden discovers her and gives her 48 hours to leave the station before alerting the Council

CHANGES:
- Kira shifts from grief to obsessive urgency; her relationship with the warden is now adversarial
- The reader now knows the brother is alive but the destination is redacted

PRESSURE: Kira has 48 hours and a partial lead — but acting will expose her to the Council
```

This format is:
- Compact (~250 tokens max)
- Useful for `## Story so far` injection (the EVENTS/CHANGES/PRESSURE labels are scannable)
- Useful for open-thread tracking (PRESSURE feeds directly into the story threads system)

**New token cap:** 350 tokens (up from 200). For single-scene chapters, 200 is fine; for multi-scene, 350 allows the full three-section format without truncation. Make this dynamic: `min(scenes*120, 350)`.

**File:** `backend/internal/ai/service.go` — `summarizeSystemPrompt()` + `Summarize()` MaxTokens

---

### 3.2 Inject chapter title and position into summarize call

The model currently summarizes blind — it doesn't know if this is chapter 2 of a 30-chapter novel or chapter 29.

**Add to user turn:**
```go
header := fmt.Sprintf("Chapter %d of %d: \"%s\"\n\n", chapterIndex+1, totalChapters, chapterTitle)
combined = header + combined
```

Requires: `GetChapterWithPosition(chapterID)` query — join chapters with a `ROW_NUMBER()` window function ordered by position.

**File:** `backend/internal/ai/context.go` — `regenerateSummary()`

---

### 3.3 Scene separators in summarize input

Change concatenation from `"\n\n"` to `"\n\n---\n\n"` between scenes. Add scene title as a label:

```go
fmt.Fprintf(&sb, "## Scene: %s\n\n%s", sc.Title, content)
if i < len(scenes)-1 {
    sb.WriteString("\n\n---\n\n")
}
```

**File:** `backend/internal/ai/context.go` — `regenerateSummary()` or `service.go:Summarize()`

---

### 3.4 Basic retry + validation on summarize output

```go
func isValidSummary(s string) bool {
    s = strings.TrimSpace(s)
    if len(s) < 30 { return false }                          // too short
    if strings.HasPrefix(s, "In this chapter") { return false }  // over-narrated
    if strings.HasPrefix(s, "This chapter") { return false }
    return true
}

// In regenerateSummary:
for attempt := 0; attempt < 2; attempt++ {
    summary, err = adapter.Summarize(ctx, req)
    if err == nil && isValidSummary(summary) { break }
}
```

**File:** `backend/internal/ai/context.go` — `regenerateSummary()`

---

## Phase 4 — Task-Specific Model Routing

Simple but high ROI. All three graders flagged this. No architectural change — just a routing function.

**Expected outcome:** 40–70% cost reduction on background tasks; better prose quality on generation tasks (if writer uses haiku as default, beat/continue get upgraded automatically).

---

### 4.1 Task-tier model selection

Add a `taskTier` concept to the adapter resolution:

```go
type taskTier int

const (
    tierBackground taskTier = iota  // summarize, tool structure reads
    tierAnalysis                    // chat, workshop, agent planning rounds
    tierCreative                    // beat, continue
)
```

```go
func (s *Service) getAdapterForTier(ctx context.Context, userID uuid.UUID, requestedProvider string, tier taskTier) (adapters.Adapter, error) {
    // If user explicitly specified a provider, respect it
    if requestedProvider != "" {
        return s.getAdapter(ctx, userID, requestedProvider)
    }
    
    // Otherwise, route by tier using user's stored keys
    // tierBackground → prefer haiku/flash/mini
    // tierCreative   → prefer sonnet/gpt-4o
    // tierAnalysis   → prefer sonnet or mid-tier
}
```

**Routing table:**

| Task | Tier | Preferred Anthropic | Preferred OpenAI |
|------|------|--------------------|--------------------|
| Chapter summarize | Background | claude-haiku-4-5 | gpt-4o-mini |
| Regular Chat | Analysis | claude-sonnet-4-6 | gpt-4o |
| Workshop Chat | Analysis | claude-sonnet-4-6 | gpt-4o |
| Beat completion | Creative | claude-sonnet-4-6 | gpt-4o |
| Continue completion | Creative | claude-sonnet-4-6 | gpt-4o |
| Agent tool rounds | Analysis | claude-sonnet-4-6 | gpt-4o |
| Agent final text | Creative | claude-sonnet-4-6 | gpt-4o |

Writers can override at the request level (existing `Provider` field). The tier is the default when no override is set.

**File:** `backend/internal/ai/service.go` — new `getAdapterForTier()` + call sites updated

---

## Phase 5 — Style Fingerprinting

The highest-value improvement for prose quality that no other commercial writing app does well. All three graders noted NexusTale tracks lore but not prose DNA.

**Expected outcome:** Beat/Continue output that sounds like the writer's voice without any manual style preset configuration.

---

### 5.1 Prose fingerprint extraction

Add a background job that runs after summarization on chapter save. Reads the scene content and extracts measurable prose statistics:

```go
type ProseFingerprint struct {
    AvgSentenceLength    float32  // words per sentence
    AvgParagraphLength   float32  // sentences per paragraph
    DialogueRatio        float32  // 0.0–1.0 (fraction of lines that are dialogue)
    IntoriorityFrequency float32  // "thought", "wondered", "felt", "knew" per 1000 words
    AdverbDensity        float32  // words ending in "-ly" per 1000 words
    SentenceVariance     float32  // std dev of sentence lengths (rhythm indicator)
}
```

Store as JSONB in a new `projects.prose_fingerprint` column (migration 036). Recompute after every 3 scene saves (not every save — the fingerprint only needs approximate accuracy).

**Inject as a section in Beat/Continue system prompt:**

```
## Author's prose style
- Sentence length: short-medium (~12 words avg)
- Paragraph rhythm: tight (2–3 sentences avg)
- Dialogue: moderate (35% of prose)
- Interiority: high (frequent thought rendering)
- Adverb usage: sparse
Match this style exactly. Do not write long flowing sentences if the style is short and punchy.
```

**File:**
- Migration 036: `projects.prose_fingerprint JSONB`
- New `backend/internal/ai/fingerprint.go`: extraction function (pure string analysis, no ML)
- `context.go`: `buildStyleFingerprintContext()` called from `StreamComplete` for Beat/Continue only

---

### 5.2 Fingerprint-aware paragraph count

Use `AvgParagraphLength` from the fingerprint to dynamically set the paragraph count in Beat:

```go
paragraphHint := "approximately 2–3 paragraphs"
if fp.AvgParagraphLength < 2.0 {
    paragraphHint = "approximately 3–5 short paragraphs"
} else if fp.AvgParagraphLength > 4.0 {
    paragraphHint = "approximately 1–2 longer paragraphs"
}
```

---

## Phase 6 — Regular Chat Mode Specialization

`[unique: o3, validated by GPT-4o]` Regular Chat is currently a catch-all that serves four different use cases with a single generic identity. Each use case should get a tailored context profile.

---

### 6.1 Chat modes

Extend `ChatRequest` with an optional `Mode string` field:

| Mode | Identity emphasis | Context injected | History compression |
|------|------------------|-----------------|---------------------|
| `brainstorm` | Creative partner; suggest multiple options; be generative | Summaries + threads only (no full scene) | Digest |
| `editorial` | Structural editor lens; be specific and critical | Full BuildContext | Digest |
| `lore` | Wiki oracle; answer accurately from known entities | Entities + summaries only | Sliding window |
| (default/empty) | Current Nexus identity | Full BuildContext | Digest (after Phase 2.5) |

`brainstorm` mode should also explicitly include: "Suggest 2–3 alternatives when proposing a direction. Do not converge on a single answer unless the writer asks for it."

**File:** `backend/internal/ai/handler.go` + `service.go` — add mode routing in `StreamChat()`

---

## Phase 7 — Semantic Retrieval (Embedding-Based RAG)

All three graders flagged this as the most impactful architectural upgrade for scaling. This is the largest change and should be tackled last, after Phases 1–4 have stabilized the token budget.

**Expected outcome:** 60–80% reduction in context tokens; elimination of "needle in a haystack" attention dilution; project-size-independent context quality.

---

### 7.1 What to embed

| Item | When embedded | Index key |
|------|---------------|-----------|
| Chapter summaries | After Phase 3 summarize runs | `(project_id, chapter_id, branch_name)` |
| Entity descriptions | On entity create/update | `(project_id, entity_id)` |
| Research notes | On note create/update | `(project_id, note_id)` |

**Do NOT embed:** Scene full text (too expensive + too long), magic rules (always inject all ≤5), story structure (always inject), current scene tail (always inject via head/tail split).

### 7.2 Storage

Add `pgvector` extension to PostgreSQL:

```sql
-- migration 037
CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE chapter_summaries ADD COLUMN embedding vector(1536);
ALTER TABLE wiki_entities      ADD COLUMN embedding vector(1536);
ALTER TABLE research_notes     ADD COLUMN embedding vector(1536);
```

Use `text-embedding-3-small` (OpenAI) or `voyage-lite-02-instruct` (Anthropic) — both are cheap and fast. For Ollama users, fall back to `nomic-embed-text`.

### 7.3 Retrieval at call time

Replace brute-force summary injection with:

```go
// In BuildContext, for ## Story so far:
queryEmbedding := embed(currentBeatText + sceneDirective)
relevantSummaries := pgvector.Query(
    "SELECT chapter_id, summary_text FROM chapter_summaries 
     WHERE project_id = $1 AND branch_name = $2
     ORDER BY embedding <=> $3 LIMIT 5",
    projectID, branch, queryEmbedding,
)
// Always also inject chapter 1 (anchor) and current chapter
```

Same pattern for entity context: retrieve top-5 most semantically relevant entities by embedding similarity to the beat text, instead of all mentions.

### 7.4 Embedding pipeline

- Trigger embedding recompute asynchronously after summarize completes (same debounce pattern as `ScheduleSummarize`)
- Store `embedding_updated_at` so stale embeddings can be detected
- On first run (no embeddings yet), fall back to current brute-force injection — no regression

**File:**
- Migration 037 (pgvector extension + columns)
- New `backend/pkg/embedding/` package — provider-agnostic embed function
- `backend/internal/ai/context.go` — replace `buildStorySoFarContext()` and `buildEntitiesInSceneContext()` with vector retrieval versions
- `backend/cmd/api/main.go` — wire embedding client

---

## Implementation Order & Dependencies

```
Phase 1 (prompt text)      → No dependencies. Do all of 1.1–1.5 in one PR.
Phase 2 (context budget)   → Depends on nothing. Can parallel with Phase 3.
Phase 3 (summarize)        → Depends on nothing. One PR for 3.1–3.4.
Phase 4 (model routing)    → Depends on nothing. Small PR.
Phase 5 (fingerprinting)   → Depends on Phase 3 being stable (summaries must be reliable first).
Phase 6 (chat modes)       → Depends on Phase 2 (context efficiency needed for mode-specific profiles).
Phase 7 (semantic RAG)     → Depends on Phase 3 (must have good summaries to embed) + Phase 2 (budget policy still needed as fallback).
```

### Suggested sprint order

| Sprint | Phases | Effort | Expected gain |
|--------|--------|--------|--------------|
| 1 | 1.1, 1.2, 1.3 | 1–2 days | Immediate Beat/Continue quality improvement |
| 2 | 1.4, 1.5, 2.5, 2.6 | 1–2 days | Workshop/Agent reliability |
| 3 | 3.1–3.4, 4.1 | 2–3 days | Summary quality + cost reduction |
| 4 | 2.1–2.4 | 2–3 days | Token budget enforcement |
| 5 | 5.1–5.2 | 3–4 days | Style fingerprinting (new feature) |
| 6 | 6.1 | 1 day | Chat mode specialization |
| 7 | 7.1–7.4 | 1–2 weeks | Semantic retrieval (major arch change) |

---

## Key Principles (from grader consensus)

> **Relevance > Volume** — a smaller, well-chosen context outperforms a large noisy one every time.

> **Constraints > Suggestions** — "do not repeat the scene tail" eliminates a failure mode; "match the tone" does not.

> **Planning > Reactive Generation** — agents that plan before acting produce coherent multi-step outputs; agents that react produce fragmented ones.

> **Behavioral prompting > Declarative prompting** — show the model what bad output looks like, not just what good output is.
