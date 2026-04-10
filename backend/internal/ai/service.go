package ai

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jconder44/nexustale/internal/ai/adapters"
	"github.com/jconder44/nexustale/internal/auth"
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
	authSvc *auth.Service
	queries *sqlcgen.Queries
	cfg     AIConfig
}

func NewService(authSvc *auth.Service, queries *sqlcgen.Queries, cfg AIConfig) *Service {
	return &Service{authSvc: authSvc, queries: queries, cfg: cfg}
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
		OllamaURL:   s.cfg.OllamaURL,
		OllamaModel: s.cfg.OllamaModel,
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
	Mode        adapters.CompleteMode // "continue" | "beat"
	Beat        string                // required when mode=beat
	Instruction string                // optional hint for continue mode
	Provider    string                // optional — auto-selected if empty
	MaxTokens   int                   // 0 = use config default
	PromptID    uuid.UUID             // optional writing style preset
}

// ChatRequest is the public request type for chat operations.
type ChatRequest struct {
	ProjectID uuid.UUID
	SceneID   uuid.UUID
	Messages  []adapters.Message
	Provider  string
	MaxTokens int
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

	switch req.Mode {
	case adapters.CompleteModeBeat:
		if maxTok == 0 || maxTok > s.cfg.BeatMaxTokens {
			adapterReq.MaxTokens = s.cfg.BeatMaxTokens
		}
		adapterReq.SystemPrompt = beatSystemPrompt(project.Title, project.Genres, scene.Tense, scene.Pov, "")
		adapterReq.Content = req.Beat
	default:
		adapterReq.SystemPrompt = continueSystemPrompt(project.Title, project.Genres, scene.Tense, scene.Pov)
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

	return adapter.StreamComplete(ctx, adapterReq, w)
}

// StreamChat streams a chat response to w.
func (s *Service) StreamChat(ctx context.Context, userID uuid.UUID, req ChatRequest, w io.Writer) (adapters.Usage, error) {
	adapter, err := s.getAdapter(ctx, userID, req.Provider)
	if err != nil {
		return adapters.Usage{}, fmt.Errorf("get adapter: %w", err)
	}

	maxTok := req.MaxTokens
	if maxTok == 0 {
		maxTok = s.cfg.MaxTokens
	}

	return adapter.StreamChat(ctx, adapters.ChatRequest{
		Messages:  req.Messages,
		MaxTokens: maxTok,
	}, w)
}

// Summarize generates a 2–3 sentence summary of the provided text.
// Used by the auto-summarize goroutine in B2. Non-streaming.
func (s *Service) Summarize(ctx context.Context, userID uuid.UUID, provider, text string) (string, adapters.Usage, error) {
	adapter, err := s.getAdapter(ctx, userID, provider)
	if err != nil {
		return "", adapters.Usage{}, fmt.Errorf("get adapter: %w", err)
	}
	return adapter.Summarize(ctx, text)
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
