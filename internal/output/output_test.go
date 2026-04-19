package output

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cloudrift/internal/models"
)

func TestWriteJSONCSVMarkdown(t *testing.T) {
	dir := t.TempDir()
	findings := []models.Finding{{
		ID:                "f1",
		Severity:          models.SeverityHigh,
		Module:            models.ModuleOrphanedEdge,
		Hostname:          "a.example.com",
		AccountID:         "1",
		Claimability:      models.ClaimDangling,
		MonthlyDirectCost: 1.5,
		MonthlyRiskCost:   10,
		Impact:            "impact",
		Recommendation:    "fix",
		Evidence:          map[string]any{"target_service": "s3_website"},
	}}
	if err := WriteJSON(filepath.Join(dir, "f.json"), findings); err != nil {
		t.Fatal(err)
	}
	if err := WriteCSV(filepath.Join(dir, "f.csv"), findings); err != nil {
		t.Fatal(err)
	}
	if err := WriteMarkdown(filepath.Join(dir, "f.md"), findings); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "f.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "a.example.com") {
		t.Fatalf("expected hostname in markdown")
	}
	table := RenderTable(findings)
	if !strings.Contains(table, "Service") || !strings.Contains(table, "s3_website") {
		t.Fatalf("expected service column/table value in table output")
	}
	csvBytes, err := os.ReadFile(filepath.Join(dir, "f.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(csvBytes), "monthly_direct_cost_usd") {
		t.Fatalf("expected direct cost column in csv")
	}
}

func TestSanitizeCSVCell_FormulaInjectionProtection(t *testing.T) {
	got := sanitizeCSVCell("=SUM(1,2)")
	if got != "'=SUM(1,2)" {
		t.Fatalf("expected prefixed csv cell, got %q", got)
	}
}
