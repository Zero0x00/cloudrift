package handlers

import (
	"errors"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	"cloudrift/internal/api/schema"
	"cloudrift/internal/models"
	"cloudrift/internal/scans"
)

type remediationGroupDef struct {
	key   string
	label string
	why   string
	apply func([]models.Finding) []models.Finding
}

var remediationGroupDefs = []remediationGroupDef{
	{
		key:   "reclaimable",
		label: "Reclaimable assets",
		why:   "Recover unused assets and remove recurring risk cost quickly.",
		apply: func(findings []models.Finding) []models.Finding {
			return filterFindings(findings, schema.FindingsAppliedFilter{Claimability: "reclaimable"})
		},
	},
	{
		key:   "stale_external_trust",
		label: "Stale external trust",
		why:   "Review stale external trust paths that are likely over-permissive.",
		apply: func(findings []models.Finding) []models.Finding {
			t := true
			return filterFindings(findings, schema.FindingsAppliedFilter{Module: "external_access", TrustStale: &t})
		},
	},
	{
		key:   "admin_like_external",
		label: "Admin-like external roles",
		why:   "Reduce high-blast-radius third-party access first.",
		apply: func(findings []models.Finding) []models.Finding {
			t := true
			return filterFindings(findings, schema.FindingsAppliedFilter{Module: "external_access", AdminLike: &t})
		},
	},
	{
		key:   "dangling_edge",
		label: "Dangling edge",
		why:   "Remove abandoned edges that keep exposure open without value.",
		apply: func(findings []models.Finding) []models.Finding {
			return filterFindings(findings, schema.FindingsAppliedFilter{Module: "orphaned_edge", Claimability: "dangling"})
		},
	},
	{
		key:   "broken_edge",
		label: "Broken edge / invalid config",
		why:   "Fix invalid edge state before it creates hidden access paths.",
		apply: func(findings []models.Finding) []models.Finding {
			return filterFindings(findings, schema.FindingsAppliedFilter{Module: "orphaned_edge", Claimability: "broken"})
		},
	},
}

func ListRemediationGroups(outputDir string) http.HandlerFunc {
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

		items := make([]schema.RemediationGroupItem, 0, len(remediationGroupDefs))
		for _, def := range remediationGroupDefs {
			matches := def.apply(findings)
			if len(matches) == 0 {
				continue
			}
			totalRisk := 0.0
			top := topExample(matches)
			for _, finding := range matches {
				totalRisk += finding.MonthlyRiskCost
			}
			items = append(items, schema.RemediationGroupItem{
				Key:                     def.key,
				Label:                   def.label,
				Why:                     def.why,
				FindingCount:            len(matches),
				TotalMonthlyRiskCostUSD: totalRisk,
				TopExample:              top,
			})
		}

		sort.Slice(items, func(i, j int) bool {
			if items[i].TotalMonthlyRiskCostUSD != items[j].TotalMonthlyRiskCostUSD {
				return items[i].TotalMonthlyRiskCostUSD > items[j].TotalMonthlyRiskCostUSD
			}
			if items[i].FindingCount != items[j].FindingCount {
				return items[i].FindingCount > items[j].FindingCount
			}
			return items[i].Label < items[j].Label
		})

		writeJSON(w, http.StatusOK, schema.RemediationGroupsResponse{
			ScanID: scanID,
			Items:  items,
		})
	}
}

func topExample(findings []models.Finding) string {
	if len(findings) == 0 {
		return ""
	}
	best := findings[0]
	for _, f := range findings[1:] {
		if f.MonthlyRiskCost > best.MonthlyRiskCost {
			best = f
			continue
		}
		if f.MonthlyRiskCost == best.MonthlyRiskCost && strings.Compare(f.Title, best.Title) < 0 {
			best = f
		}
	}
	return strings.TrimSpace(best.Title)
}

