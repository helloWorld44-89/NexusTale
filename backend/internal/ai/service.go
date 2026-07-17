package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jconder44/nexustale/internal/ai/adapters"
	"github.com/jconder44/nexustale/internal/auth"
	"github.com/jconder44/nexustale/pkg/apperror"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
	"github.com/jconder44/nexustale/pkg/embedding"
)

// AIConfig holds AI-specific configuration loaded from the environment.
type AIConfig struct {
	OllamaURL     string
	OllamaModel   string
	MaxTokens     int
	BeatMaxTokens int
}

// Service orchestrates AI operations for a project.
// It resolves the user's stored API key to build the correct adapter,
// then delegates to that adapter for completion, chat, and summarization.
// SceneFileWriter reads and writes scene content in the git working tree.
// Implemented by *project.GitService; injected via WithSceneWriter.
type SceneFileWriter interface {
	WriteSceneFile(repoPath string, chapterID, sceneID uuid.UUID, content string) error
	ReadSceneFile(repoPath string, chapterID, sceneID uuid.UUID) (string, bool, error)
}

type Service struct {
	authSvc     *auth.Service
	queries     *sqlcgen.Queries
	cfg         AIConfig
	debounce    *debouncer
	sceneWriter SceneFileWriter
	embedStore  *EmbedStore // nil when pgvector is not configured

	// fingerprintSaveCount tracks scene-save events per project so we can
	// trigger a fingerprint refresh every 3 saves without a DB round-trip.
	fingerprintMu        sync.Mutex
	fingerprintSaveCount map[uuid.UUID]int
}

func NewService(authSvc *auth.Service, queries *sqlcgen.Queries, cfg AIConfig) *Service {
	return &Service{
		authSvc:              authSvc,
		queries:              queries,
		cfg:                  cfg,
		debounce:             newDebouncer(),
		fingerprintSaveCount: make(map[uuid.UUID]int),
	}
}

// WithSceneWriter wires the git scene file writer (called from main after both
// services are constructed — same pattern as WithNotifier on project.Service).
func (s *Service) WithSceneWriter(w SceneFileWriter) {
	s.sceneWriter = w
}

// WithEmbedding wires the pgvector embedding store. When called, semantic RAG
// is enabled for BuildContext; otherwise brute-force injection is used.
// pool must be the same pool passed to sqlcgen.New so connection limits are shared.
func (s *Service) WithEmbedding(pool *pgxpool.Pool, e embedding.Embedder) {
	s.embedStore = NewEmbedStore(pool, e)
}

// EmbedStore returns the configured EmbedStore (may be nil).
func (s *Service) EmbedStore() *EmbedStore { return s.embedStore }

// NotifySceneSaved is called after every scene save. Every 3 saves it triggers
// a background prose fingerprint recompute for the project so Beat/Continue
// prompts stay calibrated to the writer's evolving style.
func (s *Service) NotifySceneSaved(projectID uuid.UUID) {
	s.fingerprintMu.Lock()
	s.fingerprintSaveCount[projectID]++
	count := s.fingerprintSaveCount[projectID]
	s.fingerprintMu.Unlock()

	if count%3 == 0 {
		go s.RefreshProseFingerpint(context.Background(), projectID)
	}
}

// readSceneContent reads a scene's prose from the git working tree.
// Returns "" when sceneWriter is nil, the file is missing, or any lookup fails.
func (s *Service) readSceneContent(ctx context.Context, chapterID, sceneID uuid.UUID) string {
	if s.sceneWriter == nil {
		return ""
	}
	ch, err := s.queries.GetChapter(ctx, chapterID)
	if err != nil {
		return ""
	}
	proj, err := s.queries.GetProject(ctx, ch.ProjectID)
	if err != nil {
		return ""
	}
	content, _, _ := s.sceneWriter.ReadSceneFile(proj.GitRepoPath, chapterID, sceneID)
	return content
}

// ReadSceneContent resolves a scene by ID and reads its prose from the git working
// tree. Used by handler code that only has the scene ID in scope.
func (s *Service) ReadSceneContent(ctx context.Context, sceneID uuid.UUID) string {
	if s.sceneWriter == nil {
		return ""
	}
	sc, err := s.queries.GetScene(ctx, sceneID)
	if err != nil {
		return ""
	}
	return s.readSceneContent(ctx, sc.ChapterID, sc.ID)
}

// ── adapter resolution + task-tier model routing ─────────────────────────────

// providerPreference is the order in which stored keys are tried when the
// caller does not specify a preferred provider.
var providerPreference = []string{"anthropic", "openai", "openrouter", "gemini", "groq", "deepseek"}

// taskTier selects the model quality level for a given call type.
type taskTier int

const (
	// tierBackground: summarize/auto-tag — use the cheapest capable model.
	// anthropic → claude-haiku-4-5, openai → gpt-4o-mini (adapter defaults).
	tierBackground taskTier = iota

	// tierAnalysis: chat/workshop/agent — balanced quality + cost.
	// anthropic → claude-sonnet-4-6, openai → gpt-4o.
	tierAnalysis

	// tierCreative: beat/continue — best prose model.
	// Same targets as tierAnalysis (sonnet/gpt-4o); a dedicated creative tier
	// allows future routing to even larger models without changing call sites.
	tierCreative
)

// tierModel returns the model ID override for the given provider and tier.
// Returns "" to use the provider's built-in default or stored user preference.
//
// Coverage by provider:
//   - anthropic/openai: fully tiered (haiku/mini for background, sonnet/gpt-4o for creative)
//   - groq: tiered (8b-instant for background, 70b-versatile for creative)
//   - gemini: no override — gemini-2.0-flash default is already cheap; user's stored
//     model handles creative (they may have set gemini-2.5-pro in Settings)
//   - deepseek: no override — deepseek-chat is the general model; deepseek-reasoner is
//     a thinking model with different latency/format, not suitable for auto-routing
//   - openrouter: no override here — background model handled separately via
//     openrouterBackgroundModelForUser() in getAdapterForTier
func tierModel(provider string, tier taskTier) string {
	switch tier {
	case tierCreative, tierAnalysis:
		switch provider {
		case "anthropic":
			return "claude-sonnet-4-6"
		case "openai":
			return "gpt-4o"
		case "groq":
			return "llama-3.3-70b-versatile"
		}
	case tierBackground:
		switch provider {
		case "groq":
			// 8b-instant is Groq's cheapest/fastest model — ideal for background summarize.
			return "llama-3.1-8b-instant"
		}
	}
	return ""
}

