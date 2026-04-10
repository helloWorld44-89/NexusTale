# Writingway2 — Prompt Engineering Analysis for NexusTale Phase B

## What Writingway2 Is

Writingway2 is a single-HTML-file, local-first novel writing tool. It bundles llama.cpp's server
(`llama-server.exe`) for zero-install local AI. All data lives in IndexedDB (Dexie). It has no
backend — every AI call goes from the browser directly to a local or cloud API. This architecture
is the opposite of NexusTale's server-mediated model, but its UX patterns and prompt engineering
are directly applicable.

---

## Core Pattern: Beat-Driven Generation

Writingway2's key UX innovation is the **beat**. Instead of asking the AI to "continue," the
writer types a short intent sentence — "Jack discovers the hidden door is already ajar" — and the
AI expands that single beat into 2–3 paragraphs of prose.

```
Writer input:  Beat (1 sentence — what happens next)
AI output:     2–3 paragraphs of prose that bring the beat to life
```

This is superior to raw "continue" generation because:
- The writer stays in control of story direction
- AI hallucinations stay confined to prose quality, not plot
- Short inputs make re-rolling cheap; the writer just hits "retry"
- Beats can be stored as the story's structural skeleton (outline = beat list)

**NexusTale adoption:** Phase B's `/ai/complete` should support both a `beat` mode (beat → prose
expansion) and a `continue` mode (scene content → continuation). Beat mode maps cleanly to the
Guide wizard (step 4 creates chapter outlines which are essentially beats per chapter).

---

## Prompt Assembly Architecture

### Layers (innermost = highest priority)

```
1. System prompt (AI role + POV/tense/style directives)
2. Compendium entries (wiki injected by reference or panel selection)
3. Scene summaries (previous scene context — full or summarized)
4. Current scene content (what's been written so far)
5. BEAT (the user's instruction for what happens next)
```

### How compendium references work

Writers can mention wiki entries two ways:
- **Inline in the beat**: `@[Character Name]` or `@[Location]` — parsed from beat text, resolved
  to compendium body, injected into context
- **Context panel**: persistent sidebar where writer pins specific entries or entire tag groups

Both sources are merged (deduplicated by ID) before the prompt is built.

Scene summaries work the same way: `#[Scene Title]` in the beat resolves to the scene's stored
summary. The context panel can also include full scene text or summary-only per chapter.

