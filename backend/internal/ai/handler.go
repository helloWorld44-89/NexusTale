package ai

// Handler exposes Gin route handlers for AI-assisted writing features.
//   POST /ai/complete      — expand a beat into prose (core generation)
//   POST /ai/chat          — workshop / brainstorm chat session
//   POST /ai/summarize     — summarise a scene or chapter
//   POST /ai/branch        — generate N alternative continuations
//   POST /ai/voice-check   — character voice consistency lint
//   GET  /ai/models        — list available models (API + local)
//   GET  /ai/templates     — list prompt templates for a project
//   POST /ai/templates     — create or update a prompt template