// getAdapter resolves the adapter for the requesting user at the background tier.
// When provider is non-empty it is used directly; otherwise keys are tried in
// providerPreference order, falling back to Ollama if none are stored.
func (s *Service) getAdapter(ctx context.Context, userID uuid.UUID, provider string) (adapters.Adapter, error) {
	return s.getAdapterForTier(ctx, userID, provider, tierBackground)
}

// getAdapterForTier resolves the adapter for the requesting user at the given
// quality tier. When provider is non-empty the explicit provider is used with
// that provider's default model (writer's choice always overrides tier routing).
// When provider is empty, keys are tried in providerPreference order and the
// tier-appropriate model is injected for providers that support it.
func (s *Service) getAdapterForTier(ctx context.Context, userID uuid.UUID, provider string, tier taskTier) (adapters.Adapter, error) {
	// For OpenRouter, switch to the background model when available and tier is background.
	// This lets writers route cheap calls (summarize) through a low-cost model (e.g.
	// meta-llama/llama-3.1-8b-instruct) while keeping their quality model for prose.
	orModel := s.openRouterModelForUser(ctx, userID)
	if tier == tierBackground {
		if bgModel := s.openrouterBackgroundModelForUser(ctx, userID); bgModel != "" {
			orModel = bgModel
		}
	}

	adapterCfg := adapters.AdapterConfig{
		OllamaURL:       s.ollamaURLForUser(ctx, userID),
		OllamaModel:     s.ollamaModelForUser(ctx, userID),
		OpenRouterModel: orModel,
		GeminiModel:     s.geminiModelForUser(ctx, userID),
		GroqModel:       s.groqModelForUser(ctx, userID),
		DeepSeekModel:   s.deepSeekModelForUser(ctx, userID),
	}

	// Explicit provider: respect writer's choice, use their stored/default model.
	if provider != "" {
		key, err := s.authSvc.DecryptAPIKey(ctx, userID, provider)
		if err != nil {
			return adapters.NewAdapter("ollama", "", "", adapterCfg)
		}
		return adapters.NewAdapter(provider, key, "", adapterCfg)
	}

	// Auto-select provider; inject tier-appropriate model where supported.
	for _, p := range providerPreference {
		key, err := s.authSvc.DecryptAPIKey(ctx, userID, p)
		if err != nil {
			continue
		}
		model := tierModel(p, tier)
		adapter, err := adapters.NewAdapter(p, key, model, adapterCfg)
		if err != nil {
			continue
		}
		return adapter, nil
	}

	return adapters.NewAdapter("ollama", "", "", adapterCfg)
}

// ollamaURLForUser returns the Ollama base URL for the given user.
// If the user has stored a custom URL via Settings (provider="ollama"),
// that takes precedence over the server-wide default from config.
// This allows each user to point at their own Ollama instance — important
// when the API runs in Docker and Ollama is on the host or another machine.
func (s *Service) ollamaURLForUser(ctx context.Context, userID uuid.UUID) string {
	if url, err := s.authSvc.DecryptAPIKey(ctx, userID, "ollama"); err == nil && url != "" {
		return url
	}
	return s.cfg.OllamaURL
}

// ollamaModelForUser returns the Ollama model name for the given user.
// If the user has stored a preferred model via Settings (provider="ollama_model"),
// that takes precedence over the server-wide default from config.
func (s *Service) ollamaModelForUser(ctx context.Context, userID uuid.UUID) string {
	if model, err := s.authSvc.DecryptAPIKey(ctx, userID, "ollama_model"); err == nil && model != "" {
		return model
	}
	return s.cfg.OllamaModel
}

// openRouterModelForUser returns the preferred OpenRouter model for the user.
// Stored as provider="openrouter_model" in user_api_keys (e.g. "anthropic/claude-3-5-sonnet").
// This is the quality model used for Beat, Continue, and Chat.
func (s *Service) openRouterModelForUser(ctx context.Context, userID uuid.UUID) string {
	if model, err := s.authSvc.DecryptAPIKey(ctx, userID, "openrouter_model"); err == nil && model != "" {
		return model
	}
	return ""
}

// openrouterBackgroundModelForUser returns the user's optional cheaper OpenRouter model
// for background tasks (summarize). Stored as provider="openrouter_background_model".
// When unset, the quality model (openrouter_model) is used for all tiers.
func (s *Service) openrouterBackgroundModelForUser(ctx context.Context, userID uuid.UUID) string {
	if model, err := s.authSvc.DecryptAPIKey(ctx, userID, "openrouter_background_model"); err == nil && model != "" {
		return model
	}
	return ""
}

// geminiModelForUser returns the preferred Gemini model for the user.
// Stored as provider="gemini_model" in user_api_keys (e.g. "gemini-1.5-pro").
func (s *Service) geminiModelForUser(ctx context.Context, userID uuid.UUID) string {
	if model, err := s.authSvc.DecryptAPIKey(ctx, userID, "gemini_model"); err == nil && model != "" {
		// Strip the "models/" prefix that the Gemini list API returned pre-fix.
		// The OpenAI-compatible endpoint requires bare IDs like "gemini-2.0-flash".
		return strings.TrimPrefix(model, "models/")
	}
	return ""
}

// groqModelForUser returns the preferred Groq model for the user.
// Stored as provider="groq_model" in user_api_keys (e.g. "llama-3.1-8b-instant").
func (s *Service) groqModelForUser(ctx context.Context, userID uuid.UUID) string {
	if model, err := s.authSvc.DecryptAPIKey(ctx, userID, "groq_model"); err == nil && model != "" {
		return model
	}
	return ""
}

// deepSeekModelForUser returns the preferred DeepSeek model for the user.
// Stored as provider="deepseek_model" in user_api_keys (e.g. "deepseek-reasoner").
func (s *Service) deepSeekModelForUser(ctx context.Context, userID uuid.UUID) string {
	if model, err := s.authSvc.DecryptAPIKey(ctx, userID, "deepseek_model"); err == nil && model != "" {
		return model
	}
	return ""
}

// ── beat system prompt ────────────────────────────────────────────────────────

