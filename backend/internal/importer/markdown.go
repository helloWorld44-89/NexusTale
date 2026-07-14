package importer

import (
	"strings"
)

// ParseMarkdown parses a Markdown document into a PreviewTree.
//
// Structure inference (heading-first):
//   # Chapter Title   → new chapter
//   ## Scene Title    → new scene within current chapter
//   # # # or ***      → scene break (untitled scene continues in new scene)
//   (no headings at all) → single chapter, separator-based scene splitting
//
// Plain paragraphs become scene content.
func ParseMarkdown(src, filename string) PreviewTree {
	lines := strings.Split(src, "\n")
	title := titleFromFilename(filename)

	var chapters []PreviewChapter
	var curChapter *PreviewChapter
	var curScene  *PreviewScene
	var body      strings.Builder

	flushScene := func() {
		if curChapter == nil {
			return
		}
		content := strings.TrimSpace(body.String())
		body.Reset()
		if curScene != nil || content != "" {
			t := ""
			if curScene != nil {
				t = curScene.Title
			}
			curChapter.Scenes = append(curChapter.Scenes, PreviewScene{Title: t, Content: content})
			curScene = nil
		}
	}

	flushChapter := func() {
		flushScene()
		if curChapter != nil && len(curChapter.Scenes) > 0 {
			chapters = append(chapters, *curChapter)
		}
		curChapter = nil
	}

	ensureChapter := func(title string) {
		if curChapter == nil {
			c := PreviewChapter{Title: title}
			curChapter = &c
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ") {
			// H1 → new chapter
			flushChapter()
			t := strings.TrimPrefix(trimmed, "# ")
			c := PreviewChapter{Title: strings.TrimSpace(t)}
			curChapter = &c
			continue
		}

		if strings.HasPrefix(trimmed, "## ") {
			// H2 → new scene
			flushScene()
			ensureChapter("Chapter 1")
			t := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			curScene = &PreviewScene{Title: t}
			continue
		}

		if isSceneSeparator(trimmed) {
			// scene break
			flushScene()
			ensureChapter("Chapter 1")
			continue
		}

		ensureChapter("Chapter 1")
		if body.Len() > 0 {
			body.WriteByte('\n')
		}
		body.WriteString(line)
	}

	flushChapter()

	if len(chapters) == 0 {
		chapters = []PreviewChapter{{Title: "Chapter 1", Scenes: []PreviewScene{{Title: "", Content: strings.TrimSpace(src)}}}}
	}

	return PreviewTree{ProjectTitle: title, Chapters: chapters}
}

func isSceneSeparator(line string) bool {
	stripped := strings.ReplaceAll(strings.ReplaceAll(line, " ", ""), "\t", "")
	return stripped == "###" || stripped == "***" || stripped == "---" || stripped == "___"
}
