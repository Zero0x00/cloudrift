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
	router := NewRouter(t.TempDir(), "", static)

	apiReq := httptest.NewRequest(http.MethodGet, "/api/scans", nil)
	apiRR := httptest.NewRecorder()
	router.ServeHTTP(apiRR, apiReq)
	if apiRR.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/scans, got %d", apiRR.Code)
	}

	catalogReq := httptest.NewRequest(http.MethodGet, "/api/alerts/catalog", nil)
	catalogRR := httptest.NewRecorder()
	router.ServeHTTP(catalogRR, catalogReq)
	if catalogRR.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/alerts/catalog, got %d", catalogRR.Code)
	}

	staticReq := httptest.NewRequest(http.MethodGet, "/app", nil)
	staticRR := httptest.NewRecorder()
	router.ServeHTTP(staticRR, staticReq)
	if staticRR.Code != http.StatusOK {
		t.Fatalf("expected 200 for static fallback, got %d", staticRR.Code)
	}
}

func TestNewRouter_OptionalTokenAuth(t *testing.T) {
	static := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")}}
	t.Setenv(APITokenEnv, "s3cr3t")
	router := NewRouter(t.TempDir(), "", static)

	// No credentials → 401 (gate covers API and static).
	noAuth := httptest.NewRecorder()
	router.ServeHTTP(noAuth, httptest.NewRequest(http.MethodGet, "/api/scans", nil))
	if noAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", noAuth.Code)
	}
	if noAuth.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("expected WWW-Authenticate challenge")
	}

	// Wrong token → 401.
	wrong := httptest.NewRequest(http.MethodGet, "/api/scans", nil)
	wrong.SetBasicAuth("anyuser", "nope")
	wrongRR := httptest.NewRecorder()
	router.ServeHTTP(wrongRR, wrong)
	if wrongRR.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong token, got %d", wrongRR.Code)
	}

	// Correct token (as Basic password) → 200.
	ok := httptest.NewRequest(http.MethodGet, "/api/scans", nil)
	ok.SetBasicAuth("anyuser", "s3cr3t")
	okRR := httptest.NewRecorder()
	router.ServeHTTP(okRR, ok)
	if okRR.Code != http.StatusOK {
		t.Fatalf("expected 200 with correct token, got %d", okRR.Code)
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

func TestRouter_SetsSecurityHeaders(t *testing.T) {
	router := NewRouter(t.TempDir(), "", fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")},
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("missing X-Content-Type-Options header")
	}
	if rr.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("missing X-Frame-Options header")
	}
	if rr.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("missing Content-Security-Policy header")
	}
}

var _ fs.FS = fstest.MapFS{}