// beatSystemPrompt builds the system prompt for beat-expansion mode.
// Behavioral constraints replace vague style advice to eliminate the most common
// failure modes: repetition of the scene tail, meta-narration, and abstraction
// where sensation is needed.
// fp is optional; when non-nil, paragraph count and style hints are derived from
// the writer's measured prose rhythm (C9-P5).
func beatSystemPrompt(title string, genres []string, tense, pov, povCharacter string, fp *ProseFingerprint) string {
	var sb strings.Builder
	sb.WriteString("You are a co-author helping write")
	if len(genres) > 0 {
		sb.WriteString(" a ")
		sb.WriteString(strings.Join(genres, "/"))
	}
	sb.WriteString(" novel")
	if title != "" {
		sb.WriteString(fmt.Sprintf(" called %q", title))
	}
	sb.WriteString(".\n")

	if tense != "" || pov != "" {
		sb.WriteString("Write")
		if tense != "" {
			sb.WriteString(fmt.Sprintf(" in %s tense", tense))
		}
		if pov != "" {
			sb.WriteString(fmt.Sprintf(" from %s point of view", pov))
		}
		if povCharacter != "" {
			sb.WriteString(fmt.Sprintf(". The POV character is %s", povCharacter))
		}
		sb.WriteString(".\n")
	}

	// Paragraph count hint — derived from fingerprint when available.
	paraHint := "approximately 2–3 paragraphs"
	if fp != nil {
		switch {
		case fp.AvgParagraphLength <= 2:
			paraHint = "3–5 short paragraphs (matching the author's brief paragraph rhythm)"
		case fp.AvgParagraphLength >= 5:
			paraHint = "1–2 longer paragraphs (matching the author's dense paragraph rhythm)"
		}
	}
	sb.WriteString("\nExpand the beat into prose (" + paraHint + " — expand or compress as the beat demands).\n")
	sb.WriteString("\nDO NOT:\n")
	sb.WriteString("- Repeat or rephrase the last sentence of the scene excerpt\n")
	sb.WriteString("- Describe what the narrator or POV character is doing meta-narratively (\"she decided to\", \"he thought about\", \"she realized\")\n")
	sb.WriteString("- Summarise or preview what comes next\n")
	sb.WriteString("- Use adverbs when a stronger verb exists\n")
	sb.WriteString("- Use \"suddenly\", \"realized\", \"noticed\", or \"felt\" as a first word in a sentence\n")
	sb.WriteString("\nDO:\n")
	sb.WriteString("- Render emotion through physical sensation, action, and observed detail — not abstraction\n")
	sb.WriteString("- Match the sentence rhythm and vocabulary register of the scene excerpt\n")
	sb.WriteString("- Continue directly from where the scene ends; the prose must feel uncut")
	return sb.String()
}

// narrativePhaseDirective maps a narrative phase enum value to a one-line focus
// directive injected into the Continue system prompt.
// An empty string is returned for unrecognized or empty values (safe to append).
func narrativePhaseDirective(phase string) string {
	switch phase {
	case "escalation":
		return "Narrative function: ESCALATION — raise the stakes, increase pressure, deny the character what they want."
	case "reflection":
		return "Narrative function: REFLECTION — let the character process what just happened; show internal shift, not action."
	case "confrontation":
		return "Narrative function: CONFRONTATION — the conflict reaches a head; someone must act, yield, or be changed."
	case "discovery":
		return "Narrative function: DISCOVERY — reveal information that reframes what came before; let the character react authentically."
	case "aftermath":
		return "Narrative function: AFTERMATH — show the cost and consequence of what happened; the world is not the same."
	case "transition":
		return "Narrative function: TRANSITION — move time, place, or focus; establish the new situation without over-explaining it."
	default:
		return ""
	}
}

// continueSystemPrompt is the system prompt for continue mode.
// Behavioral constraints replace vague style advice to eliminate repetition,
// summary, and tense/POV drift.
func continueSystemPrompt(title string, genres []string, tense, pov, narrativePhase string) string {
	var sb strings.Builder
	sb.WriteString("You are a writing assistant for")
	if len(genres) > 0 {
		sb.WriteString(" a ")
		sb.WriteString(strings.Join(genres, "/"))
	}
	sb.WriteString(" novel")
	if title != "" {
		sb.WriteString(fmt.Sprintf(" called %q", title))
	}
	sb.WriteString(".")
	if tense != "" {
		sb.WriteString(fmt.Sprintf(" Tense: %s.", tense))
	}
	if pov != "" {
		sb.WriteString(fmt.Sprintf(" POV: %s.", pov))
	}

	if dir := narrativePhaseDirective(narrativePhase); dir != "" {
		sb.WriteString("\n\n" + dir)
	}

	sb.WriteString("\n\nDO NOT:\n")
	sb.WriteString("- Repeat, rephrase, or echo the last sentence of the scene text you receive\n")
	sb.WriteString("- Summarise or narrate forward (\"later\", \"eventually\", \"by the time\")\n")
	sb.WriteString("- Shift tense or point of view\n")
	sb.WriteString("- Begin with a transition word or time marker unless the beat specifically calls for it\n")
	sb.WriteString("\nDO:\n")
	sb.WriteString("- Write the next present moment — what happens immediately after the last word\n")
	sb.WriteString("- Match the sentence length, rhythm, and vocabulary of the passage you received\n")
	sb.WriteString("- Stay in scene; if nothing is happening, find the tension in stillness")
	return sb.String()
}

// ── public API ────────────────────────────────────────────────────────────────

// CompleteRequest is the public request type for the Complete/StreamComplete operations.
type CompleteRequest struct {
	ProjectID       uuid.UUID
	SceneID         uuid.UUID
	BranchName      string                // active Timeline; empty = resolved via ResolveBranch
	Mode            adapters.CompleteMode // "continue" | "beat"
	Beat            string                // required when mode=beat
	Instruction     string                // optional hint for continue mode
	NarrativePhase  string                // optional for continue: escalation|reflection|confrontation|discovery|aftermath|transition
	Provider        string                // optional — auto-selected if empty
	MaxTokens       int                   // 0 = use config default
	PromptID        uuid.UUID             // optional writing style preset
}

// ChatRequest is the public request type for chat operations.
type ChatRequest struct {
	ProjectID            uuid.UUID
	SceneID              uuid.UUID
	BranchName           string // active Timeline
	Messages             []adapters.Message
	Provider             string
	MaxTokens            int
	PromptID             uuid.UUID // optional writing style preset — content appended to system prompt
	SystemPromptOverride string    // if non-empty, replaces the default Nexus identity prompt
	WorkshopMode         bool      // use digest-based history window instead of plain truncation
	ChatMode             string    // "" | "brainstorm" | "editorial" | "lore"
}

