# NexusTale — AI Prompt Engineering & RAG Analysis

> **Purpose:** This document is a grader's brief. It records every distinct AI call in the NexusTale backend, reproduces the exact prompt text or construction logic, describes the RAG/context strategy, and lists concrete pros and cons for each. It is intended to be fed to a model for scoring and improvement recommendations. Grade each call on: system-prompt clarity, task specificity, context relevance, context efficiency, output constraints, and edge-case handling.

---

## How to read this document

Each call section contains:

- **What it does** — one-line purpose
- **Entry point** — file:function
- **Adapter method** — the low-level call type
- **System prompt** — verbatim text or exact construction pseudocode
- **User turn construction** — how the user message is assembled
- **RAG sources** — what context is fetched and injected
- **Constraints & limits** — token caps, history windows, truncation rules
- **Pros** — what the prompt does well
- **Cons / risks** — what may produce poor output or silently fail

---

## CALL 1 — Beat Completion

**What it does:** Given a single-sentence "beat" (e.g. "Kira confronts the warden"), writes 2–3 paragraphs of prose.

**Entry point:** `handler.go:Complete` → `service.go:StreamComplete` (mode=`beat`)

**Adapter method:** `StreamComplete` (streaming SSE)

### System prompt (exact construction)

```
You are a co-author helping write [a <GENRE>] novel [called "<TITLE>"].
[Write in <TENSE> tense from <POV> point of view. The POV character is <POV_CHARACTER>.]
Given a story beat (what should happen next), write 2–3 paragraphs that bring the beat to life.
Match the author's tone and style. Use sensory details. Show, don't tell.

## Project
[TITLE / GENRES from BuildContext section 1]
[AI BIBLE if set, else omitted]

## Story structure
[STRUCTURE TEMPLATE PHASES or freeform rules]

## Magic systems
[UP TO 5 RULES, limitations listed first]

## Story so far
[ALL CHAPTER SUMMARIES on active branch, or raw excerpts if no summaries]

## Entities in this scene
[ENTITY CONTEXT LINES from scene_entity_mentions, e.g.]
[Kira Voss (character) — Motivation: find her brother | Arc: guilt → agency (early arc) | ...]

## Open story threads
[UP TO 5 UNRESOLVED THREADS]

## Pinned context
[WRITER-CURATED PINS if any]

## This scene
[SCENE ROLE / GOAL / CONFLICT / OPEN THREADS — if scene_attributes are set]

## Scene ending
[LAST ~400 TOKENS OF SCENE CONTENT]
```

Tense, POV, and POV character are optional fields pulled from scene metadata. If a style preset is selected, style guidance is appended to the **user turn**:

```
<BEAT TEXT>

---
Style guidance: <PRESET CONTENT>
```

### User turn

Raw beat text only (e.g. `"Kira confronts the warden"`), plus optional style guidance appended below a `---` separator.

### RAG sources

| Source | How fetched | Notes |
|--------|------------|-------|
| Chapter summaries | `ListChapterSummaries(projectID, branch)` | All chapters on branch; falls back to canon |
| Scene entity mentions | `ListSceneEntityMentions(sceneID)` | Pre-indexed; fallback to `@[Name]` regex in scene content |
| Open story threads | `ListOpenThreadsByProject(projectID)` | Max 10 returned; capped to 5 in `buildSceneDirective` |
| Magic systems | `ListMagicRules(projectID)` | Most-recent 5; limitations listed before powers |
| Story structure | `GetProjectStructure(projectID)` | Named template phases or freeform rules |
| Pinned context | `ListContextPins(projectID, userID)` | Entities, chapters, scenes, or notes; summary/full mode |
| Scene tail | Git file read | Last 1,600 runes (~400 tokens) of scene content |
| Style preset | `GetProjectPrompt(promptID)` | Optional; appended to user turn |

### Constraints & limits

- Scene tail: 1,600 runes (~400 tokens)
- Beat output: capped by `BeatMaxTokens` (server config)
- Context budget: warn-only at 20,000 chars (~5,000 tokens), **no hard truncation**
- Entity mentions: no cap — every indexed entity is injected

---

### Pros

1. **Genre, title, tense, and POV are injected** — the model knows what kind of prose to write and which character's interiority to render.
2. **Scene-ending tail** anchors the model to the current prose boundary, preventing abrupt tonal breaks.
3. **Entity arc-position hint** (`early arc`, `mid arc`, `late arc`) gives the model behavioral context (a character in early arc behaves differently than one in late arc).
4. **Limitations-first magic system ordering** reduces the chance the model will invent abilities that violate established rules.
5. **`## This scene` directive** (role/goal/conflict) when populated gives structural purpose to the beat — the model is not writing into a vacuum.
6. **Style preset is appended to the user turn** (not the system prompt), which keeps the system prompt stable across requests and is cache-friendly.
7. **Open story threads** prevent the model from accidentally resolving a thread the writer hasn't reached yet.

