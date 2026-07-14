package importer

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// ParseDocx parses a DOCX file into a PreviewTree.
//
// Structure inference from Word styles:
//   Heading1 (w:styleId="Heading1")  → new chapter
//   Heading2 (w:styleId="Heading2")  → new scene
//   Normal paragraphs                → prose content
//   # # # or *** in text             → scene break
//
// DOCX is a ZIP containing word/document.xml which holds all paragraphs.
func ParseDocx(data []byte, filename string) (PreviewTree, error) {
	title := titleFromFilename(filename)

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return PreviewTree{}, fmt.Errorf("open docx zip: %w", err)
	}

	// Find word/document.xml inside the zip.
	var docFile *zip.File
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return PreviewTree{}, fmt.Errorf("word/document.xml not found in docx")
	}

	rc, err := docFile.Open()
	if err != nil {
		return PreviewTree{}, fmt.Errorf("open document.xml: %w", err)
	}
	defer rc.Close()
	xmlBytes, err := io.ReadAll(rc)
	if err != nil {
		return PreviewTree{}, fmt.Errorf("read document.xml: %w", err)
	}

	paragraphs := extractDocxParagraphs(xmlBytes)

	var chapters []PreviewChapter
	var curChapter *PreviewChapter
	var curScene  *PreviewScene
	var body      strings.Builder

	chapterNum := 1
	sceneNum   := 1

	flushScene := func() {
		if curChapter == nil {
			return
		}
		content := strings.TrimSpace(body.String())
		body.Reset()
		if content != "" {
			t := ""
			if curScene != nil {
				t = curScene.Title
			}
			curChapter.Scenes = append(curChapter.Scenes, PreviewScene{Title: t, Content: content})
			sceneNum++
			curScene = nil
		}
	}

	flushChapter := func() {
		flushScene()
		if curChapter != nil && len(curChapter.Scenes) > 0 {
			chapters = append(chapters, *curChapter)
		}
		curChapter = nil
		chapterNum++
		sceneNum = 1
	}

	ensureChapter := func() {
		if curChapter == nil {
			c := PreviewChapter{Title: fmt.Sprintf("Chapter %d", chapterNum)}
			curChapter = &c
		}
	}

	for _, p := range paragraphs {
		trimmed := strings.TrimSpace(p.text)

		switch p.style {
		case "heading1", "Heading1", "1":
			flushChapter()
			t := trimmed
			if t == "" {
				t = fmt.Sprintf("Chapter %d", chapterNum)
			}
			c := PreviewChapter{Title: t}
			curChapter = &c
			continue
		case "heading2", "Heading2", "2":
			flushScene()
			ensureChapter()
			t := trimmed
			if t == "" {
				t = fmt.Sprintf("Scene %d", sceneNum)
			}
			curScene = &PreviewScene{Title: t}
			continue
		}

		if isSceneSeparator(trimmed) {
			flushScene()
			ensureChapter()
			continue
		}

		ensureChapter()
		if body.Len() > 0 {
			body.WriteByte('\n')
		}
		body.WriteString(p.text)
	}

	flushChapter()

	if len(chapters) == 0 {
		// No headings found — treat whole document as one chapter.
		var all strings.Builder
		for i, p := range paragraphs {
			if i > 0 {
				all.WriteByte('\n')
			}
			all.WriteString(p.text)
		}
		chapters = []PreviewChapter{{Title: "Chapter 1", Scenes: []PreviewScene{{Content: all.String()}}}}
	}

	return PreviewTree{ProjectTitle: title, Chapters: chapters}, nil
}

// ── DOCX XML extraction ───────────────────────────────────────────────────────

type docxPara struct {
	style string
	text  string
}

// extractDocxParagraphs walks word/document.xml and returns paragraphs with
// their style and concatenated text content.
func extractDocxParagraphs(data []byte) []docxPara {
	type wPr struct {
		PStyle struct {
			Val string `xml:"val,attr"`
		} `xml:"pStyle"`
	}
	type wP struct {
		PPr wPr    `xml:"pPr"`
		Rs  []struct {
			T string `xml:"t"`
		} `xml:"r"`
	}
	type wBody struct {
		Ps []wP `xml:"p"`
	}
	type doc struct {
		Body wBody `xml:"body"`
	}

	var d doc
	if err := xml.Unmarshal(data, &d); err != nil {
		// Fallback: extract plain text by stripping XML tags.
		return []docxPara{{text: stripXML(string(data))}}
	}

	var result []docxPara
	for _, p := range d.Body.Ps {
		var sb strings.Builder
		for _, r := range p.Rs {
			sb.WriteString(r.T)
		}
		result = append(result, docxPara{
			style: p.PPr.PStyle.Val,
			text:  sb.String(),
		})
	}
	return result
}

func stripXML(s string) string {
	var out strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			out.WriteRune(r)
		}
	}
	return strings.TrimSpace(out.String())
}
