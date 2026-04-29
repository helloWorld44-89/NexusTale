package ai

import (
	"testing"

	"github.com/jconder44/nexustale/internal/ai/adapters"
)

func msgs(roles ...string) []adapters.Message {
	out := make([]adapters.Message, len(roles))
	for i, r := range roles {
		out[i] = adapters.Message{Role: r, Content: r}
	}
	return out
}

func TestApplyHistoryWindow(t *testing.T) {
	tests := []struct {
		name     string
		input    []adapters.Message
		max      int
		wantLen  int
		wantLast string // Role of the last message
	}{
		{"empty", msgs(), 12, 0, ""},
		{"under limit", msgs("user", "assistant", "user"), 12, 3, "user"},
		{"exactly limit", msgs("user", "assistant", "user", "assistant"), 4, 4, "assistant"},
		{"over limit keeps tail", msgs("user", "assistant", "user", "assistant", "user"), 4, 4, "user"},
		{"over limit drops head", msgs("a", "b", "c", "d", "e"), 3, 3, "e"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyHistoryWindow(tt.input, tt.max)
			if len(got) != tt.wantLen {
				t.Errorf("len = %d, want %d", len(got), tt.wantLen)
			}
			if tt.wantLast != "" && got[len(got)-1].Role != tt.wantLast {
				t.Errorf("last role = %q, want %q", got[len(got)-1].Role, tt.wantLast)
			}
		})
	}
}

func TestApplyHistoryWindowNoMutation(t *testing.T) {
	// Verify the original slice is not mutated when trimming.
	original := msgs("a", "b", "c", "d", "e")
	_ = applyHistoryWindow(original, 3)
	if len(original) != 5 {
		t.Errorf("original mutated: len = %d, want 5", len(original))
	}
}