### Cons / risks

1. **Craft instructions are minimal.** The entire prose guidance is: *"Match the author's tone and style. Use sensory details. Show, don't tell."* There is no model of what good prose looks like for this story, no examples, no explicit prohibition on clichés, filler phrases, or the word "suddenly." This is the most common prompt engineering failure mode: telling the model what to do without showing it.

2. **Beat text goes in the user turn with zero scaffolding.** A bare beat like `"tension builds"` is ambiguous — is the model supposed to *write* tension or *describe that tension builds*? There is no framing such as `"Expand this beat into prose:"` or `"Write the scene moment implied by:"`.

3. **`2–3 paragraphs` is hardcoded** in the system prompt. A long beat describing a major confrontation may need more; a short transition beat needs less. The model cannot adapt because the instruction is absolute.

4. **No "do not repeat what came before" guard.** The scene tail is injected as `## Scene ending`, but there is no instruction telling the model not to restate or paraphrase what it just read. Beat continuations sometimes rewrite the last sentence.

5. **Entity context is injected regardless of relevance.** If a scene mentions 12 entities (some peripherally), all 12 are injected. There is no relevance ranking or cap. On large scenes with many mentions, this adds significant token overhead with diminishing returns.

6. **All chapter summaries are injected** in `## Story so far` regardless of how many chapters exist. A 40-chapter novel will inject 40 summaries. There is no recency weighting, relevance filter, or truncation policy beyond the `contextBudgetWarnChars` warning.

7. **The context budget warning is a log line, not a truncation.** Once the context exceeds 20,000 chars (~5,000 tokens), the system logs a warning but continues to send the full context. The model receives an uncapped payload that may approach or exceed its context window on long projects.

8. **Style guidance is appended to user turn, not system.** For `system_content` presets (full system-prompt replacements), `applyPromptPreset` replaces the system prompt entirely, discarding the genre/title/tense/POV context. The writer loses the model's understanding of their story's metadata.

9. **No explicit "write new prose, do not describe what will happen" instruction.** Beat prompts occasionally produce meta-narration ("In this scene, Kira will...") rather than actual prose.

---

## CALL 2 — Continue Completion

**What it does:** Given the current scene text, writes the next natural continuation.

**Entry point:** `handler.go:Complete` → `service.go:StreamComplete` (mode=`continue`)

**Adapter method:** `StreamComplete` (streaming SSE)

### System prompt (exact construction)

```
You are a writing assistant for [a <GENRE>] novel [called "<TITLE>"].
[Tense: <TENSE>.] [POV: <POV>.]
Continue the story naturally from where it left off.

## Project
[... same sections 1-8 from BuildContext as Beat, minus section 6 (current scene) ...]

## Earlier in this scene
[FIRST ~150 TOKENS OF SCENE CONTENT]
```

### User turn

```
<LAST ~800 TOKENS OF SCENE CONTENT>

[Instruction: <OPTIONAL CUSTOM INSTRUCTION>]
```

The scene is split: `continueHeadExcerptRunes=600` goes to system prompt as `## Earlier in this scene`; `continueSceneTailRunes=3200` goes to the user turn as the direct content. The split prevents the model from reading the full scene twice.

### RAG sources

Same as Beat, plus:
- Scene head excerpt (first 600 runes → system prompt `## Earlier in this scene`)
- Scene tail (last 3,200 runes → user turn)

Note: `currentSceneID` is passed as `uuid.Nil`, so BuildContext does **not** inject `## Current scene` (section 6) — the scene content enters only as head/tail split above.

### Constraints & limits

- Head: 600 runes (~150 tokens) in system prompt
- Tail: 3,200 runes (~800 tokens) in user turn
- Output: same `BeatMaxTokens` cap as Beat

---

### Pros

1. **Head/tail split is architecturally sound.** Putting the beginning of the scene in the system prompt and the end in the user turn mirrors how the model's attention should work: the opening establishes tone/voice, the tail is what's immediately continued.
2. **Optional `Instruction` field** allows the writer to steer without writing a full beat (e.g. "slow the pace here", "add more interiority").
3. **All BuildContext sections (1-8) are present** for entity/thread/structure awareness.
4. **Avoids double-injecting scene content** — by passing `uuid.Nil`, section 6 (`## Current scene`) is suppressed, so the scene appears only as the deliberate head/tail split, not a third time.

