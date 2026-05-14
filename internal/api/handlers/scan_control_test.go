package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Zero0x00/cloudrift/internal/api/schema"
)

func TestRuntimeStatusDoesNotExposeSecretValues(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cloudrift.toml")
	const secret = "super-secret-openai-token"
	t.Setenv("OPENAI_API_KEY", secret)
	t.Setenv("CLOUDRIFT_NEO4J_PASSWORD", "neo4j-secret")
	content := `
[aws]
management_profile = "default"

[neo4j]
uri = "bolt://localhost:7687"
username = "neo4j"
password_env = "CLOUDRIFT_NEO4J_PASSWORD"

[embeddings]
openai_api_key_env = "OPENAI_API_KEY"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cc := NewScanControlCenter(dir, cfgPath)
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/status", nil)
	rr := httptest.NewRecorder()
	cc.RuntimeStatus().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if strings.Contains(rr.Body.String(), secret) {
		t.Fatalf("runtime status leaked secret value")
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["openai_configured"] != true {
		t.Fatalf("expected openai_configured true, got %v", got["openai_configured"])
	}
	if _, ok := got["aws_profiles"].([]any); !ok {
		t.Fatalf("expected aws_profiles as [] not null, got %#v", got["aws_profiles"])
	}
}

func TestValidateProfileInvalidMissing(t *testing.T) {
	cc := NewScanControlCenter(t.TempDir(), "")
	req := httptest.NewRequest(http.MethodPost, "/api/runtime/validate-profile", strings.NewReader(`{"profile":"definitely-no-such-profile-123"}`))
	rr := httptest.NewRecorder()
	cc.ValidateProfile().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid profile, got %d", rr.Code)
	}
}

func TestValidateProfileAmbientSourceAllowed(t *testing.T) {
	// Empty profile should validate ambient credential source (may still fail depending env);
	// this asserts endpoint contract (no "missing_profile" hard-fail).
	cc := NewScanControlCenter(t.TempDir(), "")
	req := httptest.NewRequest(http.MethodPost, "/api/runtime/validate-profile", strings.NewReader(`{"profile":""}`))
	rr := httptest.NewRecorder()
	cc.ValidateProfile().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 200 or 400, got %d", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "profile is required") {
		t.Fatalf("unexpected missing_profile requirement for ambient source: %s", rr.Body.String())
	}
}

func TestCurrentRunStatusIdle(t *testing.T) {
	cc := NewScanControlCenter(t.TempDir(), "")
	req := httptest.NewRequest(http.MethodGet, "/api/scan/status", nil)
	rr := httptest.NewRecorder()
	cc.CurrentRunStatus().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"status":"idle"`) {
		t.Fatalf("expected idle status, got %s", rr.Body.String())
	}
}

func TestRunHistoryEmptyArrayWhenNoRuns(t *testing.T) {
	cc := NewScanControlCenter(t.TempDir(), "")
	req := httptest.NewRequest(http.MethodGet, "/api/scan/history", nil)
	rr := httptest.NewRecorder()
	cc.RunHistory().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var raw map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["items"].([]any); !ok {
		t.Fatalf("expected items [] not null, got %#v", raw["items"])
	}
}

func TestStartScanMissingProfileRejected(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cloudrift.toml")
	if err := os.WriteFile(cfgPath, []byte("[aws]\nmanagement_profile = \"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cc := NewScanControlCenter(dir, cfgPath)
	req := httptest.NewRequest(http.MethodPost, "/api/scan/start", strings.NewReader(`{"module":"all","no_http":false,"neo4j":false}`))
	rr := httptest.NewRecorder()
	cc.StartScan().ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted && rr.Code != http.StatusBadRequest {
		t.Fatalf("expected accepted or validation failure, got %d", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "missing profile") || strings.Contains(rr.Body.String(), "profile is required") {
		t.Fatalf("start should allow ambient source when profile empty: %s", rr.Body.String())
	}
}

func TestStartScanRejectsUnknownProvider(t *testing.T) {
	cc := NewScanControlCenter(t.TempDir(), "")
	req := httptest.NewRequest(http.MethodPost, "/api/scan/start", strings.NewReader(`{"profile":"default","module":"all","provider":"weird"}`))
	rr := httptest.NewRecorder()
	cc.StartScan().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid provider, got %d", rr.Code)
	}
}

func TestRunHistoryRingBufferNewestFirstAndBounded(t *testing.T) {
	cc := NewScanControlCenter(t.TempDir(), "")
	cc.mu.Lock()
	for i := 1; i <= 12; i++ {
		cc.appendHistoryLocked(schema.ScanRunHistoryItem{
			RunID:   "run-" + strconv.Itoa(i),
			Status:  "completed",
			Message: "ok",
		})
	}
	cc.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/scan/history", nil)
	rr := httptest.NewRecorder()
	cc.RunHistory().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp schema.ScanRunHistoryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 10 {
		t.Fatalf("expected 10 history items, got %d", len(resp.Items))
	}
	if resp.Items[0].RunID != "run-12" {
		t.Fatalf("expected newest run-12 first, got %s", resp.Items[0].RunID)
	}
	if resp.Items[9].RunID != "run-3" {
		t.Fatalf("expected oldest retained run-3, got %s", resp.Items[9].RunID)
	}
}

func TestRunHistoryRetainsCompletedAndFailed(t *testing.T) {
	cc := NewScanControlCenter(t.TempDir(), "")
	cc.mu.Lock()
	cc.current = schema.ScanRunStatusResponse{
		RunID:     "run-completed",
		Status:    "running",
		Profile:   "p1",
		Module:    "all",
		NoHTTP:    true,
		Neo4j:     false,
		StartedAt: time.Now().UTC().Add(-2 * time.Minute),
	}
	cc.mu.Unlock()
	cc.finishRun("run-completed", "scan-1", "done")

	cc.mu.Lock()
	cc.current = schema.ScanRunStatusResponse{
		RunID:     "run-failed",
		Status:    "running",
		Profile:   "p2",
		Module:    "external_access",
		NoHTTP:    false,
		Neo4j:     true,
		StartedAt: time.Now().UTC().Add(-1 * time.Minute),
	}
	cc.mu.Unlock()
	cc.failRun("run-failed", "failed")

	req := httptest.NewRequest(http.MethodGet, "/api/scan/history", nil)
	rr := httptest.NewRecorder()
	cc.RunHistory().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp schema.ScanRunHistoryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) < 2 {
		t.Fatalf("expected at least 2 runs in history, got %d", len(resp.Items))
	}
	if resp.Items[0].Status != "failed" || resp.Items[1].Status != "completed" {
		t.Fatalf("unexpected status ordering: %+v", resp.Items)
	}
}
