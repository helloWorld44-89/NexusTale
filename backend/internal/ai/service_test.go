package ai

import (
	"strings"
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

func TestSceneEndingExcerpt(t *testing.T) {
	t.Run("short content unchanged", func(t *testing.T) {
		got := sceneEndingExcerpt("hello", 100)
		if got != "hello" {
			t.Errorf("got %q, want %q", got, "hello")
		}
	})
	t.Run("long content truncated to tail", func(t *testing.T) {
		content := strings.Repeat("a", 50) + strings.Repeat("b", 50)
		got := sceneEndingExcerpt(content, 50)
		if got != strings.Repeat("b", 50) {
			t.Errorf("expected tail of bs, got %q", got[:10])
		}
	})
}

func TestSplitSceneContent(t *testing.T) {
	t.Run("short content: empty head, full tail", func(t *testing.T) {
		head, tail := splitSceneContent("short", 100, 20)
		if head != "" {
			t.Errorf("head should be empty, got %q", head)
		}
		if tail != "short" {
			t.Errorf("tail = %q, want %q", tail, "short")
		}
	})
	t.Run("long content: head excerpt + tail", func(t *testing.T) {
		head := strings.Repeat("h", 200) // head portion
		tail := strings.Repeat("t", 100) // tail portion
		content := head + tail
		h, tl := splitSceneContent(content, 100, 50)
		if tl != tail {
			t.Errorf("tail mismatch")
		}
		// head excerpt is 50 runes + ellipsis
		if len([]rune(h)) != 51 || !strings.HasSuffix(h, "…") {
			t.Errorf("head excerpt wrong: len=%d suffix=%q", len([]rune(h)), string([]rune(h)[len([]rune(h))-1]))
		}
	})
	t.Run("head fits within excerpt limit", func(t *testing.T) {
		content := strings.Repeat("h", 30) + strings.Repeat("t", 100)
		h, _ := splitSceneContent(content, 100, 50)
		// head is only 30 runes — no ellipsis needed
		if strings.Contains(h, "…") {
			t.Errorf("unexpected ellipsis when head fits limit")
		}
	})
}

func TestApplyWorkshopHistoryWindow(t *testing.T) {
	t.Run("under limit unchanged", func(t *testing.T) {
		input := msgs("user", "assistant", "user")
		got := applyWorkshopHistoryWindow(input, 12)
		if len(got) != 3 {
			t.Fatalf("len = %d, want 3", len(got))
		}
	})

	t.Run("over limit injects digest", func(t *testing.T) {
		// 14 messages with alternating roles: head=2, tail=12
		input := make([]adapters.Message, 14)
		for i := range input {
			if i%2 == 0 {
				input[i] = adapters.Message{Role: "user", Content: "user msg"}
			} else {
				input[i] = adapters.Message{Role: "assistant", Content: "assistant msg"}
			}
		}
		got := applyWorkshopHistoryWindow(input, 12)
		// digest (user) + ack (assistant, because tail[0] is user) + 12 tail = 14
		if len(got) != 14 {
			t.Fatalf("len = %d, want 14", len(got))
		}
		if got[0].Role != "user" {
			t.Errorf("first message role = %q, want user", got[0].Role)
		}
		if got[1].Role != "assistant" {
			t.Errorf("second message role = %q, want assistant (ack)", got[1].Role)
		}
		if got[0].Content[:len("[Earlier in this session:]")] != "[Earlier in this session:]" {
			t.Errorf("digest missing prefix: %q", got[0].Content[:40])
		}
	})

	t.Run("over limit tail starts with assistant no extra ack", func(t *testing.T) {
		// head=1 (user), tail=1 (assistant) — tail[0] is assistant so no ack needed
		input := []adapters.Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi there"},
		}
		got := applyWorkshopHistoryWindow(input, 1)
		// digest (user) + tail[0] (assistant) = 2
		if len(got) != 2 {
			t.Fatalf("len = %d, want 2", len(got))
		}
		if got[0].Role != "user" {
			t.Errorf("first role = %q, want user", got[0].Role)
		}
		if got[1].Role != "assistant" {
			t.Errorf("second role = %q, want assistant", got[1].Role)
		}
	})

	t.Run("digest truncates long turns", func(t *testing.T) {
		long := strings.Repeat("x", 500)
		input := []adapters.Message{
			{Role: "user", Content: long},
			{Role: "assistant", Content: "short"},
			{Role: "user", Content: "new question"},
		}
		got := applyWorkshopHistoryWindow(input, 1)
		// digest content should contain the truncation ellipsis
		if !strings.Contains(got[0].Content, "…") {
			t.Errorf("expected truncation ellipsis in digest, got: %q", got[0].Content[:50])
		}
	})
}
