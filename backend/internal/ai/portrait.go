package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jconder44/nexustale/internal/ai/adapters"
	"github.com/jconder44/nexustale/pkg/apperror"
)

// defaultImageProvider is used when the writer hasn't picked one in Settings.
const defaultImageProvider = "openai"

// imageProviderForUser returns the writer's chosen image-generation provider.
// Stored as a pseudo-key on user_api_keys (provider="image_provider", value is
// a provider name), same pattern as ollamaModelForUser/openRouterModelForUser.
func (s *Service) imageProviderForUser(ctx context.Context, userID uuid.UUID) string {
	if provider, err := s.authSvc.DecryptAPIKey(ctx, userID, "image_provider"); err == nil && provider != "" {
		return provider
	}
	return defaultImageProvider
}

// artStyleExcerptRunes caps how much of the project's AI Bible is folded into
// an image prompt — the Bible is prose-oriented and often long; only a short
// excerpt is needed as a visual style hint, not the full narrative brief.
const artStyleExcerptRunes = 300

// buildPortraitPrompt builds an image-generation prompt from an entity's
// existing wiki data plus optional free-text guidance from the writer. This
// is deliberately separate from buildEntityContextLine/buildCharacterContextLine
// in context.go — those are tuned for prose context injection, not visual
// description, and entities without appearance-relevant summaries would
// produce weak image prompts if reused as-is.
//
// Structured character attributes take priority over the generic summary:
// they're specific, writer-curated fields, whereas summary is often narrative
// prose that describes plot role more than physical appearance.
func buildPortraitPrompt(e entityRow, userPrompt, artStyle string) string {
	desc := fmt.Sprintf("A detailed portrait of %s, a %s.", e.Name, e.Type)

	if e.Type == "character" {
		var attrs charContextAttrs
		if len(e.Attributes) > 0 {
			_ = json.Unmarshal(e.Attributes, &attrs)
		}
		if attrs.Motivation != "" {
			desc += " " + attrs.Motivation
		}
		if attrs.CapabilityNotes != "" {
			desc += " " + attrs.CapabilityNotes
		}
	}

	if e.Summary != "" {
		desc += " " + truncateRunes(e.Summary, 500)
	}

	if artStyle != "" {
		desc += "\n\nVisual style guidance: " + truncateRunes(artStyle, artStyleExcerptRunes)
	}

	if userPrompt != "" {
		desc += "\n\nAdditional guidance: " + userPrompt
	}

	return desc
}

// GenerateEntityPortrait generates or revises a portrait image for a wiki
// entity. When referenceImage is nil this is a first draft, built from the
// entity's own data plus prompt as optional style guidance. When
// referenceImage is set, prompt is treated as a revision instruction applied
// against that reference — the entity's descriptive data is not re-injected,
// since the reference image already encodes the visual identity and doing so
// would dilute the edit instruction.
//
// The result is never persisted here — it's returned to the caller as raw
// image bytes for an ephemeral draft; saving goes through the existing wiki
// entity image upload endpoint.
func (s *Service) GenerateEntityPortrait(ctx context.Context, userID, projectID, entityID uuid.UUID, prompt string, referenceImage []byte) (adapters.ImageResult, error) {
	row, err := s.queries.GetEntity(ctx, entityID)
	if err != nil {
		return adapters.ImageResult{}, apperror.NotFound("entity", err.Error())
	}
	e := entityRow(row)
	if e.ProjectID != projectID {
		return adapters.ImageResult{}, apperror.NotFound("entity", "entity does not belong to project")
	}

	finalPrompt := prompt
	if referenceImage == nil {
		var artStyle string
		if proj, err := s.queries.GetProject(ctx, projectID); err == nil {
			artStyle = strings.TrimSpace(proj.AiInstructions)
		}
		finalPrompt = buildPortraitPrompt(e, prompt, artStyle)
	}

	provider := s.imageProviderForUser(ctx, userID)
	key, err := s.authSvc.DecryptAPIKey(ctx, userID, provider)
	if err != nil || key == "" {
		return adapters.ImageResult{}, apperror.Validation("no API key configured for image provider " + provider)
	}

	adapter, err := adapters.NewImageAdapter(provider, key, "")
	if err != nil {
		return adapters.ImageResult{}, apperror.Validation(err.Error())
	}

	result, usage, err := adapter.GenerateImage(ctx, adapters.ImageRequest{
		Prompt:         finalPrompt,
		ReferenceImage: referenceImage,
	})
	if err != nil {
		return adapters.ImageResult{}, apperror.Internal(err.Error())
	}

	s.recordUsage(projectID, userID, provider+"-image", usage, "portrait", "", uuid.Nil)

	return result, nil
}