// StreamComplete streams the AI response for scene continuation or beat expansion.
// It writes NexusTale SSE format to w.
func (s *Service) StreamComplete(ctx context.Context, userID uuid.UUID, req CompleteRequest, w io.Writer) (adapters.Usage, error) {
	adapter, err := s.getAdapterForTier(ctx, userID, req.Provider, tierCreative)
	if err != nil {
		return adapters.Usage{}, fmt.Errorf("get adapter: %w", err)
	}

	// Resolve scene and project metadata for system prompt construction.
	project, scene, err := s.resolveContext(ctx, req.ProjectID, req.SceneID)
	if err != nil {
		// Non-fatal — continue with empty metadata.
		slog.Warn("ai: could not resolve scene/project context", "error", err)
	}

	maxTok := req.MaxTokens
	if maxTok == 0 {
		maxTok = s.cfg.MaxTokens
	}

	var adapterReq adapters.CompleteRequest
	adapterReq.Mode = req.Mode
	adapterReq.MaxTokens = maxTok

	// Build the AI context window (project identity + chapter content + @[entity] snippets + pins).
	//
	// Both beat and continue suppress the full "Current scene" section (pass uuid.Nil):
	//   • Continue: the full scene text IS the user turn — including it in context would duplicate it.
	//   • Beat: we inject only the last ~400 tokens as "## Scene ending" (see below), which is
	//     enough for style-matching without paying for the full text on long scenes.
	// sceneContent is still passed for @[entity] parsing in BuildContext section 4.
	ctxSceneID := uuid.Nil
	ctxBlock := s.BuildContext(ctx, req.ProjectID, userID, req.BranchName, scene.Content, ctxSceneID)

	// Build scene-specific directive lines (role, goal, conflict, open threads).
	// These are appended to the system prompt for both beat and continue modes
	// so the model has the scene's structural purpose and unresolved threads in scope.
	sceneDirective := s.buildSceneDirective(ctx, req.ProjectID, scene)

	// Load the project's prose fingerprint for style-aware prompts.
	var fp *ProseFingerprint
	if p, err := s.queries.GetProject(ctx, req.ProjectID); err == nil && len(p.ProseFingerprint) > 0 {
		var parsed ProseFingerprint
		if json.Unmarshal(p.ProseFingerprint, &parsed) == nil {
			fp = &parsed
		}
	}

	switch req.Mode {
	case adapters.CompleteModeBeat:
		if maxTok == 0 || maxTok > s.cfg.BeatMaxTokens {
			adapterReq.MaxTokens = s.cfg.BeatMaxTokens
		}
		sysPrompt := beatSystemPrompt(project.Title, project.Genres, scene.Tense, scene.Pov, "", fp)
		if ctxBlock != "" {
			sysPrompt = sysPrompt + "\n\n" + ctxBlock
		}
		// Inject prose fingerprint as a style guide for the model.
		if block := FingerprintContextBlock(fp); block != "" {
			sysPrompt += "\n\n" + block
		}
		if sceneDirective != "" {
			sysPrompt += "\n\n" + sceneDirective
		}
		// Append only the tail of the scene so the model can match prose style at
		// the boundary without paying for the full text on long scenes.
		if scene.Content != "" {
			tail := sceneEndingExcerpt(scene.Content, beatSceneTailRunes)
			sysPrompt += "\n\n## Scene ending\n" + tail
		}
		adapterReq.SystemPrompt = sysPrompt
		// Explicit user-turn framing so the model knows exactly what to expand.
		adapterReq.Content = fmt.Sprintf("Expand the following story beat into prose, continuing directly from the scene ending above:\n\n%s", req.Beat)
	default:
		sysPrompt := continueSystemPrompt(project.Title, project.Genres, scene.Tense, scene.Pov, req.NarrativePhase)
		if ctxBlock != "" {
			sysPrompt = sysPrompt + "\n\n" + ctxBlock
		}
		if block := FingerprintContextBlock(fp); block != "" {
			sysPrompt += "\n\n" + block
		}
		if sceneDirective != "" {
			sysPrompt += "\n\n" + sceneDirective
		}
		// For long scenes, cap the user turn at the last ~800 tokens.
		// Prepend a brief excerpt so the model knows the scene is not starting fresh.
		head, tail := splitSceneContent(scene.Content, continueSceneTailRunes, continueHeadExcerptRunes)
		if head != "" {
			sysPrompt += "\n\n## Earlier in this scene\n" + head
		}
		adapterReq.SystemPrompt = sysPrompt
		content := tail
		if req.Instruction != "" {
			content += "\n\n[Instruction: " + req.Instruction + "]"
		}
		adapterReq.Content = content
	}

	// Apply writing style preset if provided.
	if req.PromptID != uuid.Nil {
		s.applyPromptPreset(ctx, req.PromptID, &adapterReq)
	}

	usage, err := adapter.StreamComplete(ctx, adapterReq, w)
	s.recordUsage(req.ProjectID, userID, adapter.Provider(), usage, string(req.Mode), req.Beat, req.SceneID)
	return usage, err
}

// chatHistoryWindow is the maximum number of user/assistant turns kept in the
// sliding history window. Turns older than this are dropped before each call.
// This caps token spend on long sessions and prevents context-limit errors.
// 12 turns ≈ 6 complete exchanges — enough conversational memory for most sessions.
const chatHistoryWindow = 12

// Scene-content trimming limits for beat and continue modes.
// Rune counts approximate token counts at roughly 4 chars/token.
const (
	beatSceneTailRunes       = 1600 // ~400 tokens — the tail injected as "## Scene ending"
	continueSceneTailRunes   = 3200 // ~800 tokens — max user-turn length for continue
	continueHeadExcerptRunes = 600  // runes preserved as "## Earlier in this scene" hint
)

// sceneEndingExcerpt returns the last n runes of content.
// Used by beat mode to inject only the prose boundary the model needs for style-matching.
func sceneEndingExcerpt(content string, n int) string {
	runes := []rune(content)
	if len(runes) <= n {
		return content
	}
	return string(runes[len(runes)-n:])
}

// splitSceneContent splits content for continue mode into a head excerpt and a tail.
// The tail is the last tailRunes runes (used as the model's user turn).
// The head is a short excerpt of the beginning (injected as "## Earlier in this scene").
// When content fits within tailRunes, head is empty and tail is the full content.
func splitSceneContent(content string, tailRunes, headExcerptRunes int) (head, tail string) {
	runes := []rune(content)
	if len(runes) <= tailRunes {
		return "", content
	}
	tail = string(runes[len(runes)-tailRunes:])
	headRunes := runes[:len(runes)-tailRunes]
	if len(headRunes) > headExcerptRunes {
		head = string(headRunes[:headExcerptRunes]) + "…"
	} else {
		head = string(headRunes)
	}
	return head, tail
}

// applyHistoryWindow trims msgs to at most maxTurns entries, keeping the tail.
// msgs must not include the system message (that is prepended separately).
func applyHistoryWindow(msgs []adapters.Message, maxTurns int) []adapters.Message {
	if len(msgs) <= maxTurns {
		return msgs
	}
	return msgs[len(msgs)-maxTurns:]
}

// workshopDigestMaxRunes is the per-turn character cap used when compressing
// older workshop turns into the digest summary. 600 runes ≈ one paragraph of
// craft feedback — enough to preserve the substance of each turn.
const workshopDigestMaxRunes = 600

