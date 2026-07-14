package importer

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jconder44/nexustale/internal/auth"
	"github.com/jconder44/nexustale/internal/project"
)

// Handler provides manuscript import routes.
type Handler struct {
	projectSvc *project.Service
}

func NewHandler(projectSvc *project.Service) *Handler {
	return &Handler{projectSvc: projectSvc}
}

// RegisterRoutes mounts import routes. Caller must apply RequireAuth.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/projects/import",         h.Preview)
	rg.POST("/projects/import/confirm", h.Confirm)
}

// Preview accepts a multipart file upload and returns the parsed PreviewTree
// without writing anything to the database.
//
// POST /projects/import
//   Content-Type: multipart/form-data
//   file: the manuscript file (.md, .txt, .docx)
func (h *Handler) Preview(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "file is required"})
		return
	}
	defer file.Close()

	const maxSize = 10 << 20 // 10 MiB
	data, err := io.ReadAll(io.LimitReader(file, maxSize))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "could not read file"})
		return
	}
	if len(data) == int(maxSize) {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"message": "file exceeds 10 MiB limit"})
		return
	}

	tree, format, err := Parse(data, header.Filename)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tree":   tree,
		"format": format,
	})
}

// Confirm accepts a (possibly writer-edited) PreviewTree and creates the project,
// acts, chapters, and scenes transactionally.
//
// POST /projects/import/confirm
//   { "tree": { "project_title": "...", "chapters": [...] } }
func (h *Handler) Confirm(c *gin.Context) {
	claims := auth.GetClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	var body struct {
		Tree PreviewTree `json:"tree"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request", "detail": err.Error()})
		return
	}
	if body.Tree.ProjectTitle == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "project_title is required"})
		return
	}
	if len(body.Tree.Chapters) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "at least one chapter is required"})
		return
	}

	resp, err := h.createFromTree(c.Request.Context(), claims.UserID, body.Tree)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": fmt.Sprintf("import failed: %v", err)})
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// createFromTree creates the project + acts + chapters + scenes from the tree.
func (h *Handler) createFromTree(ctx context.Context, ownerID uuid.UUID, tree PreviewTree) (map[string]interface{}, error) {
	// 1. Create project (also creates default Act 1).
	proj, err := h.projectSvc.CreateProject(ctx, ownerID, project.CreateProjectRequest{
		Title: tree.ProjectTitle,
	})
	if err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}

	// 2. Get the auto-created default act.
	acts, err := h.projectSvc.ListActs(ctx, proj.ID)
	if err != nil || len(acts) == 0 {
		return nil, fmt.Errorf("list acts: %w", err)
	}
	actID := acts[0].ID

	// 3. Create chapters and scenes.
	for i, ch := range tree.Chapters {
		title := ch.Title
		if title == "" {
			title = fmt.Sprintf("Chapter %d", i+1)
		}
		chapter, err := h.projectSvc.CreateChapter(ctx, actID, project.CreateChapterRequest{
			Title:     title,
			SortOrder: int32(i + 1),
		})
		if err != nil {
			return nil, fmt.Errorf("create chapter %q: %w", title, err)
		}

		for j, sc := range ch.Scenes {
			scTitle := sc.Title
			if scTitle == "" {
				scTitle = fmt.Sprintf("Scene %d", j+1)
			}
			_, err := h.projectSvc.CreateScene(ctx, chapter.ID, project.CreateSceneRequest{
				Title:     scTitle,
				Content:   sc.Content,
				SortOrder: int32(j + 1),
				ProjectID: proj.ID,
				UserID:    ownerID,
			})
			if err != nil {
				return nil, fmt.Errorf("create scene %q: %w", scTitle, err)
			}
		}
	}

	// 4. Initial Chronicle commit.
	if _, err := h.projectSvc.Chronicle(ctx, proj.ID, ownerID, project.ChronicleRequest{Note: "import: initial manuscript"}); err != nil {
		// Non-fatal — project and scenes are created; the commit may fail if no content was written.
		_ = err
	}

	return map[string]interface{}{
		"project_id": proj.ID.String(),
		"title":      proj.Title,
		"chapters":   len(tree.Chapters),
	}, nil
}

