package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloudrift/internal/api/schema"
	"cloudrift/internal/models"
)

func TestListTopFixesCriticalBeforeMedium(t *testing.T) {
	dir := t.TempDir()
	findings := []models.Finding{
		{
			ID: "m1", Title: "medium", Severity: models.SeverityMedium, Module: models.ModuleOrphanedEdge,
			Claimability: models.ClaimUnknown, AccountID: "111", AffectedARN: "arn:m", MonthlyRiskCost: 10,
		},
		{
			ID: "c1", Title: "critical", Severity: models.SeverityCritical, Module: models.ModuleOrphanedEdge,
			Claimability: models.ClaimReclaimable, AccountID: "222", AffectedARN: "arn:c", MonthlyRiskCost: 10,
		},
	}
	writeScan(t, dir, "scan-prio", time.Now().UTC(), findings)

	req := withParam(httptest.NewRequest(http.MethodGet, "/api/scans/scan-prio/top-fixes?limit=5", nil), "id", "scan-prio")
	rr := httptest.NewRecorder()
	ListTopFixes(dir).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp schema.TopFixesResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}
	if resp.Items[0].ID != "c1" {
		t.Fatalf("want critical first, got %+v", resp.Items)
	}
	if resp.Items[0].PriorityScore <= resp.Items[1].PriorityScore {
		t.Fatalf("first score should exceed second: %v %v", resp.Items[0].PriorityScore, resp.Items[1].PriorityScore)
	}
	if resp.ScanID != "scan-prio" || resp.Limit != 5 {
		t.Fatalf("unexpected envelope: %+v", resp)
	}
}

func TestListTopFixesExternalSignalsBoostScore(t *testing.T) {
	dir := t.TempDir()
	base := models.Finding{
		ID: "x1", Title: "ext", Severity: models.SeverityHigh, Module: models.ModuleExternalAccess,
		Claimability: models.ClaimUnknown, AccountID: "111", AffectedARN: "arn:x", MonthlyRiskCost: 5,
		Evidence: map[string]any{
			"verdict":            "stale_review_now",
			"permission_visibility": map[string]any{
				"classification": "privileged",
				"capabilities": map[string]any{"admin_like": true},
			},
		},
	}
	plain := models.Finding{
		ID: "p1", Title: "plain", Severity: models.SeverityHigh, Module: models.ModuleOrphanedEdge,
		Claimability: models.ClaimUnknown, AccountID: "222", AffectedARN: "arn:p", MonthlyRiskCost: 5,
	}
	writeScan(t, dir, "scan-ext", time.Now().UTC(), []models.Finding{plain, base})

	req := withParam(httptest.NewRequest(http.MethodGet, "/api/scans/scan-ext/top-fixes", nil), "id", "scan-ext")
	rr := httptest.NewRecorder()
	ListTopFixes(dir).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp schema.TopFixesResponse
	mustDecode(t, rr, &resp)
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}
	if resp.Items[0].ID != "x1" {
		t.Fatalf("external signals should rank higher: %+v", resp.Items)
	}
}

func TestListTopFixesScanNotFound(t *testing.T) {
	dir := t.TempDir()
	req := withParam(httptest.NewRequest(http.MethodGet, "/api/scans/missing/top-fixes", nil), "id", "missing")
	rr := httptest.NewRecorder()
	ListTopFixes(dir).ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestParseTopFixesLimitClamped(t *testing.T) {
	if parseTopFixesLimit("") != defaultTopFixesLimit {
		t.Fatal()
	}
	if parseTopFixesLimit("200") != maxTopFixesLimit {
		t.Fatal()
	}
	if parseTopFixesLimit("0") != defaultTopFixesLimit {
		t.Fatal()
	}
	if parseTopFixesLimit("10") != 10 {
		t.Fatal()
	}
}
