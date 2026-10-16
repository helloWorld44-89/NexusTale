package export

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/google/uuid"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// orderedChapters returns a project's chapters in manuscript order: by act
// sort_order, then chapter sort_order within that act.
//
// ListChaptersByProject alone is insufficient — chapter.sort_order is scoped
// per-act (the editor numbers chapters 1, 2, 3... within each act, resetting
// at each new act; see Editor.tsx's handleCreateChapter), not globally
// unique across the project. Ordering by sort_order ASC alone interleaves
// acts unpredictably for any project with more than one act, which every
// export format (Markdown, EPUB, DOCX) inherited since they all call
// ListChaptersByProject directly.
func orderedChapters(ctx context.Context, queries *sqlcgen.Queries, projectID uuid.UUID) ([]sqlcgen.Chapter, error) {
	acts, err := queries.ListActsByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list acts: %w", err)
	}
	actOrder := make(map[uuid.UUID]int32, len(acts))
	for _, a := range acts {
		actOrder[a.ID] = a.SortOrder
	}

	chapters, err := queries.ListChaptersByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list chapters: %w", err)
	}

	sort.SliceStable(chapters, func(i, j int) bool {
		oi, oj := actOrder[chapters[i].ActID], actOrder[chapters[j].ActID]
		if oi != oj {
			return oi < oj
		}
		return chapters[i].SortOrder < chapters[j].SortOrder
	})

	return chapters, nil
}

// sceneFileContent returns the content of a scene from the git working tree if
// the file exists, or fallback (the Postgres-sourced value) otherwise.
// Callers pass repoPath="" when no git repo is available; fallback is returned.
func sceneFileContent(repoPath string, chapterID, sceneID uuid.UUID, fallback string) string {
	if repoPath == "" {
		return fallback
	}
	path := filepath.Join(repoPath, "chapters", chapterID.String(), "scenes", sceneID.String()+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	return string(data)
}
