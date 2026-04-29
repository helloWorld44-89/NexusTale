package export

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// docScene holds the data needed to render one scene in a DOCX.
type docScene struct{ title, content string }

// docChapter holds the data needed to render one chapter in a DOCX.
type docChapter struct {
	title  string
	scenes []docScene
}

// BuildDOCX assembles a manuscript-formatted DOCX file from a project's chapters
// and scenes. It writes to a temp file and returns the path; the caller must
// os.Remove it after uploading to storage.
//
// Formatting: Times New Roman 12pt, double-spaced, 0.5-inch first-line indent,
// 1-inch margins. Page breaks between chapters. Scene headings are italic,
// centered. Scene breaks use "# # #".
func BuildDOCX(ctx context.Context, queries *sqlcgen.Queries, projectID uuid.UUID, title, repoPath string) (string, error) {
	chapters, err := queries.ListChaptersByProject(ctx, projectID)
	if err != nil {
		return "", fmt.Errorf("list chapters: %w", err)
	}

	var docs []docChapter
	for _, ch := range chapters {
		scenes, err := queries.ListScenesByChapter(ctx, ch.ID)
		if err != nil {
			return "", fmt.Errorf("list scenes for chapter %s: %w", ch.ID, err)
		}
		var ss []docScene
		for _, sc := range scenes {
			ss = append(ss, docScene{title: sc.Title, content: sceneFileContent(repoPath, ch.ID, sc.ID, "")})
		}
		docs = append(docs, docChapter{title: ch.Title, scenes: ss})
	}

	f, err := os.CreateTemp("", "nexustale-docx-*.docx")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := f.Name()

	zw := zip.NewWriter(f)
	if err := writeDocxEntries(zw, title, docs); err != nil {
		zw.Close()
		f.Close()
		os.Remove(tmpPath)
		return "", err
	}
	if err := zw.Close(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("close docx zip: %w", err)
	}
	f.Close()
	return tmpPath, nil
}

// writeDocxEntries writes all OOXML parts into the zip writer.
func writeDocxEntries(zw *zip.Writer, title string, chapters []docChapter) error {
	entries := []struct {
		name    string
		content string
	}{
		{"[Content_Types].xml", docxContentTypes},
		{"_rels/.rels", docxRootRels},
		{"word/_rels/document.xml.rels", docxDocumentRels},
		{"word/styles.xml", docxStyles},
		{"word/document.xml", buildDocumentXML(title, chapters)},
	}

	for _, e := range entries {
		fw, err := zw.Create(e.name)
		if err != nil {
			return fmt.Errorf("create zip entry %s: %w", e.name, err)
		}
		if _, err := fmt.Fprint(fw, e.content); err != nil {
			return fmt.Errorf("write zip entry %s: %w", e.name, err)
		}
	}
	return nil
}

// buildDocumentXML generates the word/document.xml body from project data.
func buildDocumentXML(title string, chapters []docChapter) string {
	var b strings.Builder

	// Title paragraph — centered, bold, 18pt.
	b.WriteString(paStyledRun("DocTitle", xmlEscape(title)))
	// Blank line after title.
	b.WriteString(paEmpty("Normal"))

	for i, ch := range chapters {
		// Page break before each chapter except the first.
		if i > 0 {
			b.WriteString(paPageBreak())
		}
		b.WriteString(paStyledRun("Heading1", xmlEscape(ch.title)))

		for si, sc := range ch.scenes {
			if sc.title != "" {
				b.WriteString(paStyledRun("Heading2", xmlEscape(sc.title)))
			}

			// Split double-newlines into paragraphs; single newlines become spaces.
			paras := strings.Split(strings.TrimSpace(sc.content), "\n\n")
			for _, p := range paras {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				// Collapse intra-paragraph newlines to a single space.
				p = strings.ReplaceAll(p, "\n", " ")
				b.WriteString(paBodyText(xmlEscape(p)))
			}

			// Scene break between scenes within the same chapter.
			if si < len(ch.scenes)-1 {
				b.WriteString(paStyledRun("SceneBreak", "# # #"))
			}
		}
	}

	// sectPr must be the last child of w:body.
	b.WriteString(docxSectPr)

	return fmt.Sprintf(docxDocumentWrapper, b.String())
}

// ── paragraph builders ────────────────────────────────────────────────────────

// paStyledRun returns a paragraph using a named style with a single text run.
func paStyledRun(style, text string) string {
	return fmt.Sprintf(
		`<w:p><w:pPr><w:pStyle w:val="%s"/></w:pPr><w:r><w:t xml:space="preserve">%s</w:t></w:r></w:p>`,
		style, text,
	)
}

// paBodyText returns a Normal-styled paragraph (double-spaced, indented).
func paBodyText(text string) string {
	return fmt.Sprintf(
		`<w:p><w:pPr><w:pStyle w:val="Normal"/></w:pPr><w:r><w:t xml:space="preserve">%s</w:t></w:r></w:p>`,
		text,
	)
}

