package output

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"cloudrift/internal/models"

	"github.com/xuri/excelize/v2"
)

func TestWriteExcel_WorkbookStructure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.xlsx")
	if err := WriteExcel(path, sampleFindings()); err != nil {
		t.Fatalf("WriteExcel error: %v", err)
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile error: %v", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	expected := []string{"Findings", "Cost Summary", "Trust Report"}
	if len(sheets) != len(expected) {
		t.Fatalf("expected %d sheets, got %d (%v)", len(expected), len(sheets), sheets)
	}
	for i := range expected {
		if sheets[i] != expected[i] {
			t.Fatalf("expected sheet order %v, got %v", expected, sheets)
		}
	}
}

func TestWriteExcel_FindingsSheetHeadersAndValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "findings.xlsx")
	findings := sampleFindings()
	if err := WriteExcel(path, findings); err != nil {
		t.Fatalf("WriteExcel error: %v", err)
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile error: %v", err)
	}
	defer f.Close()

	if got, _ := f.GetCellValue("Findings", "A1"); got != "ID" {
		t.Fatalf("expected A1 header ID, got %q", got)
	}
	if got, _ := f.GetCellValue("Findings", "C2"); got == "" {
		t.Fatalf("expected severity value in C2")
	}
	if got, _ := f.GetCellValue("Findings", "B2"); got == "" {
		t.Fatalf("expected title in B2")
	}
	if got, _ := f.GetCellValue("Findings", "R2"); got == "" {
		t.Fatalf("expected EvidenceJSON value in R2")
	}

	criticalStyleID, highStyleID, mediumStyleID := -1, -1, -1
	for row := 2; row < 20; row++ {
		v, _ := f.GetCellValue("Findings", "C"+strconv.Itoa(row))
		styleID, _ := f.GetCellStyle("Findings", "C"+strconv.Itoa(row))
		switch v {
		case "critical":
			criticalStyleID = styleID
		case "high":
			highStyleID = styleID
		case "medium":
			mediumStyleID = styleID
		}
	}
	if criticalStyleID == 0 || highStyleID <= 0 || mediumStyleID <= 0 {
		t.Fatalf("expected non-zero severity style ids")
	}
	if criticalStyleID == highStyleID || highStyleID == mediumStyleID {
		t.Fatalf("expected distinct styles for critical/high/medium")
	}
}

func TestWriteExcel_CostSummaryRowsAndTotals(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cost.xlsx")
	if err := WriteExcel(path, sampleFindings()); err != nil {
		t.Fatalf("WriteExcel error: %v", err)
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile error: %v", err)
	}
	defer f.Close()

	rows, err := f.GetRows("Cost Summary")
	if err != nil {
		t.Fatalf("GetRows error: %v", err)
	}
	if len(rows) < 3 {
		t.Fatalf("expected grouped rows plus totals, got %d rows", len(rows))
	}
	last := rows[len(rows)-1]
	if len(last) == 0 || last[0] != "TOTAL" {
		t.Fatalf("expected TOTAL row at bottom, got %v", last)
	}

	direct, err := f.GetCellValue("Cost Summary", "E2")
	if err != nil {
		t.Fatalf("GetCellValue error: %v", err)
	}
	if _, err := strconv.ParseFloat(direct, 64); err != nil {
		t.Fatalf("expected numeric direct cost in E2, got %q", direct)
	}
}

func TestWriteExcel_TrustReportColumnsAndOptionals(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trust.xlsx")
	findings := sampleFindings()
	if err := WriteExcel(path, findings); err != nil {
		t.Fatalf("WriteExcel error: %v", err)
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile error: %v", err)
	}
	defer f.Close()

	if got, _ := f.GetCellValue("Trust Report", "A1"); got != "RoleARN" {
		t.Fatalf("expected RoleARN header, got %q", got)
	}
	if got, _ := f.GetCellValue("Trust Report", "D1"); got != "PrincipalType" {
		t.Fatalf("expected PrincipalType header, got %q", got)
	}

	// First trust row has external account id.
	if got, _ := f.GetCellValue("Trust Report", "E2"); got != "222222222222" {
		t.Fatalf("expected external account id in E2, got %q", got)
	}
	// Second trust row is optional/missing external account id.
	if got, _ := f.GetCellValue("Trust Report", "E3"); got != "" {
		t.Fatalf("expected empty external account id for optional field, got %q", got)
	}
	if got, _ := f.GetCellValue("Trust Report", "G2"); got == "" {
		t.Fatalf("expected DaysSinceUsed in G2")
	}
	if got, _ := f.GetCellValue("Trust Report", "I3"); got == "" {
		t.Fatalf("expected reason value in I3")
	}
}

func TestWriteExcel_EmptyDatasetStillValid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.xlsx")
	if err := WriteExcel(path, nil); err != nil {
		t.Fatalf("WriteExcel error: %v", err)
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile error: %v", err)
	}
	defer f.Close()

	if sheets := f.GetSheetList(); len(sheets) != 3 {
		t.Fatalf("expected 3 sheets for empty dataset, got %d", len(sheets))
	}
	if got, _ := f.GetCellValue("Findings", "A1"); got != "ID" {
		t.Fatalf("expected findings headers even when empty")
	}
	if got, _ := f.GetCellValue("Cost Summary", "A1"); got != "AccountID" {
		t.Fatalf("expected cost summary headers even when empty")
	}
	if got, _ := f.GetCellValue("Trust Report", "A1"); got != "RoleARN" {
		t.Fatalf("expected trust headers even when empty")
	}
}

