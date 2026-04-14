package ai

import (
	"context"
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
	ProjectID  uuid.UUID
	SceneID    uuid.UUID
	BranchName string // active Timeline
	Messages   []adapters.Message
	Provider   string
	MaxTokens  int
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

	// Build the AI context window (project identity + chapter content + @[entity] snippets).
	ctxBlock := s.BuildContext(ctx, req.ProjectID, req.BranchName, scene.Content, req.SceneID)

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
	s.recordUsage(req.ProjectID, userID, adapter.Provider(), usage)
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

	// Build the context window (project identity + chapter content + @[entity] snippets).
	ctxBlock := s.BuildContext(ctx, req.ProjectID, req.BranchName, sceneContent, req.SceneID)

	// Always prepend a system message so the model has an identity and story context.
	// The context block already includes the project title and all chapter content,
	// so no further project metadata fetch is needed here.
	nexusSystem := "You are Nexus, an AI co-author and story intelligence embedded in NexusTale. " +
		"You have full access to this project's chapters, wiki, and timeline. " +
		"Answer questions about the story accurately, help develop the narrative, suggest improvements, " +
		"and assist with writing. Be concise unless the user asks for detail.\n\n" + ctxBlock

	messages = append([]adapters.Message{{Role: "system", Content: nexusSystem}}, messages...)

	usage, err := adapter.StreamChat(ctx, adapters.ChatRequest{
		Messages:  messages,
		MaxTokens: maxTok,
	}, w)
	s.recordUsage(req.ProjectID, userID, adapter.Provider(), usage)
	return usage, err
}

// Summarize generates a 2–3 sentence summary of the provided text.
// Used by the auto-summarize goroutine in B2. Non-streaming.
func (s *Service) Summarize(ctx context.Context, userID uuid.UUID, provider, text string) (string, adapters.Usage, error) {
	adapter, err := s.getAdapter(ctx, userID, provider)
	if err != nil {
		return "", adapters.Usage{}, fmt.Errorf("get adapter: %w", err)
	}
	summary, usage, err := adapter.Summarize(ctx, text)
	s.recordUsage(uuid.Nil, userID, adapter.Provider(), usage)
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
func (s *Service) recordUsage(projectID, userID uuid.UUID, model string, usage adapters.Usage) {
	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		return // nothing to record (e.g. streaming aborted before tokens known)
	}
	go func() {
		// Build a pgtype.Numeric from the float64 cost via string scan.
		var cost pgtype.Numeric
		if err := cost.Scan(fmt.Sprintf("%.8f", usage.CostUSD)); err != nil {
			// Fallback: zero cost rather than dropping the record entirely.
			_ = cost.Scan("0")
		}
		if err := s.queries.InsertUsage(context.Background(), sqlcgen.InsertUsageParams{
			UserID:           userID,
			ProjectID:        projectID,
			Model:            model,
			PromptTokens:     int32(usage.PromptTokens),
			CompletionTokens: int32(usage.CompletionTokens),
			CostUsd:          cost,
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
