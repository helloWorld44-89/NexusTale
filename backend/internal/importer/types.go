package importer

// types.go — shared types for the manuscript import pipeline.
//
// The import flow is two-step:
//  1. POST /projects/import   — parse file → return PreviewTree (read-only)
//  2. POST /projects/import/confirm — accept (possibly edited) PreviewTree → create project

// PreviewChapter is one chapter in the import preview tree.
type PreviewChapter struct {
	Title  string         `json:"title"`
	Scenes []PreviewScene `json:"scenes"`
}

// PreviewScene is one scene within a chapter.
type PreviewScene struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// PreviewTree is the full parsed manuscript, returned by the preview endpoint.
type PreviewTree struct {
	ProjectTitle string           `json:"project_title"`
	Chapters     []PreviewChapter `json:"chapters"`
}
