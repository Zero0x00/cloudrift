package handlers

import (
	"testing"

	"github.com/Zero0x00/cloudrift/internal/models"
)

func extFinding(id, ep, pt, extAcct, roleKey string, sev models.Severity, stale, priv, admin bool) models.Finding {
	ev := map[string]any{
		"external_principal":  ep,
		"principal_type":      pt,
		"external_account_id": extAcct,
		"role_arn":            roleKey,
		"verdict":             "",
		"permission_visibility": map[string]any{
			"classification": "scoped",
			"capabilities": map[string]any{
				"admin_like": admin,
			},
		},
	}
	if stale {
		ev["verdict"] = "stale_review_now"
	}
	if priv {
		pv := ev["permission_visibility"].(map[string]any)
		pv["classification"] = "privileged"
	}
	return models.Finding{
		ID:              id,
		Module:          models.ModuleExternalAccess,
		Severity:        sev,
		AffectedARN:     roleKey,
		AccountID:       "111",
		MonthlyRiskCost: 10,
		Evidence:        ev,
	}
}

func TestAggregateExternalEntities_groupsAndRollups(t *testing.T) {
	// Same external principal + type + ext account, two roles → one entity, 2 trusted roles.
	r1 := extFinding("a", "arn:aws:iam::999:root", "AWS", "999", "role/a", models.SeverityHigh, true, true, false)
	r2 := extFinding("b", "arn:aws:iam::999:root", "AWS", "999", "role/b", models.SeverityCritical, false, false, true)
	rows := aggregateExternalEntities([]models.Finding{r1, r2})
	if len(rows) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(rows))
	}
	e := rows[0]
	if e.UniqueTrustedRoleCount != 2 {
		t.Fatalf("UniqueTrustedRoleCount: got %d want 2", e.UniqueTrustedRoleCount)
	}
	if e.UniqueInternalAccountCount != 1 {
		t.Fatalf("UniqueInternalAccountCount: got %d want 1", e.UniqueInternalAccountCount)
	}
	if e.HighestSeverity != "critical" {
		t.Fatalf("HighestSeverity: got %q", e.HighestSeverity)
	}
	if e.StaleRoleCount != 1 || e.PrivilegedRoleCount != 1 || e.AdminLikeRoleCount != 1 {
		t.Fatalf("role buckets stale/priv/admin: %d %d %d", e.StaleRoleCount, e.PrivilegedRoleCount, e.AdminLikeRoleCount)
	}
	if e.ExternalAccessFindingCount != 2 {
		t.Fatalf("ExternalAccessFindingCount: got %d", e.ExternalAccessFindingCount)
	}
	if e.PrincipalID != "" {
		t.Fatalf("expected principal_id omitted for multi-role entity bucket, got %q", e.PrincipalID)
	}

	ec, st, pr, ad, byPT, prev := summaryExternalEntityRollups([]models.Finding{r1, r2})
	if ec != 1 || st != 1 || pr != 1 || ad != 1 {
		t.Fatalf("summary rollups ec/st/pr/ad: %d %d %d %d", ec, st, pr, ad)
	}
	if len(prev) != 1 || len(byPT) < 1 {
		t.Fatalf("preview / byPT: %d %d", len(prev), len(byPT))
	}
}

func TestAggregateExternalEntities_principalIDWhenSingleRoleDerivable(t *testing.T) {
	r := extFinding("a", "arn:aws:iam::999:root", "AWS", "999", "arn:aws:iam::111111111111:role/vendor-access", models.SeverityHigh, false, false, false)
	rows := aggregateExternalEntities([]models.Finding{r})
	if len(rows) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(rows))
	}
	if rows[0].EntityID == "" {
		t.Fatalf("expected entity id")
	}
	if rows[0].PrincipalID == "" {
		t.Fatalf("expected principal_id for single-role derivable bucket")
	}
}

func TestAggregateExternalEntities_principalIDOmittedForSyntheticRoleKey(t *testing.T) {
	f := extFinding("a", "arn:aws:iam::999:root", "AWS", "999", "", models.SeverityHigh, false, false, false)
	// role_arn missing, trustedRoleKey falls back to affected ARN, clear that too so it degrades to finding key.
	f.AffectedARN = ""
	rows := aggregateExternalEntities([]models.Finding{f})
	if len(rows) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(rows))
	}
	if rows[0].PrincipalID != "" {
		t.Fatalf("expected principal_id omitted for non-derivable role key, got %q", rows[0].PrincipalID)
	}
}

func TestAggregateExternalEntities_unknownDimensions(t *testing.T) {
	r := extFinding("x", "", "", "", "role/x", models.SeverityLow, false, false, false)
	rows := aggregateExternalEntities([]models.Finding{r})
	if len(rows) != 1 {
		t.Fatalf("expected 1 entity")
	}
	if rows[0].ExternalPrincipal != "unknown" || rows[0].PrincipalType != "unknown" || rows[0].ExternalAccountID != "unknown" {
		t.Fatalf("normalization: %+v", rows[0])
	}
}

func TestFilterExternalEntityRows(t *testing.T) {
	r1 := extFinding("a", "P1", "AWS", "1", "role/1", models.SeverityHigh, false, false, false)
	r2 := extFinding("b", "P2", "OIDC", "2", "role/2", models.SeverityMedium, false, false, false)
	all := aggregateExternalEntities([]models.Finding{r1, r2})
	f := filterExternalEntityRows(all, "oidc", "", "")
	if len(f) != 1 || f[0].ExternalPrincipal != "P2" {
		t.Fatalf("principal_type filter: %+v", f)
	}
}
