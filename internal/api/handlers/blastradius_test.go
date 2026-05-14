package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Zero0x00/cloudrift/internal/api/schema"
	"github.com/Zero0x00/cloudrift/internal/blastradius"
	"github.com/Zero0x00/cloudrift/internal/models"
)

func TestBlastRadiusFindingSummary_neo4jDisabled(t *testing.T) {
	dir := t.TempDir()
	writeScan(t, dir, "scan-a", time.Now().UTC(), []models.Finding{{
		ID:          "f1",
		Title:       "x",
		Severity:    models.SeverityCritical,
		Module:      models.ModuleOrphanedEdge,
		AffectedARN: "arn:aws:iam::1:role/R",
	}})
	svc := blastradius.NewService(nil, dir)
	req := withParams(httptest.NewRequest(http.MethodGet, "/api/scans/scan-a/findings/f1/blast-radius/summary", nil), map[string]string{
		"id":  "scan-a",
		"fid": "f1",
	})
	rr := httptest.NewRecorder()
	BlastRadiusFindingSummary(svc, dir)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	var sum schema.BlastRadiusSummary
	if err := json.Unmarshal(rr.Body.Bytes(), &sum); err != nil {
		t.Fatal(err)
	}
	if sum.GraphAvailable || sum.SourceFindingID != "f1" {
		t.Fatalf("unexpected %+v", sum)
	}
}

func TestBlastRadiusFindingExplorer_neo4jDisabled(t *testing.T) {
	dir := t.TempDir()
	writeScan(t, dir, "scan-a", time.Now().UTC(), []models.Finding{{
		ID:          "f1",
		Title:       "x",
		Severity:    models.SeverityHigh,
		Module:      models.ModuleOrphanedEdge,
		AffectedARN: "arn:aws:iam::1:role/R",
	}})
	svc := blastradius.NewService(nil, dir)
	req := withParams(httptest.NewRequest(http.MethodGet, "/api/scans/scan-a/findings/f1/blast-radius/explorer", nil), map[string]string{
		"id":  "scan-a",
		"fid": "f1",
	})
	rr := httptest.NewRecorder()
	BlastRadiusFindingExplorer(svc, dir)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	var ex schema.BlastExplorerResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &ex); err != nil {
		t.Fatal(err)
	}
	if ex.Summary.GraphAvailable {
		t.Fatalf("graph should be off")
	}
	if len(ex.Nodes) != 0 {
		t.Fatalf("no graph nodes when unavailable")
	}
}

