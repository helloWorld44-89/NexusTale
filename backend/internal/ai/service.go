package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jconder44/nexustale/internal/ai/adapters"
	"github.com/jconder44/nexustale/internal/auth"
	"github.com/jconder44/nexustale/pkg/apperror"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
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
type Service struct {
	authSvc  *auth.Service
	queries  *sqlcgen.Queries
	cfg      AIConfig
	debounce *debouncer
}

func NewService(authSvc *auth.Service, queries *sqlcgen.Queries, cfg AIConfig) *Service {
	return &Service{
		authSvc:  authSvc,
		queries:  queries,
		cfg:      cfg,
		debounce: newDebouncer(),
	}
}

// ── adapter resolution ────────────────────────────────────────────────────────

// providerPreference is the order in which stored keys are tried when the
// caller does not specify a preferred provider.
var providerPreference = []string{"anthropic", "openai"}

// getAdapter resolves the adapter for the requesting user.
// If provider is non-empty it is used directly; otherwise keys are tried in
// providerPreference order, falling back to Ollama if none are stored.
func (s *Service) getAdapter(ctx context.Context, userID uuid.UUID, provider string) (adapters.Adapter, error) {
	adapterCfg := adapters.AdapterConfig{
		OllamaURL:   s.ollamaURLForUser(ctx, userID),
		OllamaModel: s.ollamaModelForUser(ctx, userID),
	}

	tryProvider := func(p string) (adapters.Adapter, bool) {
		key, err := s.authSvc.DecryptAPIKey(ctx, userID, p)
		if err != nil {
			return nil, false
		}
		adapter, err := adapters.NewAdapter(p, key, "", adapterCfg)
		if err != nil {
			return nil, false
		}
		return adapter, true
	}

	// Explicit provider requested.
	if provider != "" {
		key, err := s.authSvc.DecryptAPIKey(ctx, userID, provider)
		if err != nil {
			// No key stored → Ollama fallback.
			return adapters.NewAdapter("ollama", "", "", adapterCfg)
		}
		return adapters.NewAdapter(provider, key, "", adapterCfg)
	}

	// Auto-select from stored keys.
	for _, p := range providerPreference {
		if adapter, ok := tryProvider(p); ok {
			return adapter, nil
		}
	}

	// No cloud key found — use Ollama.
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

// ── beat system prompt ────────────────────────────────────────────────────────

// beatSystemPrompt builds the system prompt for beat-expansion mode.
// It substitutes project and scene metadata into the template.
func beatSystemPrompt(title string, genres []string, tense, pov, povCharacter string) string {
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

	sb.WriteString("Given a story beat (what should happen next), write 2–3 paragraphs ")
	sb.WriteString("that bring the beat to life. Match the author's tone and style. ")
	sb.WriteString("Use sensory details. Show, don't tell.")
	return sb.String()
}

// continueSystemPrompt is the default system prompt for continue mode.
func continueSystemPrompt(title string, genres []string, tense, pov string) string {
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
	sb.WriteString("\nContinue the story naturally from where it left off.")
	return sb.String()
}

// ── public API ────────────────────────────────────────────────────────────────

// CompleteRequest is the public request type for the Complete/StreamComplete operations.
type CompleteRequest struct {
	ProjectID   uuid.UUID
	SceneID     uuid.UUID
	BranchName  string                // active Timeline; empty = resolved via ResolveBranch
	Mode        adapters.CompleteMode // "continue" | "beat"
	Beat        string                // required when mode=beat
	Instruction string                // optional hint for continue mode
	Provider    string                // optional — auto-selected if empty
	MaxTokens   int                   // 0 = use config default
	PromptID    uuid.UUID             // optional writing style preset
}

// ChatRequest is the public request type for chat operations.
type ChatRequest struct {
	ProjectID            uuid.UUID
	SceneID              uuid.UUID
	BranchName           string // active Timeline
	Messages             []adapters.Message
	Provider             string
	MaxTokens            int
	SystemPromptOverride string // if non-empty, replaces the default Nexus identity prompt
}

// StreamComplete streams the AI response for scene continuation or beat expansion.
// It writes NexusTale SSE format to w.
func (s *Service) StreamComplete(ctx context.Context, userID uuid.UUID, req CompleteRequest, w io.Writer) (adapters.Usage, error) {
	adapter, err := s.getAdapter(ctx, userID, req.Provider)
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
	ctxBlock := s.BuildContext(ctx, req.ProjectID, userID, req.BranchName, scene.Content, req.SceneID)

	switch req.Mode {
	case adapters.CompleteModeBeat:
		if maxTok == 0 || maxTok > s.cfg.BeatMaxTokens {
			adapterReq.MaxTokens = s.cfg.BeatMaxTokens
		}
		sysPrompt := beatSystemPrompt(project.Title, project.Genres, scene.Tense, scene.Pov, "")
		if ctxBlock != "" {
			sysPrompt = ctxBlock + "\n\n" + sysPrompt
		}
		adapterReq.SystemPrompt = sysPrompt
		adapterReq.Content = req.Beat
	default:
		sysPrompt := continueSystemPrompt(project.Title, project.Genres, scene.Tense, scene.Pov)
		if ctxBlock != "" {
			sysPrompt = ctxBlock + "\n\n" + sysPrompt
		}
		adapterReq.SystemPrompt = sysPrompt
		content := scene.Content
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

// StreamChat streams a chat response to w.
// A context window (chapter summaries + @[entity] snippets) is injected as a
// system message prepended to the conversation.
func (s *Service) StreamChat(ctx context.Context, userID uuid.UUID, req ChatRequest, w io.Writer) (adapters.Usage, error) {
	adapter, err := s.getAdapter(ctx, userID, req.Provider)
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
		if sc, err := s.queries.GetScene(ctx, req.SceneID); err == nil {
			sceneContent = sc.Content
		}
	}

	messages := req.Messages

	// Build the context window (project identity + chapter content + @[entity] snippets + pins).
	ctxBlock := s.BuildContext(ctx, req.ProjectID, userID, req.BranchName, sceneContent, req.SceneID)

	// Build the identity+context system prompt.
	// SystemPromptOverride lets callers (e.g. Workshop) substitute a different
	// persona or craft focus without changing the context block.
	var nexusSystem string
	if req.SystemPromptOverride != "" {
		nexusSystem = req.SystemPromptOverride + "\n\n" + ctxBlock
	} else {
		nexusSystem = "You are Nexus, an AI co-author and story intelligence embedded in NexusTale. " +
			"You have full access to this project's chapters, wiki, and timeline. " +
			"Answer questions about the story accurately, help develop the narrative, suggest improvements, " +
			"and assist with writing. Be concise unless the user asks for detail.\n\n" + ctxBlock
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
//  1. Executed against the database.
//  2. Emitted as an SSE event: data: {"tool":"name","result":"..."}\n\n
//     (the frontend SSE parser ignores events without a "delta" key, so these
//     pass through safely without any frontend changes.)
//  3. Fed back to the model as a tool result.
//
// The loop runs for at most 10 rounds. The final text is streamed as normal
// delta + [DONE] events. Falls back to StreamChat if the adapter does not
// implement ToolAdapter (Ollama).
func (s *Service) StreamChatWithTools(ctx context.Context, userID uuid.UUID, req ChatRequest, w io.Writer) (adapters.Usage, error) {
	adapter, err := s.getAdapter(ctx, userID, req.Provider)
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
		if sc, err := s.queries.GetScene(ctx, req.SceneID); err == nil {
			sceneContent = sc.Content
		}
	}

	ctxBlock := s.BuildContext(ctx, req.ProjectID, userID, req.BranchName, sceneContent, req.SceneID)

	var nexusSystem string
	if req.SystemPromptOverride != "" {
		nexusSystem = req.SystemPromptOverride + "\n\n" + ctxBlock
	} else {
		nexusSystem = "You are Nexus, an AI co-author and story intelligence embedded in NexusTale. " +
			"You have full access to this project's chapters, wiki, and timeline. " +
			"You may use tools to write directly to the manuscript — appending to scenes, " +
			"replacing their content, or creating new scenes, chapters, and acts. " +
			"When the author asks you to write, expand, or create story content, use the appropriate tool. " +
			"After each tool call, briefly confirm what you did and what comes next.\n\n" + ctxBlock
	}

	messages := append([]adapters.Message{{Role: "system", Content: nexusSystem}}, req.Messages...)

	var extraMsgs []json.RawMessage
	var totalUsage adapters.Usage
	const maxRounds = 10
	finalText := ""

	for round := 0; round < maxRounds; round++ {
		resp, err := ta.ChatTools(ctx, messages, extraMsgs, ManuscriptTools, maxTok)
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
			result := s.executeToolCall(ctx, req.ProjectID, tc)
			toolResults = append(toolResults, result)

			// Emit a tool SSE event so the frontend can show progress.
			// The existing SSE parsers check for "delta" before calling onDelta,
			// so this event is silently ignored by parsers that don't handle it.
			evtPayload, _ := json.Marshal(map[string]string{"tool": tc.Name, "result": result.Content})
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

// Summarize generates a 2–3 sentence summary of the provided text.
// Used by the auto-summarize goroutine in B2. Non-streaming.
func (s *Service) Summarize(ctx context.Context, userID uuid.UUID, provider, text string) (string, adapters.Usage, error) {
	adapter, err := s.getAdapter(ctx, userID, provider)
	if err != nil {
		return "", adapters.Usage{}, fmt.Errorf("get adapter: %w", err)
	}
	summary, usage, err := adapter.Summarize(ctx, text)
	s.recordUsage(uuid.Nil, userID, adapter.Provider(), usage, "summarize", "", uuid.Nil)
	return summary, usage, err
}

// ── internal helpers ──────────────────────────────────────────────────────────

type resolvedContext struct {
	Title  string
	Genres []string
}

type resolvedScene struct {
	Content string
	Tense   string
	Pov     string
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
func (s *Service) RegenerateChapterSummary(ctx context.Context, userID, chapterID uuid.UUID, branchName string) (string, error) {
	scenes, err := s.queries.ListScenesByChapter(ctx, chapterID)
	if err != nil {
		return "", fmt.Errorf("list scenes: %w", err)
	}
	if len(scenes) == 0 {
		return "", apperror.Validation("chapter has no scenes to summarize")
	}

	var sb strings.Builder
	for i, sc := range scenes {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(sc.Content)
	}

	summary, _, err := s.Summarize(ctx, userID, "", sb.String())
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
// mode is one of: "beat", "continue", "chat", "summarize".
// beatText is the writer's beat sentence (mode=beat only; empty otherwise).
// sceneID is the scene in focus at call time (uuid.Nil when not applicable).
func (s *Service) recordUsage(projectID, userID uuid.UUID, model string, usage adapters.Usage, mode, beatText string, sceneID uuid.UUID) {
	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
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
			scene.Content = sc.Content
			scene.Tense = sc.Tense
			scene.Pov = sc.Pov
		}
	}

	return proj, scene, nil
}
