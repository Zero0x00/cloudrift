package api

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"cloudrift/internal/alerting"
	"cloudrift/internal/api/handlers"
	"cloudrift/internal/blastradius"
	"cloudrift/internal/config"
	"cloudrift/internal/queryv2"
)

func NewRouter(outputDir, configPath string, staticFS fs.FS) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)

	r.Mount("/api", apiRouter(outputDir, configPath))
	r.Mount("/", staticRouter(staticFS))
	return r
}

func apiRouter(outputDir, configPath string) http.Handler {
	r := chi.NewRouter()
	cfg, _ := config.Load(configPath)
	connCtx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	neo := blastradius.TryConnect(connCtx, cfg)
	cancel()
	blast := blastradius.NewService(neo, outputDir)

	alertSvc := alerting.NewService(outputDir, strings.TrimSpace(os.Getenv("CLOUDRIFT_APP_BASE_URL")), blast)
	querySvc := queryv2.NewService(outputDir, cfg, blast)
	queryHandler := handlers.NewQueryHandler(querySvc)
	control := handlers.NewScanControlCenter(outputDir, configPath)
	control.SetAlertService(alertSvc)
	alertingHandler := handlers.NewAlertingHandler(outputDir, alertSvc)
	r.Get("/scans", handlers.ListScans(outputDir))
	r.Get("/scans/{id}/summary", handlers.GetScanSummary(outputDir))
	r.Get("/scans/{id}/external-entities", handlers.ListExternalEntities(outputDir))
	r.Get("/scans/{id}/findings", handlers.ListFindings(outputDir))
	r.Get("/scans/{id}/remediation-groups", handlers.ListRemediationGroups(outputDir))
	r.Get("/scans/{id}/top-fixes", handlers.ListTopFixes(outputDir))
	r.Get("/scans/{id}/findings/{fid}", handlers.GetFinding(outputDir))
	r.Get("/scans/{id}/findings/{fid}/blast-radius/summary", handlers.BlastRadiusFindingSummary(blast, outputDir))
	r.Get("/scans/{id}/findings/{fid}/blast-radius/explorer", handlers.BlastRadiusFindingExplorer(blast, outputDir))
	r.Get("/scans/{id}/blast-radius/entity/summary", handlers.BlastRadiusEntitySummary(blast, outputDir))
	r.Get("/scans/{id}/blast-radius/entity/explorer", handlers.BlastRadiusEntityExplorer(blast, outputDir))
	r.Get("/scans/{id}/principals/blast-radius/summary", handlers.BlastRadiusPrincipalSummary(blast, outputDir))
	r.Get("/scans/{id}/principals/blast-radius/explorer", handlers.BlastRadiusPrincipalExplorer(blast, outputDir))
	r.Get("/scans/{id}/blast-radius/explorer/expand", handlers.BlastRadiusExplorerExpand(blast, outputDir))
	r.Get("/scans/{id}/accounts", handlers.ListAccounts(outputDir))
	r.Get("/diff", handlers.DiffScans(outputDir))
	r.Get("/scan/progress", handlers.ScanProgressWS(control))
	r.Get("/runtime/status", control.RuntimeStatus())
	r.Post("/runtime/validate-profile", control.ValidateProfile())
	r.Post("/scan/start", control.StartScan())
	r.Get("/scan/status", control.CurrentRunStatus())
	r.Get("/scan/history", control.RunHistory())
	r.Get("/alerts/catalog", alertingHandler.Catalog())
	r.Get("/alerts/routing", alertingHandler.GetRoutingCatalog())
	r.Put("/alerts/routing", alertingHandler.PutRoutingCatalog())
	r.Get("/alerts/rules", alertingHandler.ListRules())
	r.Get("/alerts/rules/{ruleID}", alertingHandler.GetRule())
	r.Post("/alerts/rules", alertingHandler.CreateRule())
	r.Put("/alerts/rules/{ruleID}", alertingHandler.UpdateRule())
	r.Post("/alerts/rules/{ruleID}/enable", alertingHandler.EnableRule(true))
	r.Post("/alerts/rules/{ruleID}/disable", alertingHandler.EnableRule(false))
	r.Post("/alerts/rules/{ruleID}/preview", alertingHandler.PreviewRule())
	r.Post("/alerts/rules/{ruleID}/test", alertingHandler.TestRule())
	r.Get("/alerts/events", alertingHandler.ListEvents())
	r.Post("/query", queryHandler.Query())
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

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		// Blast explorer labels (troika text via drei) rely on a blob: worker. Allow workers from self + blob.
		w.Header().Set(
			"Content-Security-Policy",
			"default-src 'self' 'unsafe-inline'; worker-src 'self' blob:; frame-ancestors 'none'; base-uri 'self'",
		)
		next.ServeHTTP(w, r)
	})
}
