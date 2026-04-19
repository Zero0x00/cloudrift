package api

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestNewRouter_MountsAPIAndStatic(t *testing.T) {
	static := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")},
	}
	router := NewRouter(t.TempDir(), static)

	apiReq := httptest.NewRequest(http.MethodGet, "/api/scans", nil)
	apiRR := httptest.NewRecorder()
	router.ServeHTTP(apiRR, apiReq)
	if apiRR.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/scans, got %d", apiRR.Code)
	}

	staticReq := httptest.NewRequest(http.MethodGet, "/app", nil)
	staticRR := httptest.NewRecorder()
	router.ServeHTTP(staticRR, staticReq)
	if staticRR.Code != http.StatusOK {
		t.Fatalf("expected 200 for static fallback, got %d", staticRR.Code)
	}
}

func TestStaticRouter_NoFS(t *testing.T) {
	h := staticRouter(nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestStaticRouter_ServesAsset(t *testing.T) {
	h := staticRouter(fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("index")},
		"app.js":     &fstest.MapFile{Data: []byte("console.log(1)")},
	})
	req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

var _ fs.FS = fstest.MapFS{}
