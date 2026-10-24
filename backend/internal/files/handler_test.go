package files

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jconder44/nexustale/pkg/storage"
)

func setup(t *testing.T) (*gin.Engine, *storage.Client) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st, err := storage.New(storage.Config{Root: t.TempDir(), SigningKey: []byte("k")})
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	NewHandler(st).RegisterRoutes(r.Group("/api/v1"))
	return r, st
}

func get(r http.Handler, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
	return w
}

func TestServe(t *testing.T) {
	r, st := setup(t)
	ctx := context.Background()

	if err := st.Put(ctx, "wiki/entities/e1/a.png", strings.NewReader("PNGDATA")); err != nil {
		t.Fatal(err)
	}
	if err := st.Put(ctx, "exports/p/j.epub", strings.NewReader("EPUBDATA")); err != nil {
		t.Fatal(err)
	}

	img, _ := st.SignedURL("wiki/entities/e1/a.png", time.Hour)
	w := get(r, img)
	if w.Code != 200 || w.Body.String() != "PNGDATA" {
		t.Fatalf("image: %d %q", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("content-type = %q", ct)
	}
	if w.Header().Get("Content-Disposition") != "" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("unexpected headers: %v", w.Header())
	}

	ex, _ := st.SignedURL("exports/p/j.epub", time.Hour)
	w = get(r, ex)
	if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("export: %d %v", w.Code, w.Header())
	}

	// Range requests (large exports, resumable downloads).
	req := httptest.NewRequest(http.MethodGet, ex, nil)
	req.Header.Set("Range", "bytes=0-3")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusPartialContent || w.Body.String() != "EPUB" {
		t.Errorf("range: %d %q", w.Code, w.Body.String())
	}
}

func TestServeRejects(t *testing.T) {
	r, st := setup(t)
	_ = st.Put(context.Background(), "exports/p/j.epub", strings.NewReader("x"))
	good, _ := st.SignedURL("exports/p/j.epub", time.Hour)

	cases := map[string]string{
		"no signature":      "/api/v1/files/exports/p/j.epub",
		"tampered sig":      strings.Replace(good, "sig=", "sig=00", 1),
		"expired":           func() string { u, _ := st.SignedURL("exports/p/j.epub", -time.Minute); return u }(),
		"other key, sig re": strings.Replace(good, "j.epub", "k.epub", 1),
		"traversal":         "/api/v1/files/../../etc/passwd?exp=1&sig=00",
	}
	for name, u := range cases {
		if w := get(r, u); w.Code == 200 {
			t.Errorf("%s: served (status %d)", name, w.Code)
		}
	}

	// Valid signature but object missing → 404, not 500.
	missing, _ := st.SignedURL("exports/p/gone.epub", time.Hour)
	if w := get(r, missing); w.Code != http.StatusNotFound {
		t.Errorf("missing: %d", w.Code)
	}
}
