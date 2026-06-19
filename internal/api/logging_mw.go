package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// requestLogger logs one structured line per HTTP request via the slog default. It records
// only request metadata — never headers, query strings, or bodies — so secrets and finding
// contents don't leak into logs. Level is chosen to keep an actionable audit trail without
// poll/asset spam:
//   - 5xx          → Error (server-side errors are now visible, with the request id)
//   - 4xx          → Warn
//   - mutations    → Info  (POST/PUT/DELETE/PATCH: scans started, exports, rule edits — the audit trail)
//   - safe reads   → Debug (GET/HEAD: dashboard polls/assets stay quiet at info)
//
// Place it inside RequestID (so the id is in context) and outside Recoverer (so panics that
// become 500s are logged with their final status).
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)

		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}
		level := slog.LevelInfo
		switch {
		case status >= 500:
			level = slog.LevelError
		case status >= 400:
			level = slog.LevelWarn
		case r.Method == http.MethodGet || r.Method == http.MethodHead:
			level = slog.LevelDebug
		}
		slog.LogAttrs(r.Context(), level, "http_request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", status),
			slog.Int("bytes", ww.BytesWritten()),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)
	})
}