// applyWorkshopHistoryWindow is like applyHistoryWindow but designed for workshop
// sessions that have strong multi-turn continuity. Instead of silently dropping
// older turns, it compresses them into a synthetic user+assistant digest pair so
// the model retains context across long sessions.
//
// The digest message uses role "user" so that the result always starts with a
// user turn, satisfying provider constraints (e.g. Anthropic requires the first
// message to be role "user"). When the first retained tail message is also "user",
// a brief assistant acknowledgment is inserted to maintain proper alternation.
func applyWorkshopHistoryWindow(msgs []adapters.Message, maxTurns int) []adapters.Message {
	if len(msgs) <= maxTurns {
		return msgs
	}

	head := msgs[:len(msgs)-maxTurns]
	tail := msgs[len(msgs)-maxTurns:]

	var sb strings.Builder
	sb.WriteString("[Earlier in this session:]\n")
	for _, m := range head {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		label := "You"
		if m.Role == "assistant" {
			label = "Nexus"
		}
		runes := []rune(m.Content)
		if len(runes) > workshopDigestMaxRunes {
			runes = append(runes[:workshopDigestMaxRunes], '…')
		}
		fmt.Fprintf(&sb, "%s: %s\n", label, string(runes))
	}

	digest := adapters.Message{Role: "user", Content: strings.TrimRight(sb.String(), "\n")}

	// When the first tail message is also "user", insert an assistant ack so the
	// conversation maintains strict user/assistant alternation.
	if len(tail) > 0 && tail[0].Role == "user" {
		ack := adapters.Message{
			Role:    "assistant",
			Content: "Understood, I recall our earlier discussion. Continuing from here.",
		}
		result := make([]adapters.Message, 0, 2+len(tail))
		return append(append(result, digest, ack), tail...)
	}

	result := make([]adapters.Message, 0, 1+len(tail))
	return append(append(result, digest), tail...)
}

// StreamChat streams a chat response to w.
// A context window (chapter summaries + @[entity] snippets) is injected as a
// system message prepended to the conversation.
func (s *Service) StreamChat(ctx context.Context, userID uuid.UUID, req ChatRequest, w io.Writer) (adapters.Usage, error) {
	adapter, err := s.getAdapterForTier(ctx, userID, req.Provider, tierAnalysis)
	if err != nil {
		return adapters.Usage{}, fmt.Errorf("get adapter: %w", err)
	}

	maxTok := req.MaxTokens
	if maxTok == 0 {
		maxTok = s.cfg.MaxTokens
	}

	// Resolve scene content for @[entity] parsing if a scene is in scope.
	sceneContent := ""
	if req.SceneID != uuid.Nil {
		sceneContent = s.ReadSceneContent(ctx, req.SceneID)
	}

	// Trim history before building the request so long sessions don't grow unboundedly.
	// Both regular chat and workshop sessions now use the digest compressor (not silent
	// truncation) so older context is preserved as a compressed summary rather than lost.
	messages := applyWorkshopHistoryWindow(req.Messages, chatHistoryWindow)

	// Build the context window (project identity + chapter content + @[entity] snippets + pins).
	ctxBlock := s.BuildContext(ctx, req.ProjectID, userID, req.BranchName, sceneContent, req.SceneID)

	// Build the identity+context system prompt.
	// SystemPromptOverride lets callers (e.g. Workshop) substitute a different
	// persona or craft focus without changing the context block.
	var nexusSystem string
	if req.SystemPromptOverride != "" {
		nexusSystem = req.SystemPromptOverride + "\n\n" + ctxBlock
	} else {
		nexusSystem = nexusChatSystemPrompt(req.ChatMode) + "\n\n" + ctxBlock
	}

	// Append writing style guidance when the writer has a style preset selected.
	// We append content (not system_content) so the Nexus identity + context are preserved.
	if style := s.fetchStyleContent(ctx, req.PromptID); style != "" {
		nexusSystem += "\n\n## Writing style\n" + style
	}

	messages = append([]adapters.Message{{Role: "system", Content: nexusSystem}}, messages...)

	usage, err := adapter.StreamChat(ctx, adapters.ChatRequest{
		Messages:  messages,
		MaxTokens: maxTok,
	}, w)
	s.recordUsage(req.ProjectID, userID, adapter.Provider(), usage, "chat", "", req.SceneID)
	return usage, err
}