### Cons / risks

1. **System prompt craft instructions are even thinner than Beat:** only *"Continue the story naturally from where it left off."* This gives the model no guidance on what "naturally" means for this story — no tone markers, no voice examples, no explicit constraints.

2. **The user turn is raw scene prose.** A 3,200-rune block of a writer's existing prose, with no framing, is an ambiguous signal: is the model continuing the story, or responding to it? Models occasionally interpret the final line as a question to answer rather than prose to continue.

3. **No "match the preceding paragraph count / sentence length" instruction.** If the writer writes in short punchy sentences and the model produces long flowing sentences, the continuation will feel jarring. There is no instruction to match rhythm.

4. **`Instruction` field is appended with `[Instruction: ...]`** — a bracket format that some models treat as a meta-comment and may follow poorly compared to direct imperative phrasing.

5. **Same unlimited entity/summary injection concerns as Beat** (Cons 5–7 above apply here too).

6. **No "do not summarize what follows" guard.** The model may end its continuation with a sentence like "Over the next few days, everything changed." which is summarizing-forward rather than writing the present moment.

---

## CALL 3 — Chapter Auto-Summarize

**What it does:** Condenses all scenes in a chapter into a 2–3 sentence summary stored in `chapter_summaries`, used by BuildContext as `## Story so far`.

**Entry point:** `context.go:ScheduleSummarize` → debounced `regenerateSummary` → `service.go:Summarize`

**Adapter method:** `Complete` (non-streaming)

### System prompt (exact construction)

```
You are a writing assistant. Summarize the following scene or chapter content in 2–3 sentences,
focusing on key plot events, character decisions, and narrative momentum. Be concise and factual.
[This is a chapter from a <GENRE> story.]
```

### User turn

```
<SCENE 1 CONTENT>

<SCENE 2 CONTENT>

...
```

All scenes in the chapter concatenated with `\n\n`. No scene title labels, no scene break markers.

### RAG sources

None. Only the raw scene content is passed.

### Constraints & limits

- Output: **hard cap of 200 tokens**
- Input: **uncapped** — all scene content is sent regardless of chapter length
- Debounce: 30-second quiet period after last scene save before firing
- No retry logic on failure (error is logged, no summary is written)

---

### Pros

1. **200-token hard cap** produces tight, scannable summaries appropriate for injection into AI context.
2. **Debounced auto-trigger** (30 seconds) avoids hammering the API on rapid saves while ensuring summaries are fresh.
3. **Genre hint** (if available) gives the model minor tonal guidance.
4. **Non-streaming** is correct for this call — no writer is waiting on the response, and the result goes directly to the database.

### Cons / risks

1. **All scenes are concatenated with `\n\n` and no labels.** A multi-scene chapter looks like an undifferentiated wall of prose. The model has no scene structure signal — it cannot tell where one scene ends and another begins, who the POV is per scene, or which events to weight.

2. **Chapter title and chapter number are not injected.** The summary will describe events in isolation without the model knowing where in the narrative arc this chapter sits (is this chapter 2 of 10, or chapter 18 of 20?). Narrative momentum is hard to assess without position.

3. **"Summarize" is the entire task description.** The prompt says *"key plot events, character decisions, and narrative momentum"* but does not define what makes a plot event "key" or what "narrative momentum" means in the context of this specific story. The model has no project identity — it doesn't know the protagonist's name, the genre's conventions, or what this chapter's structural role is.

4. **200 tokens is very tight for multi-scene chapters.** A chapter with 5 scenes, each with meaningful events, cannot be accurately summarized in 200 tokens (~150 words). The model is forced to compress aggressively, likely losing subplot threads and character moments that BuildContext needs for accurate AI context.

5. **No retry or validation.** If the model returns a summary that begins with "In this chapter..." (over-narrated) or contains a hallucination, there is no check. The bad summary is stored and used indefinitely until the next save triggers a re-summarize.

6. **Input is uncapped.** A 20,000-word chapter is sent in full. No input truncation occurs before the API call, meaning cost scales with chapter length linearly, and for very long chapters, the model's context window may be stressed.

7. **The call uses the same auto-selected provider as generation calls.** Summarization is a cheap, simple task ideally suited for a fast/cheap model. There is no task-specific model routing — a writer using Claude Sonnet for generation will also use it for background summaries.

---

## CALL 4 — Regular Chat

