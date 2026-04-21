package handlers

import (
	"errors"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"cloudrift/internal/api/schema"
	"cloudrift/internal/models"
	"cloudrift/internal/scorers"
	"cloudrift/internal/scans"
)

const (
	defaultTopFixesLimit = 25
	maxTopFixesLimit     = 100
	minTopFixesLimit     = 1
)

// ListTopFixes returns the highest-priority findings for a scan (server-ranked).
func ListTopFixes(outputDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scanID, ok := scanIDFromPath(outputDir, chi.URLParam(r, "id"))
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid_scan_id", "invalid scan id", nil)
			return
		}
		_, findings, err := scans.LoadScanArtifacts(outputDir, scanID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				writeError(w, http.StatusNotFound, "scan_not_found", "scan not found", map[string]any{"scan_id": scanID})
				return
			}
			writeError(w, http.StatusInternalServerError, "scan_load_error", "failed to load scan artifacts", nil)
			return
		}

		limit := parseTopFixesLimit(r.URL.Query().Get("limit"))

		type ranked struct {
			f     models.Finding
			score float64
		}
		rankedList := make([]ranked, 0, len(findings))
		for _, f := range findings {
			rankedList = append(rankedList, ranked{f: f, score: scorers.PriorityScore(f)})
		}
		sort.SliceStable(rankedList, func(i, j int) bool {
			return scorers.PriorityLess(rankedList[i].f, rankedList[j].f, rankedList[i].score, rankedList[j].score)
		})
		if len(rankedList) > limit {
			rankedList = rankedList[:limit]
		}

		items := make([]schema.TopFixItem, 0, len(rankedList))
		for _, row := range rankedList {
			items = append(items, schema.TopFixItem{
				FindingListItem: toFindingListItem(row.f),
				PriorityScore:   row.score,
				Reason:          scorers.PriorityReason(row.f),
			})
		}

		writeJSON(w, http.StatusOK, schema.TopFixesResponse{
			ScanID: scanID,
			Items:  items,
			Limit:  limit,
		})
	}
}

func parseTopFixesLimit(raw string) int {
	s := strings.TrimSpace(raw)
	if s == "" {
		return defaultTopFixesLimit
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < minTopFixesLimit {
		return defaultTopFixesLimit
	}
	if n > maxTopFixesLimit {
		return maxTopFixesLimit
	}
	return n
}