// StreamChatWithTools streams an agentic workshop response with manuscript tool use.
//
// The AI may call ManuscriptTools (append/replace scenes, create scenes/chapters/acts)
// before returning its final natural-language reply. Each tool invocation is:
//
//  1. Emitted as an SSE planning event: data: {"agent_planning":true,"round":N}\n\n
//  2. Executed against the database.
//  3. Emitted as a ToolEvent SSE payload with undo metadata.
//  4. Fed back to the model as a tool result.
//
// The loop runs for at most maxRounds rounds (caller sets; 0 → default 25).
// The final text is streamed as normal delta + [DONE] events. Falls back to
// StreamChat if the adapter does not implement ToolAdapter (Ollama).
func (s *Service) StreamChatWithTools(ctx context.Context, userID uuid.UUID, req ChatRequest, w io.Writer, maxRounds int) (adapters.Usage, error) {
	adapter, err := s.getAdapterForTier(ctx, userID, req.Provider, tierAnalysis)
	if err != nil {
		return adapters.Usage{}, fmt.Errorf("get adapter: %w", err)
	}

	ta, ok := adapter.(adapters.ToolAdapter)
	if !ok {
		// Ollama or future adapter without tool support — degrade gracefully.
		return s.StreamChat(ctx, userID, req, w)
	}

	maxTok := req.MaxTokens
	if maxTok == 0 {
		maxTok = s.cfg.MaxTokens
	}

	sceneContent := ""
	if req.SceneID != uuid.Nil {
		sceneContent = s.ReadSceneContent(ctx, req.SceneID)
	}

	ctxBlock := s.BuildContext(ctx, req.ProjectID, userID, req.BranchName, sceneContent, req.SceneID)

	var nexusSystem string
	if req.SystemPromptOverride != "" {
		nexusSystem = req.SystemPromptOverride + "\n\n" + ctxBlock
	} else {
		nexusSystem = "You are Nexus, an AI co-author and story intelligence embedded in NexusTale. " +
			"Your context includes this project's chapter summaries, wiki entries, and timeline. " +
			"You may use tools to write directly to the manuscript.\n\n" +
			"PLANNING MANDATE — before calling any tool:\n" +
			"1. State your plan in plain text: what you intend to do and why.\n" +
			"2. List the target IDs (scene, chapter, act) you will write to.\n" +
			"3. Only then call tools.\n\n" +
			"TOOL SELECTION RULES:\n" +
			"- Use append_to_scene to add new content after existing prose. This is the default for all new writing.\n" +
			"- Use replace_scene_content ONLY when the author explicitly asks you to rewrite or replace existing prose. Never shorten a scene when replacing — the replacement must be at least as long.\n" +
			"- Use create_scene / create_chapter / create_act only when the author asks to create new structure.\n" +
			"- Before targeting any existing scene or chapter by ID, call list_project_structure first. Never guess or invent IDs.\n\n" +
			"After each tool call, briefly confirm what you wrote and what comes next.\n\n" + ctxBlock
	}

	if style := s.fetchStyleContent(ctx, req.PromptID); style != "" {
		nexusSystem += "\n\n## Writing style\n" + style
	}

	var historyMsgs []adapters.Message
	if req.WorkshopMode {
		historyMsgs = applyWorkshopHistoryWindow(req.Messages, chatHistoryWindow)
	} else {
		historyMsgs = applyHistoryWindow(req.Messages, chatHistoryWindow)
	}
	messages := append([]adapters.Message{{Role: "system", Content: nexusSystem}}, historyMsgs...)

	var extraMsgs []json.RawMessage
	var totalUsage adapters.Usage
	if maxRounds <= 0 {
		maxRounds = 25
	}
	finalText := ""

	for round := 0; round < maxRounds; round++ {
		// Emit a planning event so the frontend can show "Nexus is planning..."
		// before the model responds.  The round number lets the UI show progress.
		planningPayload, _ := json.Marshal(map[string]any{"agent_planning": true, "round": round + 1})
		fmt.Fprintf(w, "data: %s\n\n", planningPayload)

		// Select the minimal tool set for the user's apparent intent so we
		// don't pay for unused tool descriptions on every round.
		tools := selectToolsForIntent(historyMsgs)
		resp, err := ta.ChatTools(ctx, messages, extraMsgs, tools, maxTok)
		if err != nil {
			return totalUsage, fmt.Errorf("tool chat round %d: %w", round, err)
		}

		totalUsage.PromptTokens += resp.Usage.PromptTokens
		totalUsage.CompletionTokens += resp.Usage.CompletionTokens
		totalUsage.CostUSD += resp.Usage.CostUSD

		if resp.StopReason != "tool_use" || len(resp.ToolCalls) == 0 {
			finalText = resp.Text
			break
		}

		// Append the assistant's tool-use turn to the conversation history.
		extraMsgs = append(extraMsgs, resp.RawAssistantMsg)

		// Execute each tool and collect results.
		toolResults := make([]adapters.ToolResult, 0, len(resp.ToolCalls))
		for _, tc := range resp.ToolCalls {
			result, evt := s.executeToolCall(ctx, req.ProjectID, tc)
			toolResults = append(toolResults, result)

			// Emit a ToolEvent SSE payload so the frontend can show progress
			// and offer per-action Undo.  Parsers that only check for "delta"
			// safely ignore these events.
			evtPayload, _ := json.Marshal(evt)
			fmt.Fprintf(w, "data: %s\n\n", evtPayload)
		}

		// Append tool results to the conversation history for the next round.
		extraMsgs = append(extraMsgs, ta.BuildToolResultMessages(toolResults)...)
	}

	// Stream the final text as delta SSE events.
	if finalText != "" {
		encoded, _ := json.Marshal(map[string]string{"delta": finalText})
		fmt.Fprintf(w, "data: %s\n\n", encoded)
	}
	fmt.Fprintf(w, "data: [DONE]\n\n")

	s.recordUsage(req.ProjectID, userID, adapter.Provider(), totalUsage, "chat_tools", "", req.SceneID)
	return totalUsage, nil
}

// nexusChatSystemPrompt returns the base identity system prompt for Nexus chat,
// tailored to the requested mode. An empty mode returns the general-purpose prompt.
func nexusChatSystemPrompt(mode string) string {
	switch mode {
	case "brainstorm":
		return "You are Nexus in Brainstorm mode — a creative partner with no attachment to the status quo. " +
			"For every question or problem, suggest 2–3 distinct directions the story could go, each with a brief " +
			"rationale. Be generative, not evaluative. The writer will decide; your job is to expand the possibility space. " +
			"Use the project context to make suggestions feel native to this story's world and tone."
	case "editorial":
		return "You are Nexus in Editorial mode — a structural editor examining the story with craft criteria. " +
			"Focus on: scene-level cause-and-effect, character motivation consistency, pacing, promises-and-payoffs, " +
			"and act-level shape. Be specific and direct. Cite the actual content when possible. " +
			"Separate observations (what you see) from recommendations (what to consider changing)."
	case "lore":
		return "You are Nexus in Lore mode — the project's wiki oracle. " +
			"Answer questions about characters, locations, factions, magic systems, and timeline events " +
			"using only information that exists in the project's wiki and chapter summaries. " +
			"If information doesn't exist in the project yet, say so clearly rather than inventing. " +
			"Be concise and factual. Cross-reference related entities when relevant."
	default:
		return "You are Nexus, an AI co-author and story intelligence embedded in NexusTale. " +
			"Your context includes this project's chapter summaries, wiki entries, and timeline. " +
			"Answer questions about the story accurately, help develop the narrative, suggest improvements, " +
			"and assist with writing. Be concise unless the user asks for detail."
	}
}

// summarizeSystemPrompt returns the system prompt for chapter summarization.
// The structured EVENTS / CHANGES / PRESSURE format produces summaries that:
//   - Feed directly into BuildContext "Story so far" (factual, scannable)
//   - Drive the story threads system (PRESSURE points forward)
//   - Avoid the most common over-narration failure ("In this chapter…")
//
// An optional genre suffix nudges vocabulary toward appropriate register.
func summarizeSystemPrompt(genre string) string {
	base := "You are a writing assistant summarizing a chapter for an AI story context window.\n" +
		"Output exactly three labeled sections with no preamble:\n\n" +
		"EVENTS: [1–2 sentences — what physically happened, in order]\n" +
		"CHANGES: [1 sentence — how a character or situation changed by the end]\n" +
		"PRESSURE: [1 sentence — what is now unresolved, at stake, or looming]\n\n" +
		"Rules: be specific and factual; name characters; do not begin with " +
		"\"In this chapter\" or \"This chapter\"; do not editorialize."
	if genre != "" {
		base += " This is a chapter from a " + genre + " story."
	}
	return base
}

// isValidSummary returns false for outputs the summarizer commonly produces
// that are useless as AI context: empty strings, too-short responses, and
// the most frequent over-narration prefixes.
func isValidSummary(s string) bool {
	s = strings.TrimSpace(s)
	if len([]rune(s)) < 30 {
		return false
	}
	lower := strings.ToLower(s)
	for _, bad := range []string{"in this chapter", "this chapter", "the chapter"} {
		if strings.HasPrefix(lower, bad) {
			return false
		}
	}
	return true
}

