package ai

// embeddings.go — C9-P7: Semantic RAG via pgvector.
//
// EmbedStore wraps a pgxpool.Pool and an Embedder to provide:
//   - UpsertChapterEmbedding / UpsertEntityEmbedding / UpsertNoteEmbedding
//   - SearchChapterSummaries / SearchEntities / SearchNotes
//   - BackgroundReembed: periodic worker that catches any un-embedded rows
//
// The store bypasses sqlc for all vector operations because the pgvector `<=>`
// operator and `vector` literal cast are not supported by sqlc's type system.
// All other DB access continues through sqlcgen.Queries.
//
// Fallback: when the embedding column IS NULL (extension not loaded, or no
// embeddings generated yet), BuildContext falls back to the brute-force
// injection path. The two paths are completely independent.

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jconder44/nexustale/pkg/embedding"
)

// EmbedStore manages the vector embedding lifecycle for a project.
type EmbedStore struct {
	pool    *pgxpool.Pool
	embeddr embedding.Embedder // nil when no embedding provider is configured
}

func NewEmbedStore(pool *pgxpool.Pool, e embedding.Embedder) *EmbedStore {
	return &EmbedStore{pool: pool, embeddr: e}
}

// available returns true when an embedder is configured and the pool is non-nil.
func (es *EmbedStore) available() bool {
	return es != nil && es.pool != nil && es.embeddr != nil
}

// ── Upsert ────────────────────────────────────────────────────────────────────

// UpsertChapterEmbedding generates and stores an embedding for a chapter summary.
func (es *EmbedStore) UpsertChapterEmbedding(ctx context.Context, chapterID uuid.UUID, branchName, summary string) {
	if !es.available() || summary == "" {
		return
	}
	vec, err := es.embeddr.Embed(ctx, summary)
	if err != nil {
		slog.Warn("ai: embed chapter summary failed", "chapter_id", chapterID, "error", err)
		return
	}
	_, err = es.pool.Exec(ctx,
		`UPDATE chapter_summaries
		 SET embedding = $1::vector, embedding_updated_at = now()
		 WHERE chapter_id = $2 AND branch_name = $3`,
		embedding.VecToString(vec), chapterID, branchName,
	)
	if err != nil {
		slog.Warn("ai: upsert chapter embedding failed", "chapter_id", chapterID, "error", err)
	}
}

// UpsertEntityEmbedding generates and stores an embedding for a wiki entity.
// The embedded text is "name: summary" for richer semantic matching.
func (es *EmbedStore) UpsertEntityEmbedding(ctx context.Context, entityID uuid.UUID, name, summary string) {
	if !es.available() {
		return
	}
	text := name
	if summary != "" {
		text = name + ": " + summary
	}
	vec, err := es.embeddr.Embed(ctx, text)
	if err != nil {
		slog.Warn("ai: embed entity failed", "entity_id", entityID, "error", err)
		return
	}
	_, err = es.pool.Exec(ctx,
		`UPDATE wiki_entities
		 SET embedding = $1::vector, embedding_updated_at = now()
		 WHERE id = $2`,
		embedding.VecToString(vec), entityID,
	)
	if err != nil {
		slog.Warn("ai: upsert entity embedding failed", "entity_id", entityID, "error", err)
	}
}

// UpsertNoteEmbedding generates and stores an embedding for a research note.
func (es *EmbedStore) UpsertNoteEmbedding(ctx context.Context, noteID uuid.UUID, title, body string) {
	if !es.available() {
		return
	}
	text := title
	if body != "" {
		text = title + "\n\n" + body
	}
	vec, err := es.embeddr.Embed(ctx, text)
	if err != nil {
		slog.Warn("ai: embed research note failed", "note_id", noteID, "error", err)
		return
	}
	_, err = es.pool.Exec(ctx,
		`UPDATE research_notes
		 SET embedding = $1::vector, embedding_updated_at = now()
		 WHERE id = $2`,
		embedding.VecToString(vec), noteID,
	)
	if err != nil {
		slog.Warn("ai: upsert note embedding failed", "note_id", noteID, "error", err)
	}
}