**NexusTale adoption:** The context window builder (`internal/ai/context.go`) should support
`@[entity]` inline resolution. Phase B spec already includes wiki entity injection — extend it to
parse inline mentions from the scene text being edited (or the user's chat message).

---

## System Prompt Design

### Default system prompt (from `src/generation.js`)

> "You are a co-author helping to write a novel. Given a story beat (what should happen next),
> write 2-3 paragraphs that bring the beat to life. Match the author's tone and style.
> Use sensory details. Show, don't tell."

Key characteristics:
- Explicitly sets the **output length** (2–3 paragraphs)
- Defines **the task** (expand beat, not summarize or explain)
- Gives a **style directive** ("match author's tone")
- Gives a **craft directive** ("show, don't tell")

### System prompt template variables

The system prompt accepts `{povName}`, `{tense}`, `{pov}` placeholders. When a writer has
configured their scene metadata (POV character, POV type like "third limited", tense like
"past"), these are substituted in:

> "You are a co-author... Write in **{tense}** tense from **{pov}** point of view. The POV
> character is **{povName}**."

**NexusTale adoption:** The Phase B spec's context builder should inject scene metadata into the
system prompt template. NexusTale already stores `tense` and `pov` on scenes — use them.

---

## User-Editable Prompt Sets

Writingway2 stores prompts in IndexedDB per project with this schema:

```js
{
  id: string,
  projectId: string,
  category: 'prose' | 'system',  // prose = user-facing template; system = system prompt override
  content: string,                // user message template (prose style instructions)
  systemContent: string,          // system prompt override (replaces default)
  name: string,
  order: number
}
```

Writers can:
- Create named prompt sets ("Gritty noir style", "Epic fantasy voice")
- Import/export as JSON between projects
- Switch active prose prompt per session
- Leave system prompt blank to use the default

**NexusTale adoption:** This is the highest-value feature to add. Call it **Writing Styles** or
**Prose Prompts**. Store in a `project_prompts` table with `category`, `content`,
`system_content`. Surface in the SceneMetadataPanel as a style selector dropdown. This gives
writers per-project AI persona without touching the underlying adapter logic.

---

## Context Panel (Writer-Controlled Context)

The context panel lets writers curate exactly what goes into the context window:

- **By ID**: pin specific compendium entries
- **By tag**: include all compendium entries tagged "magic-system"
- **Per chapter**: include as full text or summary-only
- **Per scene**: same, at scene granularity

This is a **explicit** context management UI rather than the automatic "last 3 chapters"
heuristic in NexusTale's current Phase B spec. Both approaches are valid.

**Recommendation for NexusTale:** Keep the automatic context window for casual use (Phase B
spec as written). Add an explicit context panel in Phase C for power users. The two-tier approach
gives beginners a working system and gives advanced writers precise control.

---

## Workshop (Chat Mode)

Writingway2's Workshop is a persistent multi-session chat, separate from beat generation:

- Multiple named sessions per project (stored in IndexedDB)
- Each session is a `[{role, content, timestamp}]` message array
- A per-session "workshop prompt" (system prompt variant for narrative discussion)
- Export session to Markdown

The key insight: Workshop and generation share the same AI provider/model settings but use
**different system prompts**. Generation = "write prose"; Workshop = "discuss story, give
feedback, brainstorm".

**NexusTale adoption:** The `ChatBar` in Phase B maps to Workshop. The Phase B spec uses the
same route (`/ai/chat`) for both. Keep that, but ensure the system prompt for chat mode is
discussion-oriented, not prose-generation-oriented.

---

## Provider + Model Management

Writingway2 supports 7 providers: local llama-server, OpenAI, Anthropic, OpenRouter, Google,
NanoGPT, LMStudio. Key patterns:

### Thinking model detection
```js
const THINKING_MODELS = ['o1', 'o3', 'deepseek-reasoner', 'qwq', 'r1'];
if (THINKING_MODELS.some(m => modelId.includes(m))) {
    // disable streaming, don't pass system prompt
}
```

Thinking models (chain-of-thought) break SSE streaming and don't accept system prompts in the
standard position. Auto-detect by model ID substring.

**NexusTale adoption:** Add model metadata to the `AdapterFactory`. When `provider=openai` and
model contains `o1`/`o3`, use batch mode (non-streaming) and skip system prompt injection.

### Model list fetching
OpenRouter and OpenAI have `/models` APIs; Anthropic and Google use hardcoded lists. LMStudio
exposes `/v1/models` (OpenAI-compatible).

**NexusTale adoption:** The `AdapterFactory` should cache model lists per provider. For the
settings page in NexusTale (Phase A+ done), surface a model dropdown that populates from the
provider's models endpoint.

### forceNonStreaming flag
Writingway2 exposes a `forceNonStreaming` toggle for models that claim streaming but behave
badly. Non-streaming simulates streaming by splitting the response on whitespace and emitting
tokens with 10ms delays on the frontend.

**NexusTale adoption:** Add `force_non_streaming: bool` to the user's stored API key settings.
The adapter `StreamComplete` falls back to simulated streaming if set.

---

## Prompt History

Every generation saves to a `promptHistory` table:

```js
{
  projectId, sceneId, timestamp,
  beat: string,       // what the writer typed
  prompt: string      // full assembled prompt sent to AI
}
```

Writers can browse their prompt history, re-apply a previous beat, and see exactly what was sent
to the AI.

**NexusTale adoption:** Add to `ai_usage` (Phase B B3) a `prompt_preview` TEXT column storing
the first 500 chars of the assembled prompt + the user's beat/instruction. Show in the AI usage
panel. Full prompt history is a good Phase C feature.

---

## Key Differences from NexusTale's Design

| Aspect | Writingway2 | NexusTale Phase B |
|--------|-------------|-------------------|
| Context strategy | Writer-curated (explicit) | Auto (last 3 chapters + recent scenes) |
| Beat/continue UX | Beat-first (primary mode) | Continue-first (primary), beat optional |
| Prompt customization | Named prompt sets per project | Not planned yet |
| Session chat | Multi-session Workshop | Single ChatBar per project |
| Model management | Dynamic list from provider API | Static per-provider defaults |
| Thinking model support | Auto-detect, disable stream | Not planned |
| Prompt history | Full per-scene history | Token count only |
| Storage | IndexedDB (local) | PostgreSQL (server) |
| Auth | None (single user) | JWT multi-user |

---

## Recommendations for Phase B

### Add to B1 (AI proxy spec)
1. **Beat mode parameter** — `POST /ai/complete` body gains `mode: "beat" | "continue"`. Beat
   mode wraps the input in the beat-expansion system prompt. Continue mode uses the scene
   continuation prompt.

2. **Thinking model flag** — `AdapterFactory` checks model ID for `o1`, `o3`, `deepseek-reasoner`,
   `qwq`, `r1`; switches to `Complete` (batch) instead of `StreamComplete`.

### Add to B2 (AI memory spec)
3. **Inline `@[entity]` resolution** — `BuildContext` should parse `@[Entity Name]` mentions in
   the current scene text or user chat message, resolve against `wiki_entities`, and inject their
   summaries before the current content layer.

### Add as B1.5 — Writing Styles (Prose Prompts)
4. **`project_prompts` table** — `id, project_id, name, category ('prose'|'system'|'workshop'),
   content, system_content, sort_order`. Migration 014. Gives writers named prompt sets they can
   switch per session.

5. **`GET/POST/PUT/DELETE /projects/:id/prompts`** — CRUD for writing styles.

6. **Active prompt per session** — `POST /ai/complete` and `/ai/chat` accept `prompt_id?: uuid`;
   if provided, inject that prompt's `system_content` as system prompt and `content` as an
   additional user context block.

7. **Frontend**: Style selector dropdown in SceneMetadataPanel. "Default (NexusTale)" option
   always present; user-created styles listed below.

### Phase C candidates (from Writingway2 patterns)
- Explicit context panel with per-chapter/per-entity include/exclude toggles
- Multi-session Workshop (promote ChatBar to tabbed sessions)
- Full prompt history browser
- Import/export writing styles as JSON

---

## Summary

Writingway2's most transferable ideas for NexusTale Phase B are:

1. **Beat mode** — a distinct "expand this intent into prose" mode alongside "continue"
2. **User-editable prose prompts** — named style sets stored per project, switchable at generation time
3. **Inline `@[entity]` context injection** — let writers reference wiki entries in their beat/chat
4. **Thinking model auto-detection** — future-proof for o-series and reasoning models
5. **Per-scene tense/POV in system prompt** — NexusTale already stores this metadata; use it

The beat pattern + editable prose prompts alone would significantly differentiate NexusTale's AI
UX from a generic "continue my story" button.
