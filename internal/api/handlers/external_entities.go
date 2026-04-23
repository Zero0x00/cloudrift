package handlers

import (
	"errors"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	"cloudrift/internal/api/schema"
	"cloudrift/internal/blastradius"
	"cloudrift/internal/models"
	"cloudrift/internal/scans"
)

const externalEntityKeySep = "\x1e"

// aggregateExternalEntities groups external_access findings by (external_principal, principal_type, external_account_id).
// Empty/missing dimension values normalize to "unknown" for stable keys (aligned with principal_type filter semantics).
func aggregateExternalEntities(findings []models.Finding) []schema.ExternalEntityRow {
	type bucket struct {
		externalPrincipal string
		principalType     string
		externalAccountID string
		roles             map[string]struct{}
		internalAccounts  map[string]struct{}
		maxRank           int
		highestSeverity   string
		staleRoles        map[string]struct{}
		privilegedRoles   map[string]struct{}
		adminLikeRoles    map[string]struct{}
		totalRisk         float64
		findingCount      int
	}

	byKey := make(map[string]*bucket)

	for _, f := range findings {
		if !strings.EqualFold(string(f.Module), "external_access") {
			continue
		}
		ep := normalizeExternalEntityPart(strEvidence(f.Evidence, "external_principal"))
		pt := normalizeExternalEntityPart(strings.ToLower(strings.TrimSpace(evidencePrincipalType(f.Evidence))))
		ea := normalizeExternalEntityPart(strEvidence(f.Evidence, "external_account_id"))
		key := ep + externalEntityKeySep + pt + externalEntityKeySep + ea

		b, ok := byKey[key]
		if !ok {
			b = &bucket{
				externalPrincipal: ep,
				principalType:     pt,
				externalAccountID: ea,
				roles:             map[string]struct{}{},
				internalAccounts:  map[string]struct{}{},
				staleRoles:        map[string]struct{}{},
				privilegedRoles:   map[string]struct{}{},
				adminLikeRoles:    map[string]struct{}{},
			}
			byKey[key] = b
		}

		roleKey := trustedRoleKey(f)
		b.roles[roleKey] = struct{}{}
		b.findingCount++

		acct := strings.TrimSpace(f.AccountID)
		if acct == "" {
			acct = "unknown"
		}
		b.internalAccounts[acct] = struct{}{}

		r := severityRank(f.Severity)
		if r > b.maxRank {
			b.maxRank = r
			b.highestSeverity = strings.ToLower(string(f.Severity))
		}
		b.totalRisk += f.MonthlyRiskCost

		if evidenceTrustVerdictStale(f.Evidence) {
			b.staleRoles[roleKey] = struct{}{}
		}
		if strings.EqualFold(evidenceTrustClassification(f.Evidence), "privileged") {
			b.privilegedRoles[roleKey] = struct{}{}
		}
		if evidenceAdminLike(f.Evidence) {
			b.adminLikeRoles[roleKey] = struct{}{}
		}
	}

	out := make([]schema.ExternalEntityRow, 0, len(byKey))
	for _, b := range byKey {
		principalID := ""
		if len(b.roles) == 1 {
			var role string
			for r := range b.roles {
				role = strings.TrimSpace(r)
				break
			}
			if role != "" && !strings.HasPrefix(role, "finding:") && strings.HasPrefix(strings.ToLower(role), "arn:") {
				pType := principalTypeForARN(role, b.principalType)
				accountID := accountFromARN(role)
				principalID = blastradius.EncodePrincipalID(role, pType, accountID)
			}
		}
		out = append(out, schema.ExternalEntityRow{
			EntityID:                   blastradius.EncodeExternalEntityID(b.externalPrincipal, b.principalType, b.externalAccountID),
			PrincipalID:                principalID,
			ExternalPrincipal:          b.externalPrincipal,
			PrincipalType:              b.principalType,
			ExternalAccountID:          b.externalAccountID,
			UniqueTrustedRoleCount:     len(b.roles),
			UniqueInternalAccountCount: len(b.internalAccounts),
			HighestSeverity:            b.highestSeverity,
			TotalMonthlyRiskCostUSD:    b.totalRisk,
			StaleRoleCount:             len(b.staleRoles),
			PrivilegedRoleCount:        len(b.privilegedRoles),
			AdminLikeRoleCount:         len(b.adminLikeRoles),
			ExternalAccessFindingCount: b.findingCount,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		ri := severityRank(models.Severity(out[i].HighestSeverity))
		rj := severityRank(models.Severity(out[j].HighestSeverity))
		if ri != rj {
			return ri > rj
		}
		if out[i].TotalMonthlyRiskCostUSD != out[j].TotalMonthlyRiskCostUSD {
			return out[i].TotalMonthlyRiskCostUSD > out[j].TotalMonthlyRiskCostUSD
		}
		return entitySortKey(out[i]) < entitySortKey(out[j])
	})

	return out
}

func principalTypeForARN(arn, fallback string) string {
	a := strings.ToLower(strings.TrimSpace(arn))
	if strings.Contains(a, ":role/") {
		return "role"
	}
	if strings.Contains(a, ":user/") {
		return "user"
	}
	fb := strings.TrimSpace(fallback)
	if fb == "" || strings.EqualFold(fb, "unknown") {
		return "principal"
	}
	return fb
}

func accountFromARN(arn string) string {
	parts := strings.Split(strings.TrimSpace(arn), ":")
	if len(parts) >= 6 && strings.EqualFold(parts[0], "arn") {
		return strings.TrimSpace(parts[4])
	}
	return ""
}

func entitySortKey(e schema.ExternalEntityRow) string {
	return e.ExternalPrincipal + "|" + e.PrincipalType + "|" + e.ExternalAccountID
}

func trustedRoleKey(f models.Finding) string {
	r := strings.TrimSpace(strEvidence(f.Evidence, "role_arn"))
	if r != "" {
		return r
	}
	r = strings.TrimSpace(f.AffectedARN)
	if r != "" {
		return r
	}
	return "finding:" + f.ID
}

func normalizeExternalEntityPart(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	return s
}

// summaryExternalEntityRollups derives summary-only counters and preview rows from the same aggregation as the list endpoint.
func summaryExternalEntityRollups(findings []models.Finding) (
	entityCount int,
	withStale int,
	withPrivileged int,
	withAdminLike int,
	entityByPrincipalType []schema.ExternalEntityPrincipalTypeCount,
	preview []schema.ExternalEntityRow,
) {
	rows := aggregateExternalEntities(findings)
	entityCount = len(rows)
	for _, e := range rows {
		if e.StaleRoleCount > 0 {
			withStale++
		}
		if e.PrivilegedRoleCount > 0 {
			withPrivileged++
		}
		if e.AdminLikeRoleCount > 0 {
			withAdminLike++
		}
	}
	entityByPrincipalType = buildExternalEntityByPrincipalType(rows)
	if len(rows) > 5 {
		preview = append(preview, rows[:5]...)
	} else {
		preview = rows
	}
	return entityCount, withStale, withPrivileged, withAdminLike, entityByPrincipalType, preview
}

func buildExternalEntityByPrincipalType(rows []schema.ExternalEntityRow) []schema.ExternalEntityPrincipalTypeCount {
	m := map[string]int{}
	for _, e := range rows {
		pt := strings.ToLower(strings.TrimSpace(e.PrincipalType))
		if pt == "" {
			pt = "unknown"
		}
		m[pt]++
	}
	out := make([]schema.ExternalEntityPrincipalTypeCount, 0, len(m))
	for t, c := range m {
		out = append(out, schema.ExternalEntityPrincipalTypeCount{PrincipalType: t, EntityCount: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].EntityCount == out[j].EntityCount {
			return out[i].PrincipalType < out[j].PrincipalType
		}
		return out[i].EntityCount > out[j].EntityCount
	})
	return out
}

func filterExternalEntityRows(rows []schema.ExternalEntityRow, principalType, externalPrincipal, externalAccountID string) []schema.ExternalEntityRow {
	pt := strings.TrimSpace(strings.ToLower(principalType))
	ep := strings.TrimSpace(externalPrincipal)
	ea := strings.TrimSpace(externalAccountID)
	if pt == "" && ep == "" && ea == "" {
		return rows
	}
	out := make([]schema.ExternalEntityRow, 0, len(rows))
	for _, row := range rows {
		if pt != "" {
			got := strings.ToLower(strings.TrimSpace(row.PrincipalType))
			if got == "" {
				got = "unknown"
			}
			if pt == "unknown" {
				if got != "unknown" {
					continue
				}
			} else if got != pt {
				continue
			}
		}
		if ep != "" {
			if !externalEntityDimMatches(ep, row.ExternalPrincipal) {
				continue
			}
		}
		if ea != "" {
			if !externalEntityDimMatches(ea, row.ExternalAccountID) {
				continue
			}
		}
		out = append(out, row)
	}
	return out
}

func applyExternalEntityFeatureFilters(
	rows []schema.ExternalEntityRow,
	hasStale, hasPriv, hasAdmin *bool,
) []schema.ExternalEntityRow {
	if hasStale == nil && hasPriv == nil && hasAdmin == nil {
		return rows
	}
	out := make([]schema.ExternalEntityRow, 0, len(rows))
	for _, row := range rows {
		if hasStale != nil && *hasStale && row.StaleRoleCount == 0 {
			continue
		}
		if hasPriv != nil && *hasPriv && row.PrivilegedRoleCount == 0 {
			continue
		}
		if hasAdmin != nil && *hasAdmin && row.AdminLikeRoleCount == 0 {
			continue
		}
		out = append(out, row)
	}
	return out
}

func externalEntityDimMatches(wantRaw, gotNorm string) bool {
	want := strings.TrimSpace(wantRaw)
	if strings.EqualFold(want, "unknown") {
		return gotNorm == "unknown"
	}
	return strings.EqualFold(strings.TrimSpace(want), gotNorm)
}

// ListExternalEntities serves GET /api/scans/:id/external-entities (paginated, same rows as aggregation used for summary).
func ListExternalEntities(outputDir string) http.HandlerFunc {
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

		all := aggregateExternalEntities(findings)
		principalType := strings.TrimSpace(r.URL.Query().Get("principal_type"))
		extPrin := strings.TrimSpace(r.URL.Query().Get("external_principal"))
		extAcct := strings.TrimSpace(r.URL.Query().Get("external_account_id"))
		filtered := filterExternalEntityRows(all, principalType, extPrin, extAcct)
		hasStale := parseQueryBoolTrueOnly(r, "has_stale_role")
		hasPriv := parseQueryBoolTrueOnly(r, "has_privileged_role")
		hasAdmin := parseQueryBoolTrueOnly(r, "has_admin_like_role")
		filtered = applyExternalEntityFeatureFilters(filtered, hasStale, hasPriv, hasAdmin)
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
		pageItems := make([]schema.ExternalEntityRow, 0, end-start)
		if start < end {
			pageItems = append(pageItems, filtered[start:end]...)
		}

		writeJSON(w, http.StatusOK, schema.ExternalEntitiesResponse{
			ScanID: scanID,
			Items:  pageItems,
			Filters: schema.ExternalEntitiesAppliedFilter{
				PrincipalType:     principalType,
				ExternalPrincipal: extPrin,
				ExternalAccountID: extAcct,
				HasStaleRole:      hasStale,
				HasPrivilegedRole: hasPriv,
				HasAdminLikeRole:  hasAdmin,
			},
			Pagination: schema.PaginationMeta{
				Page:       page,
				PageSize:   pageSize,
				TotalItems: total,
				TotalPages: totalPages,
			},
		})
	}
}
