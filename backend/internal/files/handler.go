// Package files serves locally stored assets (wiki images, export downloads)
// behind HMAC-signed URLs. See pkg/storage for the signing scheme.
package files

import (
	"errors"
	"log/slog"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/jconder44/nexustale/pkg/storage"
)

type Handler struct {
	store *storage.Client
}

func NewHandler(store *storage.Client) *Handler {
	return &Handler{store: store}
}

// RegisterRoutes mounts GET /files/*key on the unauthenticated v1 group.
// Authorization is the signature in the query string, not a bearer token —
// <img src> and download links cannot send an Authorization header.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/files/*key", h.Serve)
}

func (h *Handler) Serve(c *gin.Context) {
	key := strings.TrimPrefix(c.Param("key"), "/")

	// Verify before touching the filesystem. Bad signature, expired link and
	// malformed key all look identical to the caller so the route can't be
	// used to probe which keys exist.
	if !h.store.VerifySignature(key, c.Query("exp"), c.Query("sig")) {
		c.JSON(http.StatusForbidden, gin.H{"message": "invalid or expired link"})
		return
	}

	f, modTime, err := h.store.Open(key)
	if errors.Is(err, storage.ErrNotFound) || errors.Is(err, storage.ErrInvalidKey) {
		c.JSON(http.StatusNotFound, gin.H{"message": "file not found"})
		return
	}
	if err != nil {
		slog.Error("files: open failed", "key", key, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "could not read file"})
		return
	}
	defer f.Close()

	ct := storage.ContentTypeFor(key)
	c.Header("Content-Type", ct)
	// Never let a browser second-guess the type we declared.
	c.Header("X-Content-Type-Options", "nosniff")
	// The URL is a bearer capability; keep it out of shared caches.
	c.Header("Cache-Control", "private, max-age=300")
	if !strings.HasPrefix(ct, "image/") {
		c.Header("Content-Disposition", `attachment; filename="`+path.Base(key)+`"`)
	}
	http.ServeContent(c.Writer, c.Request, "", modTime, f)
}