**What it does:** Stateless freeform chat about the story. Writer can ask questions, brainstorm, or request analysis.

**Entry point:** `handler.go:Chat` → `service.go:StreamChat`

**Adapter method:** `StreamChat` (streaming SSE)

### System prompt (exact construction)

```
You are Nexus, an AI co-author and story intelligence embedded in NexusTale.
Your context includes this project's chapter summaries, wiki entries, and timeline.
Answer questions about the story accurately, help develop the narrative, suggest improvements,
and assist with writing. Be concise unless the user asks for detail.

## Project
[... BuildContext sections 1-8 ...]

[## Writing style]
[STYLE PRESET CONTENT if selected]
```

If a `SystemPromptOverride` is set (rare), it replaces the Nexus identity but the context block is still appended.

### User turn

Messages passed directly from the frontend. History trimmed to last **12 turns** (6 complete exchanges) via `applyHistoryWindow`.

### RAG sources

Full BuildContext output (all 8 sections), including `## Current scene` (section 6) when a scene is open in the editor.

### Constraints & limits

- History window: 12 messages (older messages dropped, **no compression**)
- Output: server `MaxTokens` config (no chat-specific cap)
- Context budget: warn-only at 20,000 chars

---

### Pros

1. **Named identity ("Nexus")** gives the model a persona to operate from, which typically produces more consistent tone than a role-less instruction.
2. **Full 8-section BuildContext** gives the model the richest story context of any call.
3. **"Be concise unless the user asks for detail"** is a useful default that prevents wall-of-text responses for simple questions.
4. **Style preset** is available if the writer wants chat responses to reflect their story's voice.

### Cons / risks

1. **The identity description is generic.** "Answer questions accurately, help develop the narrative, suggest improvements, and assist with writing" is what any writing assistant does. There is no NexusTale-specific personality, no defined communication style (analytical? encouraging? Socratic?), no guidance on when to ask clarifying questions vs. answer directly.

2. **History window drops oldest messages with no compression.** Turn 1 of a conversation may contain critical context (e.g. the writer explained a plot constraint the model needs to honor). After 12 turns, that context is silently dropped. Unlike Workshop (which compresses older turns into a digest), Regular Chat simply truncates.

3. **"Be concise" and "assist with writing" are in tension.** If the writer asks for a rewrite of a scene, "concise" may cause the model to produce a stub rather than a full draft. There is no heuristic for when to override conciseness.

4. **Section 6 (`## Current scene`) is injected in full.** For a long scene (e.g. 3,000 words), this adds significant token overhead. Unlike the Beat/Continue paths, Chat has no tail-only excerpt — the full scene text enters the context.

5. **All chapter summaries still injected** regardless of relevance to the user's question. If the writer asks about chapter 3, they receive summaries of chapters 1–40.

---

## CALL 5 — Workshop Chat (Without Tools)

**What it does:** Named, persistent craft-advisory sessions. The model adopts a focused editor persona, optionally specialized to the project's current revision phase.

**Entry point:** `workshop_handler.go:WorkshopChat` → `service.go:StreamChat` (when `tools_enabled=false`)

**Adapter method:** `StreamChat` (streaming SSE)

### System prompt (exact construction)

**Base identity** (`defaultWorkshopSystem`):
```
You are Nexus in Workshop mode — a focused craft advisor and story analyst.
Help the writer examine narrative structure, character arcs, plot consistency, theme, pacing,
and voice. Be specific and constructive. Reference the project's actual content when relevant.
Ask clarifying questions when the problem isn't clear. Avoid vague encouragement; offer actionable insight.
```

**Phase-specific directive prepended when `project.phase != "drafting"`:**

| Phase | Directive |
|-------|-----------|
| `story_pass` | `"You are a developmental editor focused on structural integrity. For any scene or chapter discussed: (1) flag scenes that don't advance character, plot, or world; (2) identify promises made to the reader that haven't been paid off; (3) call out pacing issues — scenes that rush through moments that need weight, or linger after they've landed. Be specific. Reference open story threads and the project's story structure when relevant."` |
| `character_pass` | `"You are a character editor. For any scene discussed: does each character's action flow from their stated motivation? Is their voice distinct from others? Are they behaving consistently with their arc position — early, mid, or late in their journey? Flag moments where a character acts for the plot's convenience rather than their own authentic logic."` |
| `language_pass` | `"You are a line editor. For any prose shown, identify: passive constructions that could be active; filter words ('she saw', 'he felt', 'she noticed') that create distance; weak verbs that could be specific; adverbs masking a stronger verb; repeated sentence structure in close proximity; and places where a concrete sensory detail would land harder than an abstraction. Suggest specific rewrites."` |
| `editorial_pass` | `"You are a structural editor giving big-picture notes. Does each chapter open with something that earns attention? Does it end in a way that makes the next chapter feel necessary? Are there POV inconsistencies? Does each act do its work — setup, escalation, payoff? Be direct and organized."` |

