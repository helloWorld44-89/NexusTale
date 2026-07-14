package importer

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Parse dispatches to the format-specific parser based on file extension.
// Returns a PreviewTree and the detected format name.
func Parse(data []byte, filename string) (PreviewTree, string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".md", ".markdown":
		return ParseMarkdown(string(data), filename), "markdown", nil
	case ".txt":
		return ParseTxt(string(data), filename), "txt", nil
	case ".docx":
		tree, err := ParseDocx(data, filename)
		return tree, "docx", err
	default:
		return PreviewTree{}, "", fmt.Errorf("unsupported format %q; accepted: .md, .txt, .docx", ext)
	}
}

// titleFromFilename derives a project title from the uploaded filename by
// stripping the extension and replacing underscores/hyphens with spaces.
func titleFromFilename(filename string) string {
	base := filepath.Base(filename)
	ext  := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	name  = strings.ReplaceAll(name, "_", " ")
	name  = strings.ReplaceAll(name, "-", " ")
	// Capitalise the first letter.
	if len(name) > 0 {
		name = strings.ToUpper(name[:1]) + name[1:]
	}
	return name
}
