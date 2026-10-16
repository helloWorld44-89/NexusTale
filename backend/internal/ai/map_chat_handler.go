package ai

// Map agent chat handler (D-Map M3) — a stateless, map-scoped sibling of
// WorkshopChat. There's no session persistence (frontend.MapStudio drives
// the conversation; D-Map's draft-until-committed philosophy means nothing
// here needs to survive a reload any more than an unsaved layout edit does).
//
// Route (registered via RegisterRoutes under /projects/:id):
//   POST /projects/:id/maps/:mid/ai/chat → MapChat (SSE)

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jconder44/nexustale/internal/ai/adapters"
)

// MapChat handles POST /projects/:id/maps/:mid/ai/chat.
func (h *Handler) MapChat(c *gin.Context) {
	projectID, userID, ok := resolveIDs(c)
	if !ok {
		return
	}

	mapID, err := uuid.Parse(c.Param("mid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid map id"})
		return
	}

	var req struct {
		Messages  []adapters.Message `json:"messages"`
		Provider  string             `json:"provider"`
		MaxTokens int                `json:"max_tokens"`
		MaxRounds int                `json:"max_rounds"` // 0 → service default (25)
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request", "detail": err.Error()})
		return
	}
	if len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "messages is required"})
		return
	}

	svcReq := MapChatRequest{
		ProjectID: projectID,
		MapID:     mapID,
		Messages:  req.Messages,
		Provider:  req.Provider,
		MaxTokens: req.MaxTokens,
	}

	setSSeHeaders(c)

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		if _, err := h.svc.StreamMapChatWithTools(c.Request.Context(), userID, svcReq, pw, req.MaxRounds); err != nil {
			fmt.Fprintf(pw, "data: {\"error\":%q}\n\n", err.Error())
			fmt.Fprintf(pw, "data: [DONE]\n\n")
		}
	}()

	c.Stream(func(w io.Writer) bool {
		buf := make([]byte, 4096)
		n, err := pr.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
		}
		return err == nil
	})
}