Final system prompt:
```
[PHASE_DIRECTIVE if non-drafting]

[BASE_WORKSHOP_SYSTEM or CUSTOM SYSTEM_CONTENT if writer set one]

## Project
[... BuildContext sections 1-8 ...]

[## Writing style]
[STYLE PRESET CONTENT]
```

### User turn

Messages from persisted workshop session (JSONB). Older turns beyond the 12-turn window are **compressed into a digest**:

```
[Earlier in this session:]
You: <first 200 runes of user turn>
Nexus: <first 200 rune of assistant turn>
...
```

If the first tail message is `user`-role, an assistant ack is inserted to maintain alternation:
```
"Understood, I recall our earlier discussion. Continuing from here."
```

### RAG sources

Same as Regular Chat (full BuildContext, 8 sections).

### Constraints & limits

- History window: 12 turns before compression begins
- Digest: 200 runes per turn (~50 tokens)
- Output: server `MaxTokens` config

---

### Pros

1. **Phase-specific directives are the best prompt engineering in the codebase.** Each directive is specific, multi-point, and uses concrete examples (e.g. "`filter words ('she saw', 'he felt')`"). They tell the model exactly what to look for, not just a vague role.
2. **"Avoid vague encouragement; offer actionable insight"** is an explicit anti-pattern prohibition — well-placed.
3. **"Ask clarifying questions when the problem isn't clear"** is good socratic guidance, present here but absent from Regular Chat.
4. **History digest compression** prevents silent context drop — older turns are compressed but not lost.
5. **Session persistence** (JSONB messages) means the model can build understanding of the writer's specific project across multiple turns without re-explaining.
6. **Workshop-category prompt override** lets writers customize the persona without touching code.

### Cons / risks

