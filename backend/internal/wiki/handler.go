package wiki

// Handler exposes Gin route handlers for the wiki subsystem.
// All routes are mounted under /projects/:id/wiki/ in main.go.
//
//	GET|POST         /entities               list (filter ?type=) or create entity
//	GET|PATCH|DELETE /entities/:eid          get, update, or delete entity
//	GET|POST         /entities/:eid/children list or create child entities (e.g. lore under a location)
//	POST             /entities/:eid/image    upload portrait image (multipart/form-data, field "image")
//	DELETE           /entities/:eid/image    remove portrait image
//	GET|POST         /relationships          list or create relationships
//	DELETE           /relationships/:rid     delete relationship
//	GET              /graph                  all entities + relationships for diagram rendering
//	GET|POST         /magic-rules            list or create magic rules
//	PATCH|DELETE     /magic-rules/:mid       update or delete magic rule
//	GET|POST         /timeline               list or create timeline events
//	PATCH|DELETE     /timeline/:tid          update or delete timeline event
//	GET              /autolink?text=         return entities whose names appear in the given text

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/jconder44/nexustale/pkg/apperror"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/entities", h.ListEntities)
	rg.POST("/entities", h.CreateEntity)
	rg.GET("/entities/:eid", h.GetEntity)
	rg.PATCH("/entities/:eid", h.UpdateEntity)
	rg.DELETE("/entities/:eid", h.DeleteEntity)
	rg.GET("/entities/:eid/children", h.ListChildEntities)
	rg.POST("/entities/:eid/children", h.CreateChildEntity)
	rg.POST("/entities/:eid/image", h.UploadEntityImage)
	rg.DELETE("/entities/:eid/image", h.DeleteEntityImage)

	rg.GET("/relationships", h.ListRelationships)
	rg.POST("/relationships", h.CreateRelationship)
	rg.DELETE("/relationships/:rid", h.DeleteRelationship)

	rg.GET("/graph", h.GetGraph)

	rg.GET("/magic-rules", h.ListMagicRules)
	rg.POST("/magic-rules", h.CreateMagicRule)
	rg.PATCH("/magic-rules/:mid", h.UpdateMagicRule)
	rg.DELETE("/magic-rules/:mid", h.DeleteMagicRule)

	rg.GET("/timeline", h.ListTimelineEvents)
	rg.POST("/timeline", h.CreateTimelineEvent)
	rg.PATCH("/timeline/:tid", h.UpdateTimelineEvent)
	rg.DELETE("/timeline/:tid", h.DeleteTimelineEvent)

	rg.GET("/autolink", h.Autolink)
}

// ========================
// Entities
// ========================

func (h *Handler) CreateEntity(c *gin.Context) {
	projectID, err := parseUUID(c, "id")
	if err != nil {
		return
	}
	var req CreateEntityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}
	resp, err := h.svc.CreateEntity(c.Request.Context(), projectID, req)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) GetEntity(c *gin.Context) {
	id, err := parseUUID(c, "eid")
	if err != nil {
		return
	}
	resp, err := h.svc.GetEntity(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListEntities(c *gin.Context) {
	projectID, err := parseUUID(c, "id")
	if err != nil {
		return
	}
	entityType := c.Query("type")
	resp, err := h.svc.ListEntities(c.Request.Context(), projectID, entityType)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) UpdateEntity(c *gin.Context) {
	id, err := parseUUID(c, "eid")
	if err != nil {
		return
	}
	var req UpdateEntityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}
	resp, err := h.svc.UpdateEntity(c.Request.Context(), id, req)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) DeleteEntity(c *gin.Context) {
	id, err := parseUUID(c, "eid")
	if err != nil {
		return
	}
	if err := h.svc.DeleteEntity(c.Request.Context(), id); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "entity deleted"})
}

