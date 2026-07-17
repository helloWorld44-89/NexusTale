package ai

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestBuildPortraitPromptCharacterWithAttrs(t *testing.T) {
	e := entityRow{
		ID:         uuid.New(),
		Type:       "character",
		Name:       "Kaelen",
		Summary:    "A weary starship captain.",
		Attributes: []byte(`{"motivation":"protect her crew at any cost"}`),
	}

	got := buildPortraitPrompt(e, "", "")
	if !strings.Contains(got, "Kaelen") {
		t.Errorf("expected prompt to mention entity name, got %q", got)
	}
	if !strings.Contains(got, "protect her crew") {
		t.Errorf("expected prompt to include motivation, got %q", got)
	}
	if !strings.Contains(got, "weary starship captain") {
		t.Errorf("expected prompt to include summary, got %q", got)
	}
}

func TestBuildPortraitPromptCharacterNoAttrs(t *testing.T) {
	e := entityRow{
		ID:      uuid.New(),
		Type:    "character",
		Name:    "Doss",
		Summary: "A quiet blacksmith.",
	}

	got := buildPortraitPrompt(e, "", "")
	if !strings.Contains(got, "Doss") || !strings.Contains(got, "blacksmith") {
		t.Errorf("expected name + summary in prompt, got %q", got)
	}
}

func TestBuildPortraitPromptNonCharacter(t *testing.T) {
	e := entityRow{
		ID:      uuid.New(),
		Type:    "location",
		Name:    "The Hollow Spire",
		Summary: "A crumbling watchtower on the coast.",
	}

	got := buildPortraitPrompt(e, "", "")
	if !strings.Contains(got, "The Hollow Spire") || !strings.Contains(got, "location") {
		t.Errorf("expected name + type in prompt, got %q", got)
	}
	if !strings.Contains(got, "crumbling watchtower") {
		t.Errorf("expected summary in prompt, got %q", got)
	}
}

func TestBuildPortraitPromptWithUserGuidance(t *testing.T) {
	e := entityRow{
		ID:   uuid.New(),
		Type: "item",
		Name: "Ashfall Blade",
	}

	got := buildPortraitPrompt(e, "glowing runes along the edge", "")
	if !strings.Contains(got, "Additional guidance: glowing runes along the edge") {
		t.Errorf("expected user guidance appended, got %q", got)
	}
}

func TestBuildPortraitPromptCapabilityNotesIncluded(t *testing.T) {
	e := entityRow{
		ID:         uuid.New(),
		Type:       "character",
		Name:       "Rook",
		Attributes: []byte(`{"capability_notes":"master alchemist, always carries vials of reagents"}`),
	}

	got := buildPortraitPrompt(e, "", "")
	if !strings.Contains(got, "master alchemist") {
		t.Errorf("expected capability_notes in prompt, got %q", got)
	}
}

func TestBuildPortraitPromptArtStyleAppended(t *testing.T) {
	e := entityRow{
		ID:   uuid.New(),
		Type: "character",
		Name: "Vex",
	}

	got := buildPortraitPrompt(e, "", "digital painting, muted fantasy palette")
	if !strings.Contains(got, "Visual style guidance: digital painting, muted fantasy palette") {
		t.Errorf("expected art style guidance appended, got %q", got)
	}
}

func TestBuildPortraitPromptArtStyleTruncated(t *testing.T) {
	e := entityRow{ID: uuid.New(), Type: "character", Name: "Vex"}
	long := strings.Repeat("style ", 200) // well over artStyleExcerptRunes

	got := buildPortraitPrompt(e, "", long)
	idx := strings.Index(got, "Visual style guidance: ")
	if idx == -1 {
		t.Fatalf("expected art style section, got %q", got)
	}
	styleSection := got[idx+len("Visual style guidance: "):]
	// truncateRunes appends a "…" marker, so the excerpt is n runes + 1.
	if want := artStyleExcerptRunes + 1; len([]rune(styleSection)) > want {
		t.Errorf("expected art style excerpt capped at %d runes (incl. truncation marker), got %d", want, len([]rune(styleSection)))
	}
	if !strings.HasSuffix(styleSection, "…") {
		t.Errorf("expected truncated style guidance to end with the truncation marker, got %q", styleSection)
	}
}