// ── Search ────────────────────────────────────────────────────────────────────

// SummaryResult is a single chapter summary returned by semantic search.
type SummaryResult struct {
	ChapterID  uuid.UUID
	BranchName string
	Title      string
	Summary    string
}

// SearchChapterSummaries returns the top-K chapter summaries most semantically
// similar to queryText, scoped to a project and branch.
// Returns nil (not an error) when no embeddings exist — caller falls back to
// brute-force injection.
func (es *EmbedStore) SearchChapterSummaries(ctx context.Context, projectID uuid.UUID, branchName, queryText string, limit int) ([]SummaryResult, error) {
	if !es.available() || queryText == "" {
		return nil, nil
	}

	vec, err := es.embeddr.Embed(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	rows, err := es.pool.Query(ctx, `
		SELECT cs.chapter_id, cs.branch_name, c.title, cs.ai_summary
		FROM chapter_summaries cs
		JOIN chapters c ON c.id = cs.chapter_id
		JOIN acts a ON a.id = c.act_id
		WHERE a.project_id = $2
		  AND cs.branch_name = $3
		  AND cs.embedding IS NOT NULL
		  AND cs.ai_summary != ''
		ORDER BY cs.embedding <=> $1::vector
		LIMIT $4`,
		embedding.VecToString(vec), projectID, branchName, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search chapter summaries: %w", err)
	}
	defer rows.Close()

	var results []SummaryResult
	for rows.Next() {
		var r SummaryResult
		if err := rows.Scan(&r.ChapterID, &r.BranchName, &r.Title, &r.Summary); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// EntityResult is a single wiki entity returned by semantic search.
type EntityResult struct {
	ID      uuid.UUID
	Name    string
	Type    string
	Summary string
}

// SearchEntities returns the top-K entities most semantically similar to
// queryText, scoped to a project. Returns nil when no embeddings exist.
func (es *EmbedStore) SearchEntities(ctx context.Context, projectID uuid.UUID, queryText string, limit int) ([]EntityResult, error) {
	if !es.available() || queryText == "" {
		return nil, nil
	}

	vec, err := es.embeddr.Embed(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	rows, err := es.pool.Query(ctx, `
		SELECT id, name, type, COALESCE(summary, '')
		FROM wiki_entities
		WHERE project_id = $2
		  AND embedding IS NOT NULL
		ORDER BY embedding <=> $1::vector
		LIMIT $3`,
		embedding.VecToString(vec), projectID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search entities: %w", err)
	}
	defer rows.Close()

	var results []EntityResult
	for rows.Next() {
		var r EntityResult
		if err := rows.Scan(&r.ID, &r.Name, &r.Type, &r.Summary); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// HasEmbeddings returns true when the project has at least one embedded chapter
// summary. Used by BuildContext to decide between semantic and brute-force paths.
func (es *EmbedStore) HasEmbeddings(ctx context.Context, projectID uuid.UUID) bool {
	if !es.available() {
		return false
	}
	var count int
	err := es.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM chapter_summaries cs
		JOIN chapters c ON c.id = cs.chapter_id
		JOIN acts a ON a.id = c.act_id
		WHERE a.project_id = $1
		  AND cs.embedding IS NOT NULL
		LIMIT 1`,
		projectID,
	).Scan(&count)
	return err == nil && count > 0
}

// ── Background re-embedding worker ───────────────────────────────────────────

// BackgroundReembed periodically finds un-embedded entities and research notes
// and generates their embeddings. Runs until ctx is cancelled.
// interval is typically 10–15 minutes — embeddings are not time-critical.
func (es *EmbedStore) BackgroundReembed(ctx context.Context, interval time.Duration) {
	if !es.available() {
		return
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			es.reembedStaleEntities(ctx)
			es.reembedStaleNotes(ctx)
		}
	}
}

// reembedStaleEntities generates embeddings for up to 50 entities whose
// embedding is NULL or older than their updated_at timestamp.
func (es *EmbedStore) reembedStaleEntities(ctx context.Context) {
	rows, err := es.pool.Query(ctx, `
		SELECT id, name, COALESCE(summary, '')
		FROM wiki_entities
		WHERE embedding IS NULL
		   OR embedding_updated_at < updated_at
		LIMIT 50`,
	)
	if err != nil {
		slog.Warn("ai: reembed entities — query failed", "error", err)
		return
	}
	defer rows.Close()

	type row struct {
		id      uuid.UUID
		name    string
		summary string
	}
	var batch []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.name, &r.summary); err == nil {
			batch = append(batch, r)
		}
	}
	rows.Close()

	for _, r := range batch {
		es.UpsertEntityEmbedding(ctx, r.id, r.name, r.summary)
	}
	if len(batch) > 0 {
		slog.Debug("ai: reembed entities", "count", len(batch))
	}
}

// reembedStaleNotes generates embeddings for up to 50 research notes whose
// embedding is NULL or stale.
func (es *EmbedStore) reembedStaleNotes(ctx context.Context) {
	rows, err := es.pool.Query(ctx, `
		SELECT id, title, COALESCE(body, '')
		FROM research_notes
		WHERE embedding IS NULL
		   OR embedding_updated_at < updated_at
		LIMIT 50`,
	)
	if err != nil {
		slog.Warn("ai: reembed notes — query failed", "error", err)
		return
	}
	defer rows.Close()

	type row struct {
		id    uuid.UUID
		title string
		body  string
	}
	var batch []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.title, &r.body); err == nil {
			batch = append(batch, r)
		}
	}
	rows.Close()

	for _, r := range batch {
		es.UpsertNoteEmbedding(ctx, r.id, r.title, r.body)
	}
	if len(batch) > 0 {
		slog.Debug("ai: reembed notes", "count", len(batch))
	}
}

// buildSemanticStorySoFar builds the "## Story so far" section using semantic
// search to retrieve the most relevant chapter summaries for the current context.
// Returns "" when semantic search is unavailable, triggering the brute-force fallback.
// Always includes the first chapter (story anchor) and the current chapter.
func (es *EmbedStore) buildSemanticStorySoFar(
	ctx context.Context,
	projectID uuid.UUID,
	branchName string,
	queryText string,
	currentChapterID uuid.UUID,
	firstChapterSummary, currentChapterSummary string,
	firstChapterTitle, currentChapterTitle string,
) string {
	const semanticLimit = 5

	results, err := es.SearchChapterSummaries(ctx, projectID, branchName, queryText, semanticLimit+2)
	if err != nil || len(results) == 0 {
		return "" // trigger brute-force fallback
	}

	// Deduplicate: skip the first and current chapters (injected separately).
	var semantic []SummaryResult
	for _, r := range results {
		if r.ChapterID == currentChapterID {
			continue
		}
		// Skip if this is the first chapter (we'll inject it as anchor below).
		if firstChapterTitle != "" && r.Title == firstChapterTitle {
			continue
		}
		semantic = append(semantic, r)
		if len(semantic) >= semanticLimit {
			break
		}
	}

	var sb strings.Builder
	sb.WriteString("## Story so far\n")

	// Story anchor: always inject the first chapter.
	if firstChapterSummary != "" {
		sb.WriteString(fmt.Sprintf("**%s**: %s\n", firstChapterTitle, firstChapterSummary))
	}

	// Semantically relevant chapters from the middle of the story.
	for _, r := range semantic {
		sb.WriteString(fmt.Sprintf("**%s**: %s\n", r.Title, r.Summary))
	}

	// Current chapter summary (most immediate story context).
	if currentChapterSummary != "" && currentChapterTitle != "" {
		sb.WriteString(fmt.Sprintf("**%s** *(current chapter)*: %s\n", currentChapterTitle, currentChapterSummary))
	}

	return sb.String()
}