func TestBlastRadiusEntitySummary_missingEntityID(t *testing.T) {
	dir := t.TempDir()
	writeScan(t, dir, "scan-a", time.Now().UTC(), []models.Finding{})
	svc := blastradius.NewService(nil, dir)
	req := withParam(httptest.NewRequest(http.MethodGet, "/api/scans/scan-a/blast-radius/entity/summary", nil), "id", "scan-a")
	rr := httptest.NewRecorder()
	BlastRadiusEntitySummary(svc, dir)(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestBlastRadiusEntitySummary_invalidEntityEncoding(t *testing.T) {
	dir := t.TempDir()
	writeScan(t, dir, "scan-a", time.Now().UTC(), []models.Finding{})
	svc := blastradius.NewService(nil, dir)
	u := "/scans/scan-a/blast-radius/entity/summary?entity_id=not-valid-base64!!!"
	req := withParam(httptest.NewRequest(http.MethodGet, u, nil), "id", "scan-a")
	rr := httptest.NewRecorder()
	BlastRadiusEntitySummary(svc, dir)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var sum schema.BlastRadiusSummary
	_ = json.Unmarshal(rr.Body.Bytes(), &sum)
	if sum.GraphUnavailableReason != string(blastradius.ReasonUnknownRoot) {
		t.Fatalf("expected unknown root, got %#v", sum)
	}
}

func TestBlastRadiusRoutes_chiParams(t *testing.T) {
	dir := t.TempDir()
	writeScan(t, dir, "scan-a", time.Now().UTC(), []models.Finding{{
		ID: "f1", Title: "x", Severity: models.SeverityCritical, Module: models.ModuleOrphanedEdge, AffectedARN: "arn:x",
	}})
	r := chi.NewRouter()
	svc := blastradius.NewService(nil, dir)
	r.Get("/scans/{id}/findings/{fid}/blast-radius/summary", BlastRadiusFindingSummary(svc, dir))
	req := httptest.NewRequest(http.MethodGet, "/scans/scan-a/findings/f1/blast-radius/summary", nil)
	req = withParams(req, map[string]string{"id": "scan-a", "fid": "f1"})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("%d", rr.Code)
	}
}

func TestBlastRadiusPrincipalSummary_missingPrincipalID(t *testing.T) {
	dir := t.TempDir()
	writeScan(t, dir, "scan-a", time.Now().UTC(), []models.Finding{})
	svc := blastradius.NewService(nil, dir)
	req := withParam(httptest.NewRequest(http.MethodGet, "/api/scans/scan-a/principals/blast-radius/summary", nil), "id", "scan-a")
	rr := httptest.NewRecorder()
	BlastRadiusPrincipalSummary(svc, dir)(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestBlastRadiusPrincipalSummary_invalidPrincipalID(t *testing.T) {
	dir := t.TempDir()
	writeScan(t, dir, "scan-a", time.Now().UTC(), []models.Finding{})
	svc := blastradius.NewService(nil, dir)
	req := withParam(
		httptest.NewRequest(http.MethodGet, "/api/scans/scan-a/principals/blast-radius/summary?principal_id=not-base64-principal", nil),
		"id", "scan-a",
	)
	rr := httptest.NewRecorder()
	BlastRadiusPrincipalSummary(svc, dir)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	var sum schema.BlastRadiusSummary
	if err := json.Unmarshal(rr.Body.Bytes(), &sum); err != nil {
		t.Fatal(err)
	}
	if sum.GraphUnavailableReason != string(blastradius.ReasonUnknownRoot) {
		t.Fatalf("expected unknown root, got %#v", sum)
	}
}

func TestBlastRadiusPrincipalExplorer_neo4jDisabled(t *testing.T) {
	dir := t.TempDir()
	writeScan(t, dir, "scan-a", time.Now().UTC(), []models.Finding{})
	svc := blastradius.NewService(nil, dir)
	pid := blastradius.EncodePrincipalID("arn:aws:iam::123456789012:role/PlatformAdmin", "role", "123456789012")
	req := withParam(
		httptest.NewRequest(http.MethodGet, "/api/scans/scan-a/principals/blast-radius/explorer?principal_id="+pid+"&mode=attack_path", nil),
		"id", "scan-a",
	)
	rr := httptest.NewRecorder()
	BlastRadiusPrincipalExplorer(svc, dir)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	var ex schema.BlastExplorerResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &ex); err != nil {
		t.Fatal(err)
	}
	if ex.Focus.PrincipalID == "" || ex.Summary.SourcePrincipalID == "" {
		t.Fatalf("expected principal ids in focus and summary: %#v", ex)
	}
	if ex.Summary.Mode != "attack_path" {
		t.Fatalf("expected attack_path mode, got %s", ex.Summary.Mode)
	}
	if ex.Summary.GraphAvailable {
		t.Fatalf("expected graph unavailable with nil driver")
	}
}

func TestBlastRadiusExplorerExpand_requiresRootContext(t *testing.T) {
	dir := t.TempDir()
	writeScan(t, dir, "scan-a", time.Now().UTC(), []models.Finding{})
	svc := blastradius.NewService(nil, dir)
	req := withParam(
		httptest.NewRequest(http.MethodGet, "/api/scans/scan-a/blast-radius/explorer/expand?node_id=arn:x", nil),
		"id", "scan-a",
	)
	rr := httptest.NewRecorder()
	BlastRadiusExplorerExpand(svc, dir)(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestBlastRadiusExplorerExpand_graphUnavailable(t *testing.T) {
	dir := t.TempDir()
	writeScan(t, dir, "scan-a", time.Now().UTC(), []models.Finding{{
		ID:          "f1",
		Title:       "x",
		Severity:    models.SeverityCritical,
		Module:      models.ModuleOrphanedEdge,
		AffectedARN: "arn:aws:iam::1:role/R",
	}})
	svc := blastradius.NewService(nil, dir)
	req := withParam(
		httptest.NewRequest(http.MethodGet, "/api/scans/scan-a/blast-radius/explorer/expand?node_id=arn:aws:iam::1:role/R&finding_id=f1&mode=attack_path", nil),
		"id", "scan-a",
	)
	rr := httptest.NewRecorder()
	BlastRadiusExplorerExpand(svc, dir)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	var resp schema.BlastExplorerExpansionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.GraphUnavailable || resp.GraphUnavailableReason == "" {
		t.Fatalf("expected graph unavailable response: %#v", resp)
	}
}
