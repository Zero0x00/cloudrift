package main

import (
	_ "embed"
	"encoding/json"

	"github.com/Zero0x00/cloudrift/internal/models"
)

//go:embed testdata/bundled_demo_findings.json
var bundledDemoFindingsJSON []byte

// findingsForScan returns the canonical 18-row visualization bundle for scan_id "demo",
// otherwise the smaller programmatic demo set used for timestamped demo-* scans.
func findingsForScan(scanID string) []models.Finding {
	if scanID != "demo" {
		return demoFindings(scanID, bankDemoAccounts())
	}
	var out []models.Finding
	if err := json.Unmarshal(bundledDemoFindingsJSON, &out); err != nil {
		return demoFindings(scanID, bankDemoAccounts())
	}
	for i := range out {
		out[i].ScanID = scanID
	}
	return out
}
