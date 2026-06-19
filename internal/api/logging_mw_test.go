package api

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func TestRequestLogger(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(requestLogger)
	r.Get("/ok", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/boom", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	r.Post("/do", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) })

	do := func(method, path string) {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, path, nil))
	}
	do(http.MethodGet, "/ok")
	do(http.MethodGet, "/boom")
	do(http.MethodPost, "/do")

	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 log lines, got %d:\n%s", len(lines), out)
	}
	// Every line is the structured http_request event with a request id.
	for _, ln := range lines {
		if !strings.Contains(ln, "msg=http_request") || !strings.Contains(ln, "request_id=") {
			t.Errorf("line missing event/request_id: %s", ln)
		}
	}
	// 500 → ERROR; safe GET 200 → DEBUG (kept quiet); mutation → INFO (audit trail).
	if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "status=500") || !strings.Contains(out, "path=/boom") {
		t.Errorf("expected ERROR/500/path=/boom in:\n%s", out)
	}
	if !strings.Contains(out, "level=DEBUG") || !strings.Contains(out, "path=/ok") {
		t.Errorf("expected DEBUG for GET /ok in:\n%s", out)
	}
	if !strings.Contains(out, "level=INFO") || !strings.Contains(out, "path=/do") {
		t.Errorf("expected INFO for POST /do in:\n%s", out)
	}
}
