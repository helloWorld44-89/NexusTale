package prompt

// Engine assembles the final [system, user] message pair sent to an AI adapter.
//
// Context assembly pipeline — directly mirrors Writingway2's approach:
//
//  1. System prompt
//     a. Resolve POV character, tense, narrative voice → base sentence
//     b. Load selected prose template (user-defined) or fall back to default
//     c. Replace placeholders: {povName}, {tense}, {pov}, {projectTitle}, {genre}
//
//  2. User message construction (in order):
//     a. Current scene text so far (full rolling context)
//     b. Prose prompt template block (marked with --- PROMPT TEMPLATE START/END ---)
//     c. Compendium (wiki) entries from @[Title] mentions in the beat
//     d. Scene references from #[Title] mentions (full text or summary)
//     e. Context-panel selections: chapters (full/summary mode), tag-filtered scenes
//     f. RAG: top-K pgvector similarity hits for implicit entity references
//     g. BEAT TO EXPAND block + "Write the next N paragraphs:" directive
//
//  3. Mention stripping: remove resolved @[Title] and #[Title] tags from beat text
//     before appending, preventing double-injection.
//
// Workshop/chat mode uses a separate pipeline: system persona prompt +
// context-panel entries + conversation history (with fidelity mode truncation).