func (h *Handler) ListChildEntities(c *gin.Context) {
	parentID, err := parseUUID(c, "eid")
	if err != nil {
		return
	}
	resp, err := h.svc.ListChildEntities(c.Request.Context(), parentID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// CreateChildEntity creates an entity whose parent is the entity identified by :eid.
// The parent ID comes from the URL, not the request body.
func (h *Handler) CreateChildEntity(c *gin.Context) {
	projectID, err := parseUUID(c, "id")
	if err != nil {
		return
	}
	parentID, err := parseUUID(c, "eid")
	if err != nil {
		return
	}
	var req CreateEntityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}
	req.ParentEntityID = &parentID
	resp, err := h.svc.CreateEntity(c.Request.Context(), projectID, req)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// allowedImageTypes maps the sniffed MIME type (from http.DetectContentType)
// to the content-type string used for MinIO storage. SVG is intentionally absent
// — browsers execute <script> inside SVG, enabling stored XSS.
var allowedImageTypes = map[string]string{
	"image/jpeg": "image/jpeg",
	"image/png":  "image/png",
	"image/gif":  "image/gif",
	"image/webp": "image/webp",
}

// UploadEntityImage accepts a multipart file in the "image" field,
// validates both the file extension and the actual magic bytes (via
// http.DetectContentType), and stores the result as the entity's portrait in MinIO.
func (h *Handler) UploadEntityImage(c *gin.Context) {
	id, err := parseUUID(c, "eid")
	if err != nil {
		return
	}

	fh, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "image field is required"})
		return
	}

	const maxImageSize = 5 << 20 // 5 MiB
	if fh.Size > maxImageSize {
		c.JSON(http.StatusBadRequest, gin.H{"message": "image exceeds maximum size of 5 MiB"})
		return
	}

	// Extension pre-check: reject obviously wrong or dangerous extensions before
	// opening the file. SVG is explicitly absent from this list.
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}
	if !allowedExts[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"message": "unsupported image type; allowed: jpg, png, gif, webp"})
		return
	}

	f, err := fh.Open()
	if err != nil {
		slog.Error("wiki: failed to open upload", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "could not read upload"})
		return
	}
	defer f.Close()

	// Magic-byte validation: read the first 512 bytes and let the stdlib
	// sniff the actual content type regardless of what the filename claims.
	sniffBuf := make([]byte, 512)
	n, err := io.ReadFull(f, sniffBuf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		slog.Error("wiki: failed to read upload for sniffing", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "could not read upload"})
		return
	}

	sniffed := http.DetectContentType(sniffBuf[:n])
	// DetectContentType may append "; charset=..." — strip it.
	mediaType := strings.SplitN(sniffed, ";", 2)[0]
	mediaType = strings.TrimSpace(mediaType)

	contentType, ok := allowedImageTypes[mediaType]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"message": "file content does not match an allowed image type"})
		return
	}

	// Reconstruct the full reader: already-read bytes + remainder of file.
	reader := io.MultiReader(bytes.NewReader(sniffBuf[:n]), f)

	resp, err := h.svc.UploadEntityImage(c.Request.Context(), id, fh.Filename, contentType, reader, fh.Size)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) DeleteEntityImage(c *gin.Context) {
	id, err := parseUUID(c, "eid")
	if err != nil {
		return
	}
	resp, err := h.svc.DeleteEntityImage(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ========================
// Relationships
// ========================

func (h *Handler) CreateRelationship(c *gin.Context) {
	projectID, err := parseUUID(c, "id")
	if err != nil {
		return
	}
	var req CreateRelationshipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}
	resp, err := h.svc.CreateRelationship(c.Request.Context(), projectID, req)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) ListRelationships(c *gin.Context) {
	projectID, err := parseUUID(c, "id")
	if err != nil {
		return
	}
	resp, err := h.svc.ListRelationships(c.Request.Context(), projectID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) DeleteRelationship(c *gin.Context) {
	id, err := parseUUID(c, "rid")
	if err != nil {
		return
	}
	if err := h.svc.DeleteRelationship(c.Request.Context(), id); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "relationship deleted"})
}

func (h *Handler) GetGraph(c *gin.Context) {
	projectID, err := parseUUID(c, "id")
	if err != nil {
		return
	}
	resp, err := h.svc.GetGraph(c.Request.Context(), projectID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ========================
// Magic Rules
// ========================

func (h *Handler) CreateMagicRule(c *gin.Context) {
	projectID, err := parseUUID(c, "id")
	if err != nil {
		return
	}
	var req CreateMagicRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}
	resp, err := h.svc.CreateMagicRule(c.Request.Context(), projectID, req)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) ListMagicRules(c *gin.Context) {
	projectID, err := parseUUID(c, "id")
	if err != nil {
		return
	}
	resp, err := h.svc.ListMagicRules(c.Request.Context(), projectID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) UpdateMagicRule(c *gin.Context) {
	id, err := parseUUID(c, "mid")
	if err != nil {
		return
	}
	var req UpdateMagicRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}
	resp, err := h.svc.UpdateMagicRule(c.Request.Context(), id, req)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) DeleteMagicRule(c *gin.Context) {
	id, err := parseUUID(c, "mid")
	if err != nil {
		return
	}
	if err := h.svc.DeleteMagicRule(c.Request.Context(), id); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "magic rule deleted"})
}

// ========================
// Timeline Events
// ========================

func (h *Handler) CreateTimelineEvent(c *gin.Context) {
	projectID, err := parseUUID(c, "id")
	if err != nil {
		return
	}
	var req CreateTimelineEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}
	resp, err := h.svc.CreateTimelineEvent(c.Request.Context(), projectID, req)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) ListTimelineEvents(c *gin.Context) {
	projectID, err := parseUUID(c, "id")
	if err != nil {
		return
	}
	resp, err := h.svc.ListTimelineEvents(c.Request.Context(), projectID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) UpdateTimelineEvent(c *gin.Context) {
	id, err := parseUUID(c, "tid")
	if err != nil {
		return
	}
	var req UpdateTimelineEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}
	resp, err := h.svc.UpdateTimelineEvent(c.Request.Context(), id, req)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) DeleteTimelineEvent(c *gin.Context) {
	id, err := parseUUID(c, "tid")
	if err != nil {
		return
	}
	if err := h.svc.DeleteTimelineEvent(c.Request.Context(), id); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "timeline event deleted"})
}

// ========================
// Autolink
// ========================

func (h *Handler) Autolink(c *gin.Context) {
	projectID, err := parseUUID(c, "id")
	if err != nil {
		return
	}
	text := c.Query("text")
	if text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "text query parameter is required"})
		return
	}
	resp, err := h.svc.Autolink(c.Request.Context(), projectID, text)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"entities": resp})
}

// ========================
// Helpers
// ========================

func parseUUID(c *gin.Context, param string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param(param))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid UUID", "detail": param})
		return uuid.Nil, err
	}
	return id, nil
}

func handleError(c *gin.Context, err error) {
	if appErr, ok := err.(*apperror.AppError); ok {
		c.JSON(appErr.Code, appErr)
		return
	}
	slog.Error("unhandled handler error", "path", c.FullPath(), "error", err)
	c.JSON(http.StatusInternalServerError, gin.H{"message": "internal error"})
}
