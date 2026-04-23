package alerting

import (
	"strings"

	"cloudrift/internal/models"
)

// normalizeScopeIDs returns trimmed non-empty scope entries, preserving order.
func normalizeScopeIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		out = append(out, id)
	}
	return out
}

// RuleAppliesToScan returns whether the rule should evaluate for this scan directory.
// Empty scan scope means all scans; otherwise scanID must match one listed ID exactly.
func RuleAppliesToScan(rule AlertRule, scanID string) bool {
	allowed := normalizeScopeIDs(rule.Scope.ScanIDs)
	if len(allowed) == 0 {
		return true
	}
	scanID = strings.TrimSpace(scanID)
	for _, id := range allowed {
		if id == scanID {
			return true
		}
	}
	return false
}

// FilterFindingsByAccountScope keeps findings whose account_id is in the rule scope.
// Empty account scope means no filtering (all findings retained).
func FilterFindingsByAccountScope(rule AlertRule, findings []models.Finding) []models.Finding {
	allowed := normalizeScopeIDs(rule.Scope.AccountIDs)
	if len(allowed) == 0 {
		return findings
	}
	set := make(map[string]struct{}, len(allowed))
	for _, id := range allowed {
		set[id] = struct{}{}
	}
	out := make([]models.Finding, 0, len(findings))
	for _, f := range findings {
		aid := strings.TrimSpace(f.AccountID)
		if _, ok := set[aid]; ok {
			out = append(out, f)
		}
	}
	return out
}
