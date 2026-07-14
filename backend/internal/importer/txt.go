package importer

import (
	"fmt"
	"strings"
)

// ParseTxt parses a plain-text document into a PreviewTree.
//
// Structure inference:
//   # # # or *** or ---    → scene break
//   two or more blank lines → chapter break
//   everything else         → prose content
//
// No headings in plain text, so chapter/scene titles are generated.
func ParseTxt(src, filename string) PreviewTree {
	title := titleFromFilename(filename)
	lines := strings.Split(src, "\n")

	var chapters []PreviewChapter
	var curScenes []PreviewScene
	var curContent strings.Builder
	var blankRun int

	chapterNum := 1
	sceneNum   := 1

	flushScene := func() {
		content := strings.TrimSpace(curContent.String())
		curContent.Reset()
		if content != "" {
			curScenes = append(curScenes, PreviewScene{
				Title:   sceneName(sceneNum),
				Content: content,
			})
			sceneNum++
		}
		blankRun = 0
	}

	flushChapter := func() {
		flushScene()
		if len(curScenes) > 0 {
			chapters = append(chapters, PreviewChapter{
				Title:  chapterName(chapterNum),
				Scenes: curScenes,
			})
			chapterNum++
			curScenes = nil
			sceneNum = 1
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if isSceneSeparator(trimmed) {
			flushScene()
			blankRun = 0
			continue
		}

		if trimmed == "" {
			blankRun++
			if blankRun >= 2 {
				flushChapter()
				blankRun = 0
			} else {
				if curContent.Len() > 0 {
					curContent.WriteByte('\n')
				}
			}
			continue
		}

		blankRun = 0
		if curContent.Len() > 0 {
			curContent.WriteByte('\n')
		}
		curContent.WriteString(line)
	}

	flushChapter()

	if len(chapters) == 0 {
		chapters = []PreviewChapter{{Title: "Chapter 1", Scenes: []PreviewScene{{Title: "", Content: strings.TrimSpace(src)}}}}
	}

	return PreviewTree{ProjectTitle: title, Chapters: chapters}
}

func chapterName(n int) string {
	names := []string{"", "One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine", "Ten"}
	if n < len(names) {
		return "Chapter " + names[n]
	}
	return fmt.Sprintf("Chapter %d", n)
}

func sceneName(n int) string {
	if n == 1 {
		return ""
	}
	return fmt.Sprintf("Scene %d", n)
}
