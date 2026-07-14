package ai

// fingerprint.go — C9-P5: Prose style fingerprinting.
//
// ExtractProseFingerprint analyses a writer's scene collection and returns a
// compact statistical profile of their prose. The profile is stored in
// projects.prose_fingerprint (JSONB, migration 036) and injected into Beat and
// Continue system prompts as an "## Author's prose style" block so the model
// can match the writer's natural rhythm without manual style configuration.
//
// Statistics computed:
//   - AvgSentenceLength  — mean words per sentence (short ≤8, long ≥20)
//   - AvgParagraphLength — mean sentences per paragraph (short ≤2, long ≥5)
//   - DialogueRatio      — fraction of non-blank lines starting with " (0–1)
//   - AdverbDensity      — fraction of words ending in -ly (0–1)
//   - SentenceVariance   — std-dev of words-per-sentence (low=uniform, high=varied)

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// ProseFingerprint holds the statistical prose profile for a project.
type ProseFingerprint struct {
	AvgSentenceLength  float64 `json:"avg_sentence_length"`
	AvgParagraphLength float64 `json:"avg_paragraph_length"`
	DialogueRatio      float64 `json:"dialogue_ratio"`
	AdverbDensity      float64 `json:"adverb_density"`
	SentenceVariance   float64 `json:"sentence_variance"`
}

// fingerprintMinWords is the minimum total word count before we generate a
// fingerprint — too little text produces noisy statistics.
const fingerprintMinWords = 500

// RefreshProseFingerpint recomputes the prose fingerprint for the project by
// sampling scene content from git. Called after every 3 scene saves via
// ScheduleFingerprintRefresh (same pattern as ScheduleSummarize).
// Non-fatal: errors are logged and the existing fingerprint is unchanged.
func (s *Service) RefreshProseFingerpint(ctx context.Context, projectID uuid.UUID) {
	chapters, err := s.queries.ListChaptersByProject(ctx, projectID)
	if err != nil {
		slog.Warn("ai: fingerprint — list chapters failed", "project_id", projectID, "error", err)
		return
	}

	var allText strings.Builder
	for _, ch := range chapters {
		scenes, err := s.queries.ListScenesByChapter(ctx, ch.ID)
		if err != nil {
			continue
		}
		for _, sc := range scenes {
			content := s.readSceneContent(ctx, ch.ID, sc.ID)
			if content != "" {
				allText.WriteString(content)
				allText.WriteString("\n\n")
			}
		}
	}

	text := strings.TrimSpace(allText.String())
	if text == "" {
		return
	}

	fp, ok := ExtractProseFingerprint(text)
	if !ok {
		return // not enough text
	}

	raw, err := json.Marshal(fp)
	if err != nil {
		slog.Warn("ai: fingerprint — marshal failed", "project_id", projectID, "error", err)
		return
	}

	if err := s.queries.UpdateProseFingerpint(ctx, sqlcgen.UpdateProseFingerpintParams{
		ID:               projectID,
		ProseFingerprint: raw,
	}); err != nil {
		slog.Warn("ai: fingerprint — update failed", "project_id", projectID, "error", err)
	}
}

// ExtractProseFingerprint computes a ProseFingerprint from raw prose text.
// Returns (zero, false) when the text is too short to yield reliable statistics.
func ExtractProseFingerprint(text string) (ProseFingerprint, bool) {
	paragraphs := splitParagraphs(text)
	if len(paragraphs) == 0 {
		return ProseFingerprint{}, false
	}

	var (
		totalWords      int
		totalSentences  int
		sentenceLengths []float64
		dialogueLines   int
		totalLines      int
		adverbWords     int
	)

	for _, para := range paragraphs {
		sentences := splitSentences(para)
		totalSentences += len(sentences)

		for _, sent := range sentences {
			words := strings.Fields(sent)
			wc := len(words)
			totalWords += wc
			sentenceLengths = append(sentenceLengths, float64(wc))

			for _, w := range words {
				w = strings.ToLower(strings.TrimFunc(w, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }))
				if strings.HasSuffix(w, "ly") && len(w) > 4 {
					adverbWords++
				}
			}
		}

		for _, line := range strings.Split(para, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			totalLines++
			if strings.HasPrefix(trimmed, "“") || strings.HasPrefix(trimmed, "\"") {
				dialogueLines++
			}
		}
	}

	if totalWords < fingerprintMinWords {
		return ProseFingerprint{}, false
	}

	avgSentLen := float64(totalWords) / max(float64(totalSentences), 1)
	avgParaLen := float64(totalSentences) / max(float64(len(paragraphs)), 1)
	dialogueRatio := float64(dialogueLines) / max(float64(totalLines), 1)
	adverbDensity := float64(adverbWords) / max(float64(totalWords), 1)
	sentenceVariance := stdDev(sentenceLengths)

	return ProseFingerprint{
		AvgSentenceLength:  round2(avgSentLen),
		AvgParagraphLength: round2(avgParaLen),
		DialogueRatio:      round2(dialogueRatio),
		AdverbDensity:      round4(adverbDensity),
		SentenceVariance:   round2(sentenceVariance),
	}, true
}

