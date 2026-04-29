package export

import (
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

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
