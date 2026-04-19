package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"cloudrift/internal/api/schema"
	"cloudrift/internal/models"
)

func TestListScansNewestFirst(t *testing.T) {
	dir := t.TempDir()
	writeScan(t, dir, "scan-1", time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC), []models.Finding{{ID: "f1", Title: "one", Severity: models.SeverityHigh}})
	writeScan(t, dir, "scan-2", time.Date(2026, 4, 18, 11, 0, 0, 0, time.UTC), []models.Finding{{ID: "f2", Title: "two", Severity: models.SeverityCritical}})

	req := httptest.NewRequest(http.MethodGet, "/api/scans", nil)
	rr := httptest.NewRecorder()
	ListScans(dir).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp schema.ScanListResponse
	mustDecode(t, rr, &resp)
	if len(resp.Items) != 2 || resp.Items[0].ScanID != "scan-2" {
		t.Fatalf("unexpected ordering: %+v", resp.Items)
	}
}

func TestListScansTieBreakScanIDAscending(t *testing.T) {
	dir := t.TempDir()
	ts := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	writeScan(t, dir, "scan-z", ts, []models.Finding{{ID: "z", Title: "z"}})
	writeScan(t, dir, "scan-a", ts, []models.Finding{{ID: "a", Title: "a"}})
	writeScan(t, dir, "scan-m", ts, []models.Finding{{ID: "m", Title: "m"}})

	req := httptest.NewRequest(http.MethodGet, "/api/scans", nil)
	rr := httptest.NewRecorder()
	ListScans(dir).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp schema.ScanListResponse
	mustDecode(t, rr, &resp)
	if len(resp.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(resp.Items))
	}
	want := []string{"scan-a", "scan-m", "scan-z"}
	for i := range want {
		if resp.Items[i].ScanID != want[i] {
			t.Fatalf("tie-break order at %d: want %v got %v", i, want, []string{resp.Items[0].ScanID, resp.Items[1].ScanID, resp.Items[2].ScanID})
		}
	}
}

func TestGetSummaryLatestMatchesListHead(t *testing.T) {
	dir := t.TempDir()
	writeScan(t, dir, "older", time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC), []models.Finding{{ID: "1", Title: "old"}})
	writeScan(t, dir, "newer", time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC), []models.Finding{
		{ID: "2", Title: "new", Severity: models.SeverityCritical},
	})

	reqLatest := withParam(httptest.NewRequest(http.MethodGet, "/api/scans/latest/summary", nil), "id", "latest")
	rr := httptest.NewRecorder()
	GetScanSummary(dir).ServeHTTP(rr, reqLatest)
	if rr.Code != http.StatusOK {
		t.Fatalf("latest summary: %d", rr.Code)
	}
	var sum schema.ScanSummaryResponse
	mustDecode(t, rr, &sum)
	if sum.ScanID != "newer" {
		t.Fatalf("expected scan_id newer, got %q", sum.ScanID)
	}
	if sum.FindingCount != 1 || sum.CriticalCount != 1 {
		t.Fatalf("unexpected summary: %+v", sum)
	}
}

func TestGetScanSummaryNotFound(t *testing.T) {
	dir := t.TempDir()
	req := withParam(httptest.NewRequest(http.MethodGet, "/api/scans/x/summary", nil), "id", "x")
	rr := httptest.NewRecorder()
	GetScanSummary(dir).ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 404 or 400, got %d", rr.Code)
	}
}

func TestListFindingsModuleFilterPagination(t *testing.T) {
	dir := t.TempDir()
	findings := []models.Finding{
		{ID: "1", Title: "edge", Severity: models.SeverityHigh, Module: models.ModuleOrphanedEdge, AccountID: "111", AffectedARN: "a"},
		{ID: "2", Title: "trust-a", Severity: models.SeverityHigh, Module: models.ModuleExternalAccess, AccountID: "222", AffectedARN: "b"},
		{ID: "3", Title: "trust-b", Severity: models.SeverityLow, Module: models.ModuleExternalAccess, AccountID: "333", AffectedARN: "c"},
	}
	writeScan(t, dir, "scan-a", time.Now().UTC(), findings)

	req := withParam(httptest.NewRequest(http.MethodGet, "/api/scans/scan-a/findings?module=external_access&page=1&page_size=1", nil), "id", "scan-a")
	rr := httptest.NewRecorder()
	ListFindings(dir).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp schema.FindingsListResponse
	mustDecode(t, rr, &resp)
	if resp.Pagination.TotalItems != 2 || resp.Pagination.TotalPages != 2 || len(resp.Items) != 1 {
		t.Fatalf("expected 2 total external_access, 2 pages, 1 item on page: %+v", resp)
	}
	if resp.Items[0].ID != "2" {
		t.Fatalf("expected first trust finding id 2, got %q", resp.Items[0].ID)
	}
	if resp.Filters.Module != "external_access" {
		t.Fatalf("expected filter echo, got %+v", resp.Filters)
	}
}