// FingerprintContextBlock renders the fingerprint as an "## Author's prose style"
// context block for injection into Beat/Continue system prompts.
// Returns "" when fp is nil or contains no useful signal.
func FingerprintContextBlock(fp *ProseFingerprint) string {
	if fp == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Author's prose style\n")

	// Sentence length register.
	switch {
	case fp.AvgSentenceLength <= 8:
		sb.WriteString("- Short, punchy sentences (avg ~" + fmt.Sprintf("%.0f", fp.AvgSentenceLength) + " words). Favour brevity.\n")
	case fp.AvgSentenceLength >= 20:
		sb.WriteString("- Long, complex sentences (avg ~" + fmt.Sprintf("%.0f", fp.AvgSentenceLength) + " words). Favour subordinate clauses and flow.\n")
	default:
		sb.WriteString("- Mid-length sentences (avg ~" + fmt.Sprintf("%.0f", fp.AvgSentenceLength) + " words).\n")
	}

	// Paragraph density.
	switch {
	case fp.AvgParagraphLength <= 2:
		sb.WriteString("- Short paragraphs (1–2 sentences). Keep paragraphs tight.\n")
	case fp.AvgParagraphLength >= 5:
		sb.WriteString("- Dense paragraphs (5+ sentences). Allow paragraphs to breathe.\n")
	}

	// Sentence rhythm (variance).
	if fp.SentenceVariance < 3 {
		sb.WriteString("- Very consistent sentence rhythm — avoid mixing short and long sentences randomly.\n")
	} else if fp.SentenceVariance > 8 {
		sb.WriteString("- Highly varied sentence rhythm — mix short punches with longer flows.\n")
	}

	// Dialogue ratio.
	switch {
	case fp.DialogueRatio > 0.4:
		sb.WriteString("- Dialogue-heavy style. Advance scenes through speech as much as action.\n")
	case fp.DialogueRatio < 0.05:
		sb.WriteString("- Minimal dialogue. Prefer interiority and description.\n")
	}

	// Adverb density.
	if fp.AdverbDensity > 0.02 {
		sb.WriteString("- Note: prose uses adverbs frequently — match where appropriate, but prefer strong verbs.\n")
	}

	if sb.Len() == len("## Author's prose style\n") {
		return "" // only wrote the header
	}
	return sb.String()
}

// ── text analysis helpers ─────────────────────────────────────────────────────

// splitParagraphs splits prose into non-empty paragraph blocks.
func splitParagraphs(text string) []string {
	var out []string
	for _, block := range strings.Split(text, "\n\n") {
		if trimmed := strings.TrimSpace(block); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// splitSentences splits a paragraph into individual sentences using terminal
// punctuation as a heuristic. Handles "..", "!", "?" and common abbreviations.
func splitSentences(para string) []string {
	var sentences []string
	var current strings.Builder
	runes := []rune(para)
	for i, r := range runes {
		current.WriteRune(r)
		if r == '.' || r == '!' || r == '?' {
			// Avoid splitting on common abbreviations (Mr., Dr., etc.) and
			// decimal numbers by checking if the next char is a capital letter or
			// we're at the end of the paragraph.
			next := ' '
			for j := i + 1; j < len(runes); j++ {
				if runes[j] != ' ' && runes[j] != '\n' {
					next = runes[j]
					break
				}
			}
			if unicode.IsUpper(next) || i == len(runes)-1 {
				if s := strings.TrimSpace(current.String()); s != "" {
					sentences = append(sentences, s)
				}
				current.Reset()
			}
		}
	}
	if remaining := strings.TrimSpace(current.String()); remaining != "" {
		sentences = append(sentences, remaining)
	}
	return sentences
}

func stdDev(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	mean := sum / float64(len(vals))
	var variance float64
	for _, v := range vals {
		d := v - mean
		variance += d * d
	}
	return math.Sqrt(variance / float64(len(vals)))
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func round2(f float64) float64  { return math.Round(f*100) / 100 }
func round4(f float64) float64  { return math.Round(f*10000) / 10000 }
