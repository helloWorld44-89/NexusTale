package export

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/bmaupin/go-epub"
	"github.com/google/uuid"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// BuildEPUB assembles an EPUB 2 file from a project's chapters and scenes.
// It writes to a temp file and returns the path; the caller must os.Remove it
// after uploading to storage.
func BuildEPUB(ctx context.Context, queries *sqlcgen.Queries, projectID uuid.UUID, title string) (string, error) {
	e := epub.NewEpub(title)
	e.SetAuthor("NexusTale")

	chapters, err := queries.ListChaptersByProject(ctx, projectID)
	if err != nil {
		return "", fmt.Errorf("list chapters: %w", err)
	}

	for _, ch := range chapters {
		scenes, err := queries.ListScenesByChapter(ctx, ch.ID)
		if err != nil {
			return "", fmt.Errorf("list scenes for chapter %s: %w", ch.ID, err)
		}

		var body strings.Builder
		for _, sc := range scenes {
			if sc.Title != "" {
				body.WriteString(fmt.Sprintf("<h2>%s</h2>\n", htmlEscape(sc.Title)))
			}
			if sc.Content != "" {
				// Split on double-newline paragraph breaks; single newlines become <br/>.
				paras := strings.Split(strings.TrimSpace(sc.Content), "\n\n")
				for _, p := range paras {
					if p == "" {
						continue
					}
					inner := strings.ReplaceAll(htmlEscape(p), "\n", "<br/>")
					body.WriteString(fmt.Sprintf("<p>%s</p>\n", inner))
				}
			}
		}

		_, err = e.AddSection(body.String(), ch.Title, "", "")
		if err != nil {
			return "", fmt.Errorf("add epub section %q: %w", ch.Title, err)
		}
	}

	// go-epub writes by file path, so we create a named temp file and close it
	// before passing the path to Write.
	f, err := os.CreateTemp("", "nexustale-epub-*.epub")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := f.Name()
	f.Close()

	if err := e.Write(tmpPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("write epub: %w", err)
	}

	return tmpPath, nil
}

// htmlEscape replaces the five XML special characters so content is safe
// inside EPUB XHTML sections.
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}