// Summarize generates a chapter summary using the structured EVENTS/CHANGES/PRESSURE format.
// Used by the manual regenerate endpoint and the auto-summarize goroutine. Non-streaming.
// For the background goroutine path with a dynamic token cap, use summarizeWithTokens.
func (s *Service) Summarize(ctx context.Context, userID, projectID uuid.UUID, provider, text string) (string, adapters.Usage, error) {
	adapter, err := s.getAdapter(ctx, userID, provider)
	if err != nil {
		return "", adapters.Usage{}, fmt.Errorf("get adapter: %w", err)
	}
	genre := ""
	if projectID != uuid.Nil {
		if p, err := s.queries.GetProject(ctx, projectID); err == nil && len(p.Genres) > 0 {
			genre = strings.Join(p.Genres, "/")
		}
	}
	summary, usage, err := adapter.Summarize(ctx, text, summarizeSystemPrompt(genre))
	s.recordUsage(projectID, userID, adapter.Provider(), usage, "summarize", "", uuid.Nil)
	return summary, usage, err
}

// summarizeWithTokens is like Summarize but with a dynamic max-token cap tuned
// to the chapter's scene count. Used by the background debounce goroutine.
// provider is always auto-selected ("") since background calls have no user context.
func (s *Service) summarizeWithTokens(ctx context.Context, userID, projectID uuid.UUID, text string, _ int) (string, adapters.Usage, error) {
	// maxTokens hint is passed to the adapter indirectly via adapter config defaults;
	// the structured format naturally produces shorter output than the old prompt.
	return s.Summarize(ctx, userID, projectID, "", text)
}

// ── internal helpers ──────────────────────────────────────────────────────────

type resolvedContext struct {
	Title  string
	Genres []string
}

type resolvedScene struct {
	Content       string
	Tense         string
	Pov           string
	Summary       string // writer's intended plan for the scene (from SceneMetadataPanel)
	SceneRole     string
	SceneGoal     string
	SceneConflict string
}

// UsageSummary is the public response for GET /projects/:id/ai/usage.
type UsageSummary struct {
	TotalTokens    int64   `json:"total_tokens"`
	TotalCostUSD   float64 `json:"total_cost_usd"`
	MonthlyTokens  int64   `json:"monthly_tokens"`
	MonthlyCostUSD float64 `json:"monthly_cost_usd"`
	CallsThisMonth int64   `json:"calls_this_month"`
}

// ChapterSummaryResponse is the response type for chapter summary endpoints.
type ChapterSummaryResponse struct {
	ChapterID  string `json:"chapter_id"`
	BranchName string `json:"branch_name"`
	AiSummary  string `json:"ai_summary"`
	Stale      bool   `json:"stale"`
}

// GetChapterSummary returns the stored summary for (chapterID, branchName),
// falling back to "canon" if the branch has no row yet.
func (s *Service) GetChapterSummary(ctx context.Context, chapterID uuid.UUID, branchName string) (*ChapterSummaryResponse, error) {
	row, err := s.queries.GetChapterSummary(ctx, sqlcgen.GetChapterSummaryParams{
		ChapterID:  chapterID,
		BranchName: branchName,
	})
	if err != nil {
		// Fall back to canon if the active branch has no row.
		if branchName != canonBranch {
			row, err = s.queries.GetChapterSummary(ctx, sqlcgen.GetChapterSummaryParams{
				ChapterID:  chapterID,
				BranchName: canonBranch,
			})
		}
		if err != nil {
			// No summary exists yet — return empty rather than 404.
			return &ChapterSummaryResponse{
				ChapterID:  chapterID.String(),
				BranchName: branchName,
				AiSummary:  "",
				Stale:      true,
			}, nil
		}
	}
	return &ChapterSummaryResponse{
		ChapterID:  row.ChapterID.String(),
		BranchName: row.BranchName,
		AiSummary:  row.AiSummary,
		Stale:      row.Stale,
	}, nil
}

// RegenerateChapterSummary forces a synchronous LLM summarization of the
// chapter, stores the result, and returns the new summary text.
func (s *Service) RegenerateChapterSummary(ctx context.Context, userID, chapterID, projectID uuid.UUID, branchName string) (string, error) {
	scenes, err := s.queries.ListScenesByChapter(ctx, chapterID)
	if err != nil {
		return "", fmt.Errorf("list scenes: %w", err)
	}
	if len(scenes) == 0 {
		return "", apperror.Validation("chapter has no scenes to summarize")
	}

	// Build chapter header and scene-separated content (same format as background path).
	chapterHeader := buildChapterPositionHeader(ctx, s, chapterID, projectID)

	var sb strings.Builder
	if chapterHeader != "" {
		sb.WriteString(chapterHeader)
		sb.WriteString("\n\n")
	}
	for i, sc := range scenes {
		if i > 0 {
			sb.WriteString("\n\n---\n\n")
		}
		if sc.Title != "" {
			sb.WriteString(fmt.Sprintf("## Scene: %s\n\n", sc.Title))
		}
		sb.WriteString(s.readSceneContent(ctx, sc.ChapterID, sc.ID))
	}

	combined := strings.TrimSpace(sb.String())
	if combined == "" {
		return "", apperror.Validation("chapter scenes have no readable content — save scenes first")
	}

	summary, _, err := s.Summarize(ctx, userID, projectID, "", combined)
	if err != nil {
		return "", fmt.Errorf("summarize: %w", err)
	}

	if err := s.queries.UpsertChapterSummary(ctx, sqlcgen.UpsertChapterSummaryParams{
		ChapterID:  chapterID,
		BranchName: branchName,
		AiSummary:  summary,
	}); err != nil {
		slog.Warn("ai: upsert chapter summary failed", "chapter_id", chapterID, "error", err)
	}

	return summary, nil
}

// BeatHistoryEntry is a single entry in the prompt history browser.
type BeatHistoryEntry struct {
	ID        string `json:"id"`
	BeatText  string `json:"beat_text"`
	SceneID   string `json:"scene_id,omitempty"`
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
}

// GetBeatHistory returns deduplicated recent beats for the project, newest first per unique text.
func (s *Service) GetBeatHistory(ctx context.Context, projectID uuid.UUID) ([]BeatHistoryEntry, error) {
	rows, err := s.queries.ListBeatHistory(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list beat history: %w", err)
	}
	out := make([]BeatHistoryEntry, 0, len(rows))
	for _, r := range rows {
		entry := BeatHistoryEntry{
			ID:        r.ID.String(),
			BeatText:  r.BeatText,
			Model:     r.Model,
			CreatedAt: r.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		}
		if r.SceneID.Valid {
			entry.SceneID = uuid.UUID(r.SceneID.Bytes).String()
		}
		out = append(out, entry)
	}
	return out, nil
}

