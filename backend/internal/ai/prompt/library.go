package prompt

// Library manages per-project prompt templates stored in Postgres.
//
// Template categories (matching Writingway2's schema):
//   prose     — prose style guides injected into generation prompts
//   workshop  — chat persona / brainstorm assistant prompts
//   system    — raw system-prompt overrides for power users
//
// Features:
//   - CRUD with ordering (mirrors Writingway2's movePromptUp/Down via timestamp swap)
//   - Export as JSON (compatible with Writingway2's exportPrompts format)
//   - Import from Writingway2-exported JSON files
//   - Community template store: shared templates browsable across projects
