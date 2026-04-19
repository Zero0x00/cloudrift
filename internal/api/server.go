package api

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"cloudrift/internal/api/handlers"
)

func NewRouter(outputDir, configPath string, staticFS fs.FS) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Mount("/api", apiRouter(outputDir, configPath))
	r.Mount("/", staticRouter(staticFS))
	return r
}

func apiRouter(outputDir, configPath string) http.Handler {
	r := chi.NewRouter()
	control := handlers.NewScanControlCenter(outputDir, configPath)
	r.Get("/scans", handlers.ListScans(outputDir))
	r.Get("/scans/{id}/summary", handlers.GetScanSummary(outputDir))
	r.Get("/scans/{id}/findings", handlers.ListFindings(outputDir))
	r.Get("/scans/{id}/findings/{fid}", handlers.GetFinding(outputDir))
	r.Get("/scans/{id}/accounts", handlers.ListAccounts(outputDir))
	r.Get("/diff", handlers.DiffScans(outputDir))
	r.Get("/scan/progress", handlers.ScanProgressWS(control))
	r.Get("/runtime/status", control.RuntimeStatus())
	r.Post("/runtime/validate-profile", control.ValidateProfile())
	r.Post("/scan/start", control.StartScan())
	r.Get("/scan/status", control.CurrentRunStatus())
	r.Get("/scan/history", control.RunHistory())
	return r
}

func StartServer(port int, outputDir, configPath string, staticFS fs.FS) error {
	addr := ":" + strconvItoa(port)
	return http.ListenAndServe(addr, NewRouter(outputDir, configPath, staticFS))
}

func staticRouter(staticFS fs.FS) http.Handler {
	r := chi.NewRouter()
	if staticFS == nil {
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
		return r
	}

	fileServer := http.FileServer(http.FS(staticFS))
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		reqPath := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
		if reqPath != "/" {
			trimmed := strings.TrimPrefix(reqPath, "/")
			if info, err := fs.Stat(staticFS, trimmed); err == nil && !info.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		index, err := fs.ReadFile(staticFS, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(index)
	})
	return r
}

func strconvItoa(v int) string {
	if v == 0 {
		return "0"
	}
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	buf := [20]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return sign + string(buf[i:])
}