func TestListFindingsFiltersAndPagination(t *testing.T) {
	dir := t.TempDir()
	findings := []models.Finding{
		{ID: "1", Title: "S3 one", Severity: models.SeverityHigh, Module: models.ModuleOrphanedEdge, Claimability: models.ClaimReclaimable, AccountID: "111", AffectedARN: "a"},
		{ID: "2", Title: "S3 two", Severity: models.SeverityLow, Module: models.ModuleExternalAccess, Claimability: models.ClaimBroken, AccountID: "222", AffectedARN: "b"},
	}
	writeScan(t, dir, "scan-a", time.Now().UTC(), findings)

	req := withParam(httptest.NewRequest(http.MethodGet, "/api/scans/scan-a/findings?severity=high&search=s3&page=1&page_size=1", nil), "id", "scan-a")
	rr := httptest.NewRecorder()
	ListFindings(dir).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp schema.FindingsListResponse
	mustDecode(t, rr, &resp)
	if len(resp.Items) != 1 || resp.Items[0].ID != "1" {
		t.Fatalf("unexpected findings response: %+v", resp)
	}
}

func TestListFindingsBadPagination(t *testing.T) {
	dir := t.TempDir()
	writeScan(t, dir, "scan-a", time.Now().UTC(), []models.Finding{})
	req := withParam(httptest.NewRequest(http.MethodGet, "/api/scans/scan-a/findings?page=foo", nil), "id", "scan-a")
	rr := httptest.NewRecorder()
	ListFindings(dir).ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestGetFindingNotFound(t *testing.T) {
	dir := t.TempDir()
	writeScan(t, dir, "scan-a", time.Now().UTC(), []models.Finding{{ID: "f1", Title: "t"}})
	req := withParams(httptest.NewRequest(http.MethodGet, "/api/scans/scan-a/findings/missing", nil), map[string]string{"id": "scan-a", "fid": "missing"})
	rr := httptest.NewRecorder()
	GetFinding(dir).ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestGetFindingRejectsUnsafeFindingID(t *testing.T) {
	dir := t.TempDir()
	writeScan(t, dir, "scan-a", time.Now().UTC(), []models.Finding{{ID: "f1", Title: "t"}})
	for _, fid := range []string{"", "..", "a/b", `a\b`, strings.Repeat("x", 200)} {
		req := withParams(httptest.NewRequest(http.MethodGet, "/x", nil), map[string]string{"id": "scan-a", "fid": fid})
		rr := httptest.NewRecorder()
		GetFinding(dir).ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("fid %q: expected 400, got %d", fid, rr.Code)
		}
	}
}

func TestListAccountsAggregation(t *testing.T) {
	dir := t.TempDir()
	writeScan(t, dir, "scan-a", time.Now().UTC(), []models.Finding{
		{ID: "1", Title: "critical", Severity: models.SeverityCritical, AccountID: "111", AccountName: "acct", MonthlyDirectCost: 10, MonthlyRiskCost: 50},
		{ID: "2", Title: "high", Severity: models.SeverityHigh, AccountID: "111", MonthlyDirectCost: 5, MonthlyRiskCost: 20},
	})
	req := withParam(httptest.NewRequest(http.MethodGet, "/api/scans/scan-a/accounts", nil), "id", "scan-a")
	rr := httptest.NewRecorder()
	ListAccounts(dir).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp schema.AccountsBreakdownResponse
	mustDecode(t, rr, &resp)
	if len(resp.Items) != 1 || resp.Items[0].FindingCount != 2 || resp.Items[0].TopFinding != "critical" {
		t.Fatalf("unexpected accounts response: %+v", resp)
	}
}

func TestDiffScans(t *testing.T) {
	dir := t.TempDir()
	writeScan(t, dir, "old", time.Now().UTC(), []models.Finding{
		{ID: "1", Title: "A", AffectedARN: "arn:a"},
		{ID: "2", Title: "B", AffectedARN: "arn:b"},
	})
	writeScan(t, dir, "new", time.Now().UTC(), []models.Finding{
		{ID: "3", Title: "A", AffectedARN: "arn:a"},
		{ID: "4", Title: "C", AffectedARN: "arn:c"},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/diff?old=old&new=new", nil)
	rr := httptest.NewRecorder()
	DiffScans(dir).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp schema.DiffResponse
	mustDecode(t, rr, &resp)
	if resp.UnchangedCount != 1 || len(resp.NewFindings) != 1 || len(resp.ResolvedFindings) != 1 {
		t.Fatalf("unexpected diff response: %+v", resp)
	}
}

func writeScan(t *testing.T, outputDir, scanID string, ts time.Time, findings []models.Finding) {
	t.Helper()
	scanDir := filepath.Join(outputDir, scanID)
	if err := os.MkdirAll(scanDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := models.ScanSnapshot{
		ScanID:       scanID,
		Timestamp:    ts,
		FindingCount: len(findings),
	}
	writeJSONFile(t, filepath.Join(scanDir, "scan-metadata.json"), meta)
	writeJSONFile(t, filepath.Join(scanDir, "findings.json"), findings)
}

func writeJSONFile(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustDecode(t *testing.T, rr *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.Unmarshal(rr.Body.Bytes(), out); err != nil {
		t.Fatal(err)
	}
}

func withParam(req *http.Request, key, value string) *http.Request {
	return withParams(req, map[string]string{key: value})
}

func withParams(req *http.Request, params map[string]string) *http.Request {
	routeCtx := chi.NewRouteContext()
	for k, v := range params {
		routeCtx.URLParams.Add(k, v)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}
