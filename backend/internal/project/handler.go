package project

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/jconder44/nexustale/internal/auth"
	"github.com/jconder44/nexustale/pkg/apperror"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, chapterGroup *gin.RouterGroup) {
	// Projects
	rg.GET("", h.ListProjects)
	rg.POST("", h.CreateProject)
	rg.GET("/:id", h.GetProject)
	rg.PATCH("/:id", h.UpdateProject)
	rg.DELETE("/:id", h.DeleteProject)

	// Acts — nested under projects
	rg.POST("/:id/acts", h.CreateAct)
	rg.GET("/:id/acts", h.ListActs)
	rg.GET("/:id/acts/:aid", h.GetAct)
	rg.PATCH("/:id/acts/:aid", h.UpdateAct)
	rg.DELETE("/:id/acts/:aid", h.DeleteAct)

	// Chapters — nested under acts
	rg.POST("/:id/acts/:aid/chapters", h.CreateChapter)
	rg.GET("/:id/acts/:aid/chapters", h.ListChapters)
	rg.GET("/:id/acts/:aid/chapters/:cid", h.GetChapter)
	rg.PATCH("/:id/acts/:aid/chapters/:cid", h.UpdateChapter)
	rg.DELETE("/:id/acts/:aid/chapters/:cid", h.DeleteChapter)

	// Scenes — scoped to chapter only (avoids deeply nested project/act/chapter/scene paths)
	chapterGroup.POST("/:cid/scenes", h.CreateScene)
	chapterGroup.GET("/:cid/scenes", h.ListScenes)
	chapterGroup.GET("/:cid/scenes/:sid", h.GetScene)
	chapterGroup.PATCH("/:cid/scenes/:sid", h.UpdateScene)
	chapterGroup.DELETE("/:cid/scenes/:sid", h.DeleteScene)

	// Git / Chronicle routes — all scoped to a specific project.
	rg.GET("/:id/git/status", h.GitStatus)
	rg.POST("/:id/git/chronicle", h.Chronicle)
	rg.GET("/:id/git/lore", h.Lore)
	rg.GET("/:id/git/echo", h.Echo)
	rg.GET("/:id/git/timelines", h.ListTimelines)
	rg.POST("/:id/git/timelines", h.Diverge)
	rg.POST("/:id/git/timelines/:tname/canonize", h.Canonize)
	rg.POST("/:id/git/timelines/:tname/travel", h.TravelTo)
}

// Projects

func (h *Handler) CreateProject(c *gin.Context) {
	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}

	userID := auth.GetUserID(c)
	resp, err := h.svc.CreateProject(c.Request.Context(), userID, req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) GetProject(c *gin.Context) {
	id, err := parseUUID(c, "id")
	if err != nil {
		return
	}

	resp, err := h.svc.GetProject(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListProjects(c *gin.Context) {
	userID := auth.GetUserID(c)
	resp, err := h.svc.ListProjects(c.Request.Context(), userID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) UpdateProject(c *gin.Context) {
	id, err := parseUUID(c, "id")
	if err != nil {
		return
	}

	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}

	resp, err := h.svc.UpdateProject(c.Request.Context(), id, req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) DeleteProject(c *gin.Context) {
	id, err := parseUUID(c, "id")
	if err != nil {
		return
	}

	if err := h.svc.DeleteProject(c.Request.Context(), id); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "project deleted"})
}

// Acts

func (h *Handler) CreateAct(c *gin.Context) {
	projectID, err := parseUUID(c, "id")
	if err != nil {
		return
	}

	var req CreateActRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}

	resp, err := h.svc.CreateAct(c.Request.Context(), projectID, req)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) GetAct(c *gin.Context) {
	actID, err := parseUUID(c, "aid")
	if err != nil {
		return
	}

	resp, err := h.svc.GetAct(c.Request.Context(), actID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListActs(c *gin.Context) {
	projectID, err := parseUUID(c, "id")
	if err != nil {
		return
	}

	resp, err := h.svc.ListActs(c.Request.Context(), projectID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) UpdateAct(c *gin.Context) {
	actID, err := parseUUID(c, "aid")
	if err != nil {
		return
	}

	var req UpdateActRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}

	resp, err := h.svc.UpdateAct(c.Request.Context(), actID, req)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) DeleteAct(c *gin.Context) {
	actID, err := parseUUID(c, "aid")
	if err != nil {
		return
	}

	if err := h.svc.DeleteAct(c.Request.Context(), actID); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "act deleted"})
}

// Chapters