// paEmpty returns an empty paragraph in the given style.
func paEmpty(style string) string {
	return fmt.Sprintf(`<w:p><w:pPr><w:pStyle w:val="%s"/></w:pPr></w:p>`, style)
}

// paPageBreak returns a paragraph that forces a page break.
func paPageBreak() string {
	return `<w:p><w:r><w:br w:type="page"/></w:r></w:p>`
}

// xmlEscape escapes the five XML special characters for safe embedding in OOXML.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// ── OOXML static parts ────────────────────────────────────────────────────────

const docxContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml"  ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml"   ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`

const docxRootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1"
    Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
    Target="word/document.xml"/>
</Relationships>`

const docxDocumentRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1"
    Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles"
    Target="styles.xml"/>
</Relationships>`

// docxStyles defines the paragraph and character styles used in the document.
// All body text: Times New Roman 12pt, double-spaced, 0.5-inch first-line indent.
// Heading1: chapter title — bold 14pt, centered, no first-line indent.
// Heading2: scene title — italic 12pt, centered, no first-line indent.
// DocTitle: cover title — bold 18pt, centered.
// SceneBreak: centered, single-spaced, no indent.
const docxStyles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">

  <w:docDefaults>
    <w:rPrDefault>
      <w:rPr>
        <w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman" w:cs="Times New Roman"/>
        <w:sz w:val="24"/>
        <w:szCs w:val="24"/>
      </w:rPr>
    </w:rPrDefault>
  </w:docDefaults>

  <!-- Normal — manuscript body text -->
  <w:style w:type="paragraph" w:styleId="Normal" w:default="1">
    <w:name w:val="Normal"/>
    <w:pPr>
      <w:spacing w:line="480" w:lineRule="auto" w:before="0" w:after="0"/>
      <w:ind w:firstLine="720"/>
      <w:jc w:val="both"/>
    </w:pPr>
    <w:rPr>
      <w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman" w:cs="Times New Roman"/>
      <w:sz w:val="24"/>
      <w:szCs w:val="24"/>
    </w:rPr>
  </w:style>

  <!-- DocTitle — cover / title page heading -->
  <w:style w:type="paragraph" w:styleId="DocTitle">
    <w:name w:val="Document Title"/>
    <w:pPr>
      <w:jc w:val="center"/>
      <w:spacing w:line="480" w:lineRule="auto" w:before="2880" w:after="480"/>
      <w:ind w:firstLine="0"/>
    </w:pPr>
    <w:rPr>
      <w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman" w:cs="Times New Roman"/>
      <w:b/><w:bCs/>
      <w:sz w:val="36"/>
      <w:szCs w:val="36"/>
    </w:rPr>
  </w:style>

  <!-- Heading1 — chapter title -->
  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="heading 1"/>
    <w:pPr>
      <w:jc w:val="center"/>
      <w:spacing w:line="480" w:lineRule="auto" w:before="0" w:after="480"/>
      <w:ind w:firstLine="0"/>
    </w:pPr>
    <w:rPr>
      <w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman" w:cs="Times New Roman"/>
      <w:b/><w:bCs/>
      <w:sz w:val="28"/>
      <w:szCs w:val="28"/>
    </w:rPr>
  </w:style>

  <!-- Heading2 — scene title -->
  <w:style w:type="paragraph" w:styleId="Heading2">
    <w:name w:val="heading 2"/>
    <w:pPr>
      <w:jc w:val="center"/>
      <w:spacing w:line="480" w:lineRule="auto" w:before="480" w:after="240"/>
      <w:ind w:firstLine="0"/>
    </w:pPr>
    <w:rPr>
      <w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman" w:cs="Times New Roman"/>
      <w:i/><w:iCs/>
      <w:sz w:val="24"/>
      <w:szCs w:val="24"/>
    </w:rPr>
  </w:style>

  <!-- SceneBreak — the "# # #" separator between scenes -->
  <w:style w:type="paragraph" w:styleId="SceneBreak">
    <w:name w:val="Scene Break"/>
    <w:pPr>
      <w:jc w:val="center"/>
      <w:spacing w:line="480" w:lineRule="auto" w:before="240" w:after="240"/>
      <w:ind w:firstLine="0"/>
    </w:pPr>
    <w:rPr>
      <w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman" w:cs="Times New Roman"/>
      <w:sz w:val="24"/>
      <w:szCs w:val="24"/>
    </w:rPr>
  </w:style>

</w:styles>`

// docxSectPr defines page size (US Letter) and 1-inch margins.
// Must be the last element inside w:body.
const docxSectPr = `<w:sectPr>
  <w:pgSz w:w="12240" w:h="15840"/>
  <w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720"/>
</w:sectPr>`

// docxDocumentWrapper wraps the body paragraphs in the full document XML shell.
const docxDocumentWrapper = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:wpc="http://schemas.microsoft.com/office/word/2010/wordprocessingCanvas"
            xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
            xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>%s</w:body>
</w:document>`
