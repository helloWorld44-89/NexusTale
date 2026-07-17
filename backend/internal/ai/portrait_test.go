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

	got := buildPortraitPrompt(e, "")
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

	got := buildPortraitPrompt(e, "")
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

	got := buildPortraitPrompt(e, "")
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

	got := buildPortraitPrompt(e, "glowing runes along the edge")
	if !strings.Contains(got, "Additional guidance: glowing runes along the edge") {
		t.Errorf("expected user guidance appended, got %q", got)
	}
}
