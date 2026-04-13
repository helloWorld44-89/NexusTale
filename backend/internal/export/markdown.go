package export

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// WriteMarkdownZip streams a project's chapters and scenes as a zip of Markdown files.
// One .md file per chapter: "01-chapter-title.md", containing all scenes in order.
// Writes directly to w — no temp file is needed.
func WriteMarkdownZip(ctx context.Context, queries *sqlcgen.Queries, projectID uuid.UUID, w io.Writer) error {
	chapters, err := queries.ListChaptersByProject(ctx, projectID)
	if err != nil {
		return fmt.Errorf("list chapters: %w", err)
	}

	zw := zip.NewWriter(w)
	defer zw.Close()

	for i, ch := range chapters {
		filename := fmt.Sprintf("%02d-%s.md", i+1, slugify(ch.Title))
		fw, err := zw.Create(filename)
		if err != nil {
			return fmt.Errorf("create zip entry %s: %w", filename, err)
		}

		fmt.Fprintf(fw, "# %s\n\n", ch.Title)

		scenes, err := queries.ListScenesByChapter(ctx, ch.ID)
		if err != nil {
			return fmt.Errorf("list scenes for chapter %s: %w", ch.ID, err)
		}

		for _, sc := range scenes {
			if sc.Title != "" {
				fmt.Fprintf(fw, "## %s\n\n", sc.Title)
			}
			if sc.Content != "" {
				fmt.Fprintf(fw, "%s\n\n", sc.Content)
			}
		}
	}

	return nil
}

// slugify converts a title to a filename-safe slug (lowercase, hyphens, alphanumeric only).
func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('-')
		}
	}
	result := b.String()
	// Collapse consecutive hyphens and trim leading/trailing ones.
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return strings.Trim(result, "-")
}
