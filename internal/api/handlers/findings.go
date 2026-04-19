package handlers

import (
	"errors"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	"cloudrift/internal/api/schema"
	"cloudrift/internal/models"
	"cloudrift/internal/scans"
)

// Finding IDs come from scan artifacts (hashes, CLI ids). Reject path-like or huge values
// so URL params cannot be abused for log noise or confused with traversal probes.
var safeFindingIDPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,128}$`)

func isSafeFindingID(id string) bool {
	return id != "" &&
		safeFindingIDPattern.MatchString(id) &&
		!strings.Contains(id, "/") &&
		!strings.Contains(id, "\\") &&
		id != "." &&
		id != ".."
}

func ListFindings(outputDir string) http.HandlerFunc {
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

		page, pageSize, err := parsePagination(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_pagination", err.Error(), nil)
			return
		}

		filters := schema.FindingsAppliedFilter{
			Severity:     strings.TrimSpace(r.URL.Query().Get("severity")),
			Module:       strings.TrimSpace(r.URL.Query().Get("module")),
			AccountID:    strings.TrimSpace(r.URL.Query().Get("account_id")),
			Claimability: strings.TrimSpace(r.URL.Query().Get("claimability")),
			Search:       strings.TrimSpace(r.URL.Query().Get("search")),
		}
		filtered := filterFindings(findings, filters)
		total := len(filtered)
		totalPages := 0
		if total > 0 {
			totalPages = (total + pageSize - 1) / pageSize
		}
		start := (page - 1) * pageSize
		end := start + pageSize
		if start > total {
			start = total
		}
		if end > total {
			end = total
		}
		pageItems := make([]schema.FindingListItem, 0, end-start)
		for _, finding := range filtered[start:end] {
			pageItems = append(pageItems, toFindingListItem(finding))
		}

		writeJSON(w, http.StatusOK, schema.FindingsListResponse{
			Items: pageItems,
			Pagination: schema.PaginationMeta{
				Page:       page,
				PageSize:   pageSize,
				TotalItems: total,
				TotalPages: totalPages,
			},
			Filters: filters,
		})
	}
}

func GetFinding(outputDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scanID, ok := scanIDFromPath(outputDir, chi.URLParam(r, "id"))
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid_scan_id", "invalid scan id", nil)
			return
		}
		findingID := strings.TrimSpace(chi.URLParam(r, "fid"))
		if !isSafeFindingID(findingID) {
			writeError(w, http.StatusBadRequest, "invalid_finding_id", "invalid finding id", nil)
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

		for _, finding := range findings {
			if finding.ID != findingID {
				continue
			}
			detail := toFindingDetailItem(scanID, finding)
			writeJSON(w, http.StatusOK, schema.FindingDetailResponse{Item: detail})
			return
		}
		writeError(w, http.StatusNotFound, "finding_not_found", "finding not found", map[string]any{
			"scan_id":    scanID,
			"finding_id": findingID,
		})
	}
}

func filterFindings(findings []models.Finding, filters schema.FindingsAppliedFilter) []models.Finding {
	search := strings.ToLower(strings.TrimSpace(filters.Search))
	filtered := make([]models.Finding, 0, len(findings))
	for _, finding := range findings {
		if filters.Severity != "" && !strings.EqualFold(string(finding.Severity), filters.Severity) {
			continue
		}
		if filters.Module != "" && !strings.EqualFold(string(finding.Module), filters.Module) {
			continue
		}
		if filters.AccountID != "" && !strings.EqualFold(finding.AccountID, filters.AccountID) {
			continue
		}
		if filters.Claimability != "" && !strings.EqualFold(string(finding.Claimability), filters.Claimability) {
			continue
		}
		if search != "" && !matchesSearch(finding, search) {
			continue
		}
		filtered = append(filtered, finding)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].AffectedARN == filtered[j].AffectedARN {
			return filtered[i].ID < filtered[j].ID
		}
		return filtered[i].AffectedARN < filtered[j].AffectedARN
	})
	return filtered
}

func matchesSearch(finding models.Finding, search string) bool {
	fields := []string{
		finding.ID,
		finding.Title,
		finding.AffectedARN,
		finding.AccountID,
		finding.AccountName,
		finding.Hostname,
		finding.Team,
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), search) {
			return true
		}
	}
	return false
}

func toFindingListItem(finding models.Finding) schema.FindingListItem {
	return schema.FindingListItem{
		ID:                   finding.ID,
		Title:                finding.Title,
		Severity:             strings.ToLower(string(finding.Severity)),
		Module:               strings.ToLower(string(finding.Module)),
		Claimability:         strings.ToLower(string(finding.Claimability)),
		AffectedARN:          finding.AffectedARN,
		AccountID:            finding.AccountID,
		AccountName:          finding.AccountName,
		OUPath:               finding.OUPath,
		Team:                 finding.Team,
		Hostname:             finding.Hostname,
		MonthlyDirectCostUSD: finding.MonthlyDirectCost,
		MonthlyRiskCostUSD:   finding.MonthlyRiskCost,
	}
}

func toFindingDetailItem(scanID string, finding models.Finding) schema.FindingDetailItem {
	item := schema.FindingDetailItem{
		FindingListItem: toFindingListItem(finding),
		Impact:          finding.Impact,
		Recommendation:  finding.Recommendation,
		RemediationCmd:  finding.RemediationCmd,
		ScanID:          scanID,
		Evidence:        finding.Evidence,
	}
	if strings.EqualFold(string(finding.Module), "external_access") {
		item.Trust = toTrustDisplay(finding.Evidence)
	}
	return item
}

func toTrustDisplay(evidence map[string]any) *schema.TrustDisplay {
	if len(evidence) == 0 {
		return &schema.TrustDisplay{}
	}
	td := &schema.TrustDisplay{
		RoleARN:           strEvidence(evidence, "role_arn"),
		RoleName:          strEvidence(evidence, "role_name"),
		ExternalPrincipal: strEvidence(evidence, "external_principal"),
		PrincipalType:     strEvidence(evidence, "principal_type"),
		ExternalAccountID: strEvidence(evidence, "external_account_id"),
		Verdict:           strEvidence(evidence, "verdict"),
		Reason:            strEvidence(evidence, "reason"),
		AdminEvalState:    strEvidence(evidence, "admin_eval_state"),
		ActivityStatus:    strEvidence(evidence, "activity_status"),
	}
	if v, ok := intEvidence(evidence, "days_since_used"); ok {
		td.DaysSinceUsed = &v
	}
	if v, ok := boolEvidence(evidence, "unknown_vendor"); ok {
		td.UnknownVendor = &v
	}
	return td
}

func strEvidence(e map[string]any, key string) string {
	v, ok := e[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return s
	}
	return ""
}

func intEvidence(e map[string]any, key string) (int, bool) {
	v, ok := e[key]
	if !ok || v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case int:
		return t, true
	case float64:
		return int(t), true
	default:
		return 0, false
	}
}

func boolEvidence(e map[string]any, key string) (bool, bool) {
	v, ok := e[key]
	if !ok || v == nil {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

func sortFindingItems(items []schema.FindingListItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].AffectedARN == items[j].AffectedARN {
			return items[i].ID < items[j].ID
		}
		return items[i].AffectedARN < items[j].AffectedARN
	})
}
