package output

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zero0x00/cloudrift/internal/models"
)

func TestWriteSARIF(t *testing.T) {
	findings := []models.Finding{
		{
			ID: "abc123", Title: "docs.example.com -> reclaimable", Severity: models.SeverityCritical,
			Module: models.ModuleOrphanedEdge, Claimability: models.ClaimReclaimable,
			AffectedARN: "arn:aws:route53:::hostedzone/Z1/docs.example.com", Hostname: "docs.example.com",
			AccountID: "111111111111", Recommendation: "Remove the DNS record", MonthlyRiskCost: 2.5,
		},
		{
			ID: "def456", Title: "External trust on VendorRole", Severity: models.SeverityMedium,
			Module: models.ModuleExternalAccess, AffectedARN: "arn:aws:iam::111111111111:role/VendorRole",
			AccountID: "111111111111",
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "out.sarif")
	if err := WriteSARIF(path, findings); err != nil {
		t.Fatalf("WriteSARIF: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var log sarifLog
	if err := json.Unmarshal(b, &log); err != nil {
		t.Fatalf("output is not valid JSON/SARIF: %v", err)
	}

	if log.Version != "2.1.0" || log.Schema == "" {
		t.Errorf("bad header: version=%q schema=%q", log.Version, log.Schema)
	}
	if len(log.Runs) != 1 {
		t.Fatalf("want 1 run, got %d", len(log.Runs))
	}
	run := log.Runs[0]
	if run.Tool.Driver.Name != "Cloudrift" {
		t.Errorf("driver name: %q", run.Tool.Driver.Name)
	}
	// Two distinct modules → two rules.
	if len(run.Tool.Driver.Rules) != 2 {
		t.Errorf("want 2 rules, got %d", len(run.Tool.Driver.Rules))
	}
	if len(run.Results) != 2 {
		t.Fatalf("want 2 results, got %d", len(run.Results))
	}

	// Severity → level mapping and location wiring.
	byRule := map[string]sarifResult{}
	for _, r := range run.Results {
		byRule[r.RuleID] = r
	}
	edge, ok := byRule["orphaned_edge"]
	if !ok || edge.Level != "error" { // critical -> error
		t.Errorf("orphaned_edge result: %+v", edge)
	}
	if len(edge.Locations) != 1 || edge.Locations[0].LogicalLocations[0].FullyQualifiedName != "arn:aws:route53:::hostedzone/Z1/docs.example.com" {
		t.Errorf("edge location wrong: %+v", edge.Locations)
	}
	if ext := byRule["external_access"]; ext.Level != "warning" { // medium -> warning
		t.Errorf("external_access level: want warning, got %q", ext.Level)
	}
}

func TestSarifLevelMapping(t *testing.T) {
	cases := map[models.Severity]string{
		models.SeverityCritical: "error",
		models.SeverityHigh:     "error",
		models.SeverityMedium:   "warning",
		models.SeverityLow:      "note",
		models.SeverityInfo:     "note",
	}
	for sev, want := range cases {
		if got := sarifLevel(sev); got != want {
			t.Errorf("sarifLevel(%s): want %s got %s", sev, want, got)
		}
	}
}
