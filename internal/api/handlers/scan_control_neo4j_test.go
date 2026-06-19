package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Neo4j export success path requires a live Neo4j (driver is created internally and is
// not injectable here), so these tests cover the deterministic guards: the endpoint must
// refuse cleanly when Neo4j is not configured. The projection itself is covered by the
// graph package's WriteScan tests (fake Execer).
func TestSetBuildVersion(t *testing.T) {
	orig := buildVersion
	t.Cleanup(func() { buildVersion = orig })

	SetBuildVersion("v9.9.9-test")
	if buildVersion != "v9.9.9-test" {
		t.Fatalf("SetBuildVersion did not apply: %q", buildVersion)
	}
	// Empty values are ignored (keep the previously-set version).
	SetBuildVersion("   ")
	if buildVersion != "v9.9.9-test" {
		t.Fatalf("empty SetBuildVersion should be ignored, got %q", buildVersion)
	}
}

func TestExportToNeo4j_NotConfigured(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cloudrift.toml")
	if err := os.WriteFile(cfgPath, []byte("[output]\noutput_dir = \""+dir+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Defaults set neo4j uri/username/password_env, so "configured" hinges on the password
	// env var being present — clear it to force the not-configured branch.
	t.Setenv("CLOUDRIFT_NEO4J_PASSWORD", "")

	s := NewScanControlCenter(dir, cfgPath)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/scans/demo/neo4j-export", nil)
	s.ExportToNeo4j()(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400 when Neo4j unconfigured, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "neo4j_not_configured") {
		t.Fatalf("want neo4j_not_configured error, got %s", rr.Body.String())
	}
}