1. **Phase directive prepends the base identity.** The final system prompt is `PHASE_DIRECTIVE + "\n\n" + BASE_IDENTITY + "\n\n" + CONTEXT`. This means the model reads a specialized role (e.g. "You are a developmental editor") before the NexusTale Nexus identity. The model may interpret these as two conflicting personas and default to whichever it most strongly weights. Best practice (Anthropic's guidance) is: establish identity first, then add specialization constraints.

2. **Digest truncates each turn to 200 runes (~50 tokens).** A single turn in a craft discussion may be several paragraphs of specific feedback about a plot hole. 50 tokens is enough to store roughly one sentence. Critical context from earlier turns may be irrecoverably compressed.

3. **Digest is injected as a `user` role message.** This is a semantic mismatch — a block beginning with `[Earlier in this session:]` is not something the writer typed. If the model's attention mechanism strongly weights user-role content, this may create confusing signal.

4. **The ack message is inserted as `assistant` role** (`"Understood, I recall our earlier discussion. Continuing from here."`). This is synthetic — the assistant never said this. While it maintains role alternation, it creates a false memory claim and may confuse the model about what it previously committed to.

5. **Auto-selection of first prose preset** (when no `prompt_id` is provided) is silent. If the writer has a "gritty noir" preset and didn't explicitly select it, the workshop may subtly apply a noir lens to craft advice without the writer knowing.

6. **Full BuildContext (8 sections) is injected even for pure craft questions** that don't require story knowledge (e.g. "explain what a turning point scene is"). This adds unnecessary token overhead for genre-generic questions.

---

## CALL 6 — Workshop Agent (Tool Loop)

**What it does:** Agentic mode where the model can read and write to the manuscript via tools. Up to 25 tool-use rounds before a final text response.

**Entry point:** `workshop_handler.go:WorkshopChat` → `service.go:StreamChatWithTools` (when `tools_enabled=true`)

**Adapter method:** `ChatTools` (non-streaming per round), final text streamed as SSE deltas

### System prompt (exact construction)

```
You are Nexus, an AI co-author and story intelligence embedded in NexusTale.
Your context includes this project's chapter summaries, wiki entries, and timeline.
You may use tools to write directly to the manuscript — appending to scenes,
replacing their content, or creating new scenes, chapters, and acts.

IMPORTANT: Before targeting any existing act, chapter, or scene by ID, always call
list_project_structure first so you have the correct UUIDs. Never guess or invent IDs.

When the author asks you to write, expand, or create story content, use the appropriate tool.
After each tool call, briefly confirm what you did and what comes next.

## Project
[... BuildContext sections 1-8 ...]

[## Writing style]
[STYLE PRESET CONTENT]
```

### Tool definitions (all 10 tools)

| Tool | Description |
|------|-------------|
| `list_project_structure` | Read the full act → chapter → scene tree with IDs |
| `append_to_scene` | Append text to the end of a scene |
| `replace_scene_content` | Replace the entire content of a scene |
| `create_scene` | Create a new scene in an existing chapter |
| `create_chapter` | Create a new chapter in an existing act |
| `create_act` | Create a new act in the project |
| `list_wiki_entities` | List wiki entities, optionally filtered by type |
| `create_wiki_entity` | Create a new wiki entity |
| `update_wiki_entity` | Update an entity's name, description, or summary |
| `create_wiki_relationship` | Record a relationship between two entities |

### Agentic loop logic

```
for round in 0..maxRounds (default 25):
    emit { agent_planning: true, round: N } SSE event
    response = ChatTools(messages, tools, maxTokens)
    if response.StopReason != "tool_use":
        finalText = response.Text
        break
    execute each tool call → emit ToolEvent SSE (with undo metadata)
    append tool results to messages
stream finalText as delta SSE events
```

### Constraints & limits

- Max rounds: 25 (configurable per-request)
- Tool results: returned as text, injected back into messages
- History: same 12-turn window with digest compression as Workshop Chat
- Per-round: **non-streaming** (model must finish thinking before tools are executed)

---

### Pros

1. **`list_project_structure` guardrail** — the system prompt explicitly states the model must call this tool before using any ID. This is a direct defense against hallucinated UUIDs, which would cause tool calls to fail silently or corrupt data.
2. **Undo metadata per tool** — each tool execution emits a `ToolEvent` with `scene_id`, `before_content`, `created_id`, and `project_id`. The frontend can offer per-action undo without needing a separate version history call.
3. **Round-by-round SSE events** (`agent_planning`, `round`) give the writer visibility into what the model is doing, reducing the black-box feel of long agent runs.
4. **25-round hard cap** prevents infinite tool loops on confused model states.
5. **Ollama graceful degradation** — if the model adapter doesn't implement `ChatTools`, the service falls back to `StreamChat`. The feature degrades rather than errors.
6. **Wiki tools (list, create, update, relationship)** allow the model to keep the wiki in sync when it creates a new character in the manuscript.

### Cons / risks

1. **The `list_project_structure first` instruction is a text instruction, not an enforced constraint.** The model may skip it if it believes it has the right IDs from context (e.g. if a UUID appears in the conversation history). There is no tool-dependency enforcement, pre-flight check, or error recovery path if a tool call targets a non-existent ID.

2. **`append_to_scene` vs `replace_scene_content` decision is entirely up to the model** with no guidance. The system prompt does not describe when to append vs. replace. A model following ambiguous instructions ("write the next scene") may replace a 3,000-word scene with a 400-word stub.

3. **No planning phase.** Best-practice agentic prompts ask the model to outline its plan before taking actions. Here the model jumps directly into tool calls. Complex multi-step tasks (e.g. "Write Act 2") may produce disjointed results because the model has no explicit plan to anchor sequential tool calls.

4. **"After each tool call, briefly confirm what you did and what comes next"** — this instruction causes confirmation messages to appear as SSE delta text between tool events. Writers may find these narration messages distracting or they may bloat the conversation history, consuming tokens that could be used for actual writing.

5. **All 10 tools are passed on every round** regardless of context. If the writer asked "append three more paragraphs to this scene," the model is still offered `create_act`, `create_wiki_entity`, etc. This increases the probability of spurious tool calls and wastes tokens on tool definitions.

6. **Tool schemas are not in the system prompt as examples.** The model receives tool JSON schemas but no examples of correct invocation. For complex tools like `replace_scene_content` (which requires a `scene_id` UUID and full new content as a string), there is no example showing how to format a 1,000-word scene as a single string argument.

7. **History digest still applies** — if the writer's instructions from turn 4 get compressed to 50 tokens in the digest, the model may lose critical context about what it already created (e.g., which scene IDs it already used) and re-create duplicate content.

8. **No explicit "do not truncate" instruction for replace_scene_content.** Models frequently produce shorter completions than the original when replacing content, especially when context window pressure is high. The model may overwrite a 5,000-word chapter with 800 words without warning.

---

## CROSS-CUTTING CONCERNS

### RAG Strategy Analysis

#### What is well-designed

1. **Pre-indexed entity mentions** (`scene_entity_mentions` table) — rather than running a regex at query time, entities are indexed asynchronously on scene save. This is architecturally sound and allows suppression at the entity level.

2. **8-section structured context** with labeled headers (`## Story so far`, `## Entities in this scene`, etc.) — labeled sections help models attend to the right information.

3. **Branch-aware summaries** — chapter summaries are fetched for the active branch, with fallback to canon. Writers on experimental branches get contextually accurate story-so-far.

4. **Pinned context** — writer-curated entity/chapter/scene pins as explicit context are a sound strategy. Writers know which references are load-bearing for the current session.

5. **Arc position hint** — `(early arc)`, `(mid arc)`, `(late arc)` on characters is derived from chapter index relative to total chapter count. This is simple but useful signal.

6. **Limitations-first magic system ordering** — surfaces the constraints that are most likely to cause hallucination before the capabilities.

#### What is problematic

1. **No semantic retrieval.** All context is injected by fixed rules (all summaries, all mentions, all pinned items). There is no embedding-based retrieval to select the most *relevant* subset. For large projects (20+ chapters, 50+ entities), most of the injected context is irrelevant to the current scene or question.

2. **All chapter summaries, always.** The `## Story so far` section grows linearly with project length. A 30-chapter novel with 3-sentence summaries per chapter is already ~90 sentences / ~1,800 tokens before any other context is added. There is no recency weighting (recent chapters are more relevant to the current beat), no relevance filter by active branch position, and no truncation.

3. **No context priority hierarchy.** When the context budget is exceeded, there is no policy for what to drop first. The warn-only threshold means the model receives the full payload even if it approaches 100,000 tokens. Critical context (current scene, entity mentions) is equally likely to be attended to as distant chapter summaries.

4. **Entity context lines are a fixed format.** The `buildCharacterContextLine` function produces: `[Name] (character) — Motivation: X | Arc: Y | Capability: Z`. This format never varies by task. For a Beat call, the full character motivation is likely useful. For a Summarize call, it is never injected (RAG is not used). For a Chat call asking about plot structure, character motivations may be irrelevant noise.

5. **Open story threads are injected as plain text list** with no explanation of their relationship to the current scene. The model must infer which threads are relevant. For a scene that is a pure fight sequence, injecting 10 political subplot threads adds noise.

6. **`## Current scene` (section 6) injects the full scene text** in Chat and Workshop. A 3,000-word scene adds ~4,000 tokens. For question-answering about story structure, the full scene text is almost never needed — a summary would suffice.

7. **Pinned context has no automatic relevance expiry.** A pin set three weeks ago for a different chapter is still injected into every call. There is no time-to-live, session scope, or "currently relevant" filter.

### Model Selection & Provider Strategy

#### What is well-designed

1. **Multi-provider with preference fallback** (`anthropic → openai → openrouter → gemini → groq → deepseek → ollama`) provides resilience against single-provider downtime.
2. **Ollama local fallback** means the app is functional without cloud credentials in dev.
3. **Thinking model detection** prevents system-prompt injection and max_tokens misuse on models that don't support them.

#### What is problematic

1. **No task-specific model routing.** Every call — from a 200-token chapter summary to a 25-round agentic tool loop — resolves to the same provider/model. A summary call that costs $0.001 on claude-haiku is billed the same as a sonnet-class creative generation call if the writer hasn't explicitly chosen a model.

2. **Default model is claude-haiku.** Haiku is fast and cheap, but prose generation quality is noticeably lower than Sonnet for creative writing tasks. There is no tier separation (e.g. haiku for summarize/workshop-draft, sonnet for beat/continue).

3. **The provider preference list is hardcoded.** There is no project-level or user-level preference beyond the stored API keys. A writer who wants OpenAI for generation and Anthropic for analysis cannot express this preference.

4. **OpenRouter, Gemini, Groq, DeepSeek adapters share the OpenAI adapter** (via base URL override). This means they inherit OpenAI-specific bugs and behaviors (e.g. thinking-model detection based on model name substrings, which will false-positive on non-OpenAI models with "o1" in their name).

### Token Efficiency

| Call | Estimated Context Tokens | Output Cap | Efficiency Note |
|------|--------------------------|-----------|-----------------|
| Beat | ~2,000–8,000+ (varies with project size) | BeatMaxTokens | Scales unboundedly with project; no RAG pruning |
| Continue | ~2,000–8,000+ | BeatMaxTokens | Same; head/tail split is good but context is still full |
| Summarize | ~500–20,000+ (all scene content) | 200 (hard) | Input uncapped; 200-token output cap is right-sized |
| Chat | ~3,000–10,000+ | MaxTokens (config) | Full scene in section 6 is the main waste |
| Workshop | ~3,000–10,000+ | MaxTokens (config) | Same; digest compression is good but 200-rune limit is too tight |
| Agent | ~3,000–10,000+ × 25 rounds | MaxTokens × rounds | Tool definitions add ~1,000 tokens per round even when tools are irrelevant |

### Streaming Architecture

The SSE pipeline uses `io.Pipe` with goroutines writing to `pw` and `c.Stream` reading from `pr` with a 4,096-byte buffer. This is functional but has a known edge case: if the SSE connection is dropped mid-stream, the goroutine writing to `pw` will block on the next write until the pipe is GC'd. There is no context-cancellation check inside the streaming goroutine loop.

---

## SUMMARY SCORECARD (for grader)

Rate each dimension 1–5 (5 = excellent).

| Call | System Prompt Clarity | Task Specificity | Context Relevance | Context Efficiency | Output Constraints | Edge-Case Handling |
|------|-----------------------|-----------------|--------------------|-------------------|-------------------|-------------------|
| Beat Completion | ? | ? | ? | ? | ? | ? |
| Continue Completion | ? | ? | ? | ? | ? | ? |
| Chapter Summarize | ? | ? | ? | ? | ? | ? |
| Regular Chat | ? | ? | ? | ? | ? | ? |
| Workshop Chat | ? | ? | ? | ? | ? | ? |
| Workshop Agent | ? | ? | ? | ? | ? | ? |
| Cross-cutting RAG | — | — | ? | ? | — | ? |
| Model Selection | — | — | — | ? | — | ? |

**Suggested grading criteria per dimension:**

- **System Prompt Clarity (1–5):** Is the model's role, persona, and task unambiguous? Are there contradictory instructions?
- **Task Specificity (1–5):** Does the prompt tell the model *exactly* what to produce (format, length, style, what to avoid)? Or is it generic?
- **Context Relevance (1–5):** Is the injected context actually useful for this specific task? How much is irrelevant noise?
- **Context Efficiency (1–5):** Is the context budget used wisely? Are tokens wasted on redundant or stale information?
- **Output Constraints (1–5):** Does the prompt define what good output looks like and what bad output looks like? Are length, format, and anti-patterns explicit?
- **Edge-Case Handling (1–5):** What happens when context is empty, the model hallucinates a UUID, the user beat is ambiguous, or the scene content is missing?

---

## QUICK-WIN IMPROVEMENTS (for implementor reference)

These are not implemented — they are observations from the analysis above, listed for the model reviewer to validate or reject:

1. **Beat/Continue: add a "do not rewrite what precedes this" instruction.** One sentence in the system prompt.
2. **Beat: add format clarity.** Replace "2–3 paragraphs" with "write [N] paragraphs of prose, in the voice of the story, continuing directly from the `## Scene ending` content above."
3. **Summarize: inject chapter title and chapter position** (e.g. "Chapter 5 of 12") so the model can assess narrative momentum in context.
4. **Summarize: add scene break separators** (`---`) between concatenated scene contents.
5. **Summarize: increase token cap to 300–400** for multi-scene chapters; keep 200 for single-scene.
6. **Chat: add Workshop-style digest compression** instead of silent history truncation.
7. **Workshop: swap phase directive and base identity order** (identity first, specialization second).
8. **Workshop: increase digest per-turn limit** from 200 to 500–800 runes to preserve more craft context.
9. **Agent: add a planning step.** Prepend the user task with: "Before calling any tools, briefly state your plan: what you will create/modify and in what order." This can be an instruction in the system prompt.
10. **Agent: pass only relevant tools per task.** If the writer's message contains no wiki-related intent, omit wiki tools from the call.
11. **RAG: cap chapter summaries by recency** (last N chapters before current) rather than injecting all summaries.
12. **RAG: cap entity mentions** to top N by type-priority (characters > locations > factions > items) when total mentions exceed a threshold.
13. **RAG: add context priority policy.** Define an explicit drop order for when context exceeds the budget: drop distant chapter summaries first, then old pinned context, keep current scene and entity mentions.
14. **Model routing: use a cheaper model for Summarize.** The summarize call does not need Sonnet-class quality. Routing to haiku/flash by default would reduce background costs significantly.
