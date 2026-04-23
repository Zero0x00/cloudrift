package handlers

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"cloudrift/internal/blastradius"
	"cloudrift/internal/scans"
)

// BlastRadiusPrincipalSummary is GET /api/scans/{id}/principals/blast-radius/summary?principal_id=...
func BlastRadiusPrincipalSummary(svc *blastradius.Service, outputDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			svc = blastradius.NewService(nil, outputDir)
		}
		scanID, ok := scanIDFromPath(outputDir, chi.URLParam(r, "id"))
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid_scan_id", "invalid scan id", nil)
			return
		}
		pid := strings.TrimSpace(r.URL.Query().Get("principal_id"))
		if pid == "" {
			writeError(w, http.StatusBadRequest, "invalid_principal_id", "principal_id query is required", nil)
			return
		}
		if _, _, err := scans.LoadScanArtifacts(outputDir, scanID); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				writeError(w, http.StatusNotFound, "scan_not_found", "scan not found", map[string]any{"scan_id": scanID})
				return
			}
			writeError(w, http.StatusInternalServerError, "scan_load_error", "failed to load scan artifacts", nil)
			return
		}
		mode := blastModeOrDefault(r.URL.Query().Get("mode"))
		ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		defer cancel()
		sum, _, _ := svc.PrincipalBlastByID(ctx, scanID, pid, mode)
		writeJSON(w, http.StatusOK, sum)
	}
}

// BlastRadiusPrincipalExplorer is GET /api/scans/{id}/principals/blast-radius/explorer?principal_id=...
func BlastRadiusPrincipalExplorer(svc *blastradius.Service, outputDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			svc = blastradius.NewService(nil, outputDir)
		}
		scanID, ok := scanIDFromPath(outputDir, chi.URLParam(r, "id"))
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid_scan_id", "invalid scan id", nil)
			return
		}
		pid := strings.TrimSpace(r.URL.Query().Get("principal_id"))
		if pid == "" {
			writeError(w, http.StatusBadRequest, "invalid_principal_id", "principal_id query is required", nil)
			return
		}
		if _, _, err := scans.LoadScanArtifacts(outputDir, scanID); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				writeError(w, http.StatusNotFound, "scan_not_found", "scan not found", map[string]any{"scan_id": scanID})
				return
			}
			writeError(w, http.StatusInternalServerError, "scan_load_error", "failed to load scan artifacts", nil)
			return
		}
		mode := blastModeOrDefault(r.URL.Query().Get("mode"))
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		payload := svc.ExplorerFromPrincipalID(ctx, scanID, pid, mode)
		writeJSON(w, http.StatusOK, payload)
	}
}