func TestWriteExcel_DeterministicLayout(t *testing.T) {
	path1 := filepath.Join(t.TempDir(), "a.xlsx")
	path2 := filepath.Join(t.TempDir(), "b.xlsx")
	findings := sampleFindings()
	if err := WriteExcel(path1, findings); err != nil {
		t.Fatalf("WriteExcel path1 error: %v", err)
	}
	if err := WriteExcel(path2, findings); err != nil {
		t.Fatalf("WriteExcel path2 error: %v", err)
	}

	f1, err := excelize.OpenFile(path1)
	if err != nil {
		t.Fatalf("OpenFile path1 error: %v", err)
	}
	defer f1.Close()
	f2, err := excelize.OpenFile(path2)
	if err != nil {
		t.Fatalf("OpenFile path2 error: %v", err)
	}
	defer f2.Close()

	for _, sheet := range []string{"Findings", "Cost Summary", "Trust Report"} {
		r1, _ := f1.GetRows(sheet)
		r2, _ := f2.GetRows(sheet)
		if len(r1) != len(r2) {
			t.Fatalf("row count mismatch in %s: %d vs %d", sheet, len(r1), len(r2))
		}
		for i := range r1 {
			if join(r1[i]) != join(r2[i]) {
				t.Fatalf("determinism mismatch in %s row %d", sheet, i+1)
			}
		}
	}
}

func sampleFindings() []models.Finding {
	return []models.Finding{
		{
			ID:                "f-critical",
			Title:             "critical finding",
			Severity:          models.SeverityCritical,
			Module:            models.ModuleOrphanedEdge,
			Claimability:      models.ClaimReclaimable,
			AffectedARN:       "arn:aws:s3:::critical-bucket",
			AccountID:         "111111111111",
			AccountName:       "prod",
			OUPath:            "/root/prod",
			Team:              "edge",
			Hostname:          "critical.example.com",
			MonthlyDirectCost: 10.5,
			MonthlyRiskCost:   52.5,
			Impact:            "impact",
			Recommendation:    "recommend",
			RemediationCmd:    "aws route53 change-resource-record-sets ...",
			ScanID:            "scan-1",
			Evidence:          map[string]any{"key": "value"},
		},
		{
			ID:                "f-high",
			Title:             "high finding",
			Severity:          models.SeverityHigh,
			Module:            models.ModuleOrphanedEdge,
			Claimability:      models.ClaimDangling,
			AffectedARN:       "arn:aws:cloudfront::111111111111:distribution/EHIGH",
			AccountID:         "111111111111",
			AccountName:       "prod",
			OUPath:            "/root/prod",
			Team:              "edge",
			Hostname:          "high.example.com",
			MonthlyDirectCost: 20,
			MonthlyRiskCost:   60,
			Impact:            "impact",
			Recommendation:    "recommend",
			RemediationCmd:    "aws cloudfront update-distribution ...",
			ScanID:            "scan-1",
			Evidence:          map[string]any{"target_service": "cloudfront"},
		},
		{
			ID:                "f-medium",
			Title:             "medium finding",
			Severity:          models.SeverityMedium,
			Module:            models.ModuleOrphanedEdge,
			Claimability:      models.ClaimEdgeObscured,
			AffectedARN:       "arn:aws:route53:::hostedzone/ZMEDIUM",
			AccountID:         "222222222222",
			AccountName:       "staging",
			OUPath:            "/root/staging",
			Team:              "app",
			Hostname:          "medium.example.com",
			MonthlyDirectCost: 3,
			MonthlyRiskCost:   3,
			Impact:            "impact",
			Recommendation:    "recommend",
			RemediationCmd:    "aws route53 ...",
			ScanID:            "scan-1",
			Evidence:          map[string]any{"target_service": "route53"},
		},
		{
			ID:             "trust-1",
			Title:          "trust finding",
			Severity:       models.SeverityHigh,
			Module:         models.ModuleExternalAccess,
			Claimability:   models.ClaimUnknown,
			AffectedARN:    "arn:aws:iam::111111111111:role/TrustedRole",
			AccountID:      "111111111111",
			AccountName:    "prod",
			Recommendation: "review trust",
			ScanID:         "scan-1",
			Evidence: map[string]any{
				"role_name":            "TrustedRole",
				"external_principal":   "arn:aws:iam::222222222222:root",
				"principal_type":       "aws_account",
				"external_account_id":  "222222222222",
				"days_since_used":      366,
				"verdict":              "stale_review_now",
				"reason":               "never used or days_since_used > ghost threshold",
				"admin_eval_state":     "false",
				"unknown_vendor":       true,
				"activity_status":      "iam_never_used",
			},
		},
		{
			ID:             "trust-2",
			Title:          "trust finding missing ext account",
			Severity:       models.SeverityMedium,
			Module:         models.ModuleExternalAccess,
			Claimability:   models.ClaimUnknown,
			AffectedARN:    "arn:aws:iam::222222222222:role/OidcRole",
			AccountID:      "222222222222",
			AccountName:    "staging",
			Recommendation: "review trust",
			ScanID:         "scan-1",
			Evidence: map[string]any{
				"external_principal": "accounts.google.com",
				"principal_type":     "oidc",
				"days_since_used":    -1,
				"verdict":            "stale_review_now",
				"reason":             "missing activity join; treated conservatively as stale",
				"admin_eval_state":   "unknown",
				"unknown_vendor":     false,
				"activity_status":    "missing_join",
			},
		},
	}
}

func join(parts []string) string {
	return strconv.Quote(strings.Join(parts, "|"))
}