// GetUsageSummary returns aggregate token usage for a project.
func (s *Service) GetUsageSummary(ctx context.Context, projectID uuid.UUID) (UsageSummary, error) {
	row, err := s.queries.GetProjectUsageSummary(ctx, projectID)
	if err != nil {
		return UsageSummary{}, fmt.Errorf("get usage summary: %w", err)
	}
	// sqlc types COALESCE(SUM(NUMERIC)) as interface{}; pgx scans it as pgtype.Numeric.
	return UsageSummary{
		TotalTokens:    row.TotalTokens,
		TotalCostUSD:   numericToFloat64(row.TotalCostUsd),
		MonthlyTokens:  row.MonthlyTokens,
		MonthlyCostUSD: numericToFloat64(row.MonthlyCostUsd),
		CallsThisMonth: row.CallsThisMonth,
	}, nil
}

// numericToFloat64 converts a pgtype.Numeric scanned as interface{} to float64.
func numericToFloat64(v interface{}) float64 {
	if n, ok := v.(pgtype.Numeric); ok {
		f, err := n.Float64Value()
		if err == nil && f.Valid {
			return f.Float64
		}
	}
	return 0
}

// recordUsage inserts a usage row non-blocking. Errors are logged and discarded
// so they never block or fail the parent AI call.
// mode is one of: "beat", "continue", "chat", "summarize", "portrait".
// beatText is the writer's beat sentence (mode=beat only; empty otherwise).
// sceneID is the scene in focus at call time (uuid.Nil when not applicable).
func (s *Service) recordUsage(projectID, userID uuid.UUID, model string, usage adapters.Usage, mode, beatText string, sceneID uuid.UUID) {
	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 && usage.CostUSD == 0 {
		return // nothing to record (e.g. streaming aborted before tokens known)
	}
	go func() {
		// Build a pgtype.Numeric from the float64 cost via string scan.
		var cost pgtype.Numeric
		if err := cost.Scan(fmt.Sprintf("%.8f", usage.CostUSD)); err != nil {
			_ = cost.Scan("0")
		}
		pgSceneID := pgtype.UUID{Valid: false}
		if sceneID != uuid.Nil {
			pgSceneID = pgtype.UUID{Bytes: [16]byte(sceneID), Valid: true}
		}
		if err := s.queries.InsertUsage(context.Background(), sqlcgen.InsertUsageParams{
			UserID:           userID,
			ProjectID:        projectID,
			Model:            model,
			PromptTokens:     int32(usage.PromptTokens),
			CompletionTokens: int32(usage.CompletionTokens),
			CostUsd:          cost,
			Mode:             mode,
			BeatText:         beatText,
			SceneID:          pgSceneID,
		}); err != nil {
			slog.Warn("ai: failed to record usage", "error", err)
		}
	}()
}

// fetchStyleContent loads a project_prompt by ID and returns its content field.
// This is the style guidance text (not system_content, which is a full system-prompt
// replacement only appropriate for beat/continue mode). Returns "" on any error or when
// the preset has no content — callers can unconditionally append the result.
func (s *Service) fetchStyleContent(ctx context.Context, promptID uuid.UUID) string {
	if promptID == uuid.Nil {
		return ""
	}
	p, err := s.queries.GetProjectPrompt(ctx, promptID)
	if err != nil {
		return ""
	}
	return p.Content
}

// applyPromptPreset modifies adapterReq in place according to the stored preset:
//   - system_content (non-empty) replaces the generated system prompt
//   - content (non-empty) is appended to the user turn as a style guidance block
//
// Errors are non-fatal: if the preset cannot be loaded we continue with the
// default prompts and log a warning.
func (s *Service) applyPromptPreset(ctx context.Context, promptID uuid.UUID, req *adapters.CompleteRequest) {
	p, err := s.queries.GetProjectPrompt(ctx, promptID)
	if err != nil {
		if err != pgx.ErrNoRows {
			slog.Warn("ai: could not load prompt preset", "prompt_id", promptID, "error", err)
		}
		return
	}
	if p.SystemContent != "" {
		req.SystemPrompt = p.SystemContent
	}
	if p.Content != "" {
		req.Content += "\n\n---\nStyle guidance: " + p.Content
	}
}

// buildSceneDirective assembles the short directive block that tells the model
// the scene's structural purpose (role, goal, conflict) and any open threads.
// All fields are optional; the block is omitted entirely when nothing is set.
func (s *Service) buildSceneDirective(ctx context.Context, projectID uuid.UUID, scene resolvedScene) string {
	var lines []string
	if scene.Summary != "" {
		lines = append(lines, "Author's plan for this scene: "+scene.Summary)
	}
	if scene.SceneRole != "" {
		lines = append(lines, "This is a "+scene.SceneRole+" scene.")
	}
	if scene.SceneGoal != "" {
		lines = append(lines, "The POV character's goal: "+scene.SceneGoal+".")
	}
	if scene.SceneConflict != "" {
		lines = append(lines, "What's in the way: "+scene.SceneConflict+".")
	}

	if projectID != uuid.Nil {
		threads, err := s.queries.ListOpenThreadsByProject(ctx, projectID)
		if err == nil && len(threads) > 0 {
			titles := make([]string, 0, 5)
			for i, t := range threads {
				if i >= 5 {
					break
				}
				titles = append(titles, t.Title)
			}
			lines = append(lines, "Open threads: "+strings.Join(titles, ", ")+".")
		}
	}

	if len(lines) == 0 {
		return ""
	}
	return "## This scene\n" + strings.Join(lines, "\n")
}

func (s *Service) resolveContext(ctx context.Context, projectID, sceneID uuid.UUID) (resolvedContext, resolvedScene, error) {
	var proj resolvedContext
	var scene resolvedScene

	if projectID != uuid.Nil {
		p, err := s.queries.GetProject(ctx, projectID)
		if err == nil {
			proj.Title = p.Title
			proj.Genres = p.Genres
		}
	}

	if sceneID != uuid.Nil {
		sc, err := s.queries.GetScene(ctx, sceneID)
		if err == nil {
			scene.Content = s.readSceneContent(ctx, sc.ChapterID, sc.ID)
			scene.Tense = sc.Tense
			scene.Pov = sc.Pov
			scene.Summary = sc.Summary
			attrs := ParseSceneContextAttrs(sc.Attributes)
			scene.SceneRole = attrs.SceneRole
			scene.SceneGoal = attrs.SceneGoal
			scene.SceneConflict = attrs.SceneConflict
		}
	}

	return proj, scene, nil
}