func (h *Handler) CreateChapter(c *gin.Context) {
	actID, err := parseUUID(c, "aid")
	if err != nil {
		return
	}

	var req CreateChapterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}

	resp, err := h.svc.CreateChapter(c.Request.Context(), actID, req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) GetChapter(c *gin.Context) {
	chapterID, err := parseUUID(c, "cid")
	if err != nil {
		return
	}

	resp, err := h.svc.GetChapter(c.Request.Context(), chapterID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListChapters(c *gin.Context) {
	actID, err := parseUUID(c, "aid")
	if err != nil {
		return
	}

	resp, err := h.svc.ListChaptersByAct(c.Request.Context(), actID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) UpdateChapter(c *gin.Context) {
	chapterID, err := parseUUID(c, "cid")
	if err != nil {
		return
	}

	var req UpdateChapterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}

	resp, err := h.svc.UpdateChapter(c.Request.Context(), chapterID, req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) DeleteChapter(c *gin.Context) {
	chapterID, err := parseUUID(c, "cid")
	if err != nil {
		return
	}

	if err := h.svc.DeleteChapter(c.Request.Context(), chapterID); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "chapter deleted"})
}

// Scenes

func (h *Handler) CreateScene(c *gin.Context) {
	chapterID, err := parseUUID(c, "cid")
	if err != nil {
		return
	}

	var req CreateSceneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}

	resp, err := h.svc.CreateScene(c.Request.Context(), chapterID, req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) GetScene(c *gin.Context) {
	sceneID, err := parseUUID(c, "sid")
	if err != nil {
		return
	}

	resp, err := h.svc.GetScene(c.Request.Context(), sceneID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListScenes(c *gin.Context) {
	chapterID, err := parseUUID(c, "cid")
	if err != nil {
		return
	}

	resp, err := h.svc.ListScenes(c.Request.Context(), chapterID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) UpdateScene(c *gin.Context) {
	sceneID, err := parseUUID(c, "sid")
	if err != nil {
		return
	}

	var req UpdateSceneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}

	resp, err := h.svc.UpdateScene(c.Request.Context(), sceneID, req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) DeleteScene(c *gin.Context) {
	sceneID, err := parseUUID(c, "sid")
	if err != nil {
		return
	}

	if err := h.svc.DeleteScene(c.Request.Context(), sceneID); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "scene deleted"})
}

// ── Git / Chronicle handlers ──────────────────────────────────────────────────

func (h *Handler) GitStatus(c *gin.Context) {
	id, err := parseUUID(c, "id")
	if err != nil {
		return
	}

	resp, err := h.svc.GitStatus(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Chronicle(c *gin.Context) {
	id, err := parseUUID(c, "id")
	if err != nil {
		return
	}

	var req ChronicleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}

	entry, err := h.svc.Chronicle(c.Request.Context(), id, req)
	if errors.Is(err, ErrNothingToChronicle) {
		c.JSON(http.StatusOK, gin.H{"message": "nothing to chronicle", "last_chronicle": entry})
		return
	}
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, entry)
}

func (h *Handler) Lore(c *gin.Context) {
	id, err := parseUUID(c, "id")
	if err != nil {
		return
	}

	page, perPage := parsePagination(c)
	entries, err := h.svc.Lore(c.Request.Context(), id, page, perPage)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, entries)
}

func (h *Handler) Echo(c *gin.Context) {
	id, err := parseUUID(c, "id")
	if err != nil {
		return
	}

	fromSHA := c.Query("from")
	toSHA := c.Query("to")
	if fromSHA == "" || toSHA == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "query params 'from' and 'to' are required"})
		return
	}

	resp, err := h.svc.Echo(c.Request.Context(), id, fromSHA, toSHA)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListTimelines(c *gin.Context) {
	id, err := parseUUID(c, "id")
	if err != nil {
		return
	}

	timelines, err := h.svc.Timelines(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, timelines)
}

func (h *Handler) Diverge(c *gin.Context) {
	id, err := parseUUID(c, "id")
	if err != nil {
		return
	}

	var req DivergeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}

	timeline, err := h.svc.Diverge(c.Request.Context(), id, req)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, timeline)
}

func (h *Handler) TravelTo(c *gin.Context) {
	id, err := parseUUID(c, "id")
	if err != nil {
		return
	}

	status, err := h.svc.TravelTo(c.Request.Context(), id, c.Param("tname"))
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *Handler) Canonize(c *gin.Context) {
	id, err := parseUUID(c, "id")
	if err != nil {
		return
	}

	result, err := h.svc.Canonize(c.Request.Context(), id, c.Param("tname"))
	if err != nil {
		handleError(c, err)
		return
	}
	// A Paradox is a business outcome, not a server error.
	if result.HasParadox {
		c.JSON(http.StatusConflict, result)
		return
	}
	c.JSON(http.StatusOK, result)
}

// Helpers

func parsePagination(c *gin.Context) (page, perPage int) {
	page = 1
	perPage = 20
	if p := c.Query("page"); p != "" {
		if v, err := parseIntQuery(p); err == nil && v > 0 {
			page = v
		}
	}
	if pp := c.Query("per_page"); pp != "" {
		if v, err := parseIntQuery(pp); err == nil && v > 0 && v <= 100 {
			perPage = v
		}
	}
	return
}

func parseIntQuery(s string) (int, error) {
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

// Helpers

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
	c.JSON(http.StatusInternalServerError, gin.H{"message": "internal error"})
}
