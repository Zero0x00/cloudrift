package output

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"cloudrift/internal/models"

	"github.com/xuri/excelize/v2"
)

const (
	sheetFindings    = "Findings"
	sheetCostSummary = "Cost Summary"
	sheetTrustReport = "Trust Report"
)

var findingsColumns = []string{
	"ID", "Title", "Severity", "Module", "Claimability", "AffectedARN", "AccountID",
	"AccountName", "OUPath", "Team", "Hostname", "MonthlyDirectCost", "MonthlyRiskCost",
	"Impact", "Recommendation", "RemediationCmd", "ScanID", "EvidenceJSON",
}

var costSummaryColumns = []string{
	"AccountID", "AccountName", "Module", "FindingCount", "TotalMonthlyDirectCost", "TotalMonthlyRiskCost",
}

var trustColumns = []string{
	"RoleARN", "RoleName", "ExternalPrincipal", "PrincipalType", "ExternalAccountID",
	"Severity", "DaysSinceUsed", "Verdict", "Reason", "AdminEvalState", "UnknownVendor",
	"Recommendation", "AccountID", "AccountName", "ScanID",
}

func WriteExcel(path string, findings []models.Finding) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	f := excelize.NewFile()
	defaultSheet := f.GetSheetName(0)
	if defaultSheet != "" && defaultSheet != sheetFindings {
		_ = f.SetSheetName(defaultSheet, sheetFindings)
	}

	findingsCopy := append([]models.Finding(nil), findings...)
	sortFindings(findingsCopy)

	headerStyle, severityStyles, err := buildStyles(f)
	if err != nil {
		return err
	}

	if err := writeFindingsSheet(f, findingsCopy, headerStyle, severityStyles); err != nil {
		return err
	}
	if err := writeCostSummarySheet(f, findingsCopy, headerStyle); err != nil {
		return err
	}
	if err := writeTrustReportSheet(f, findingsCopy, headerStyle); err != nil {
		return err
	}

	f.SetActiveSheet(0)
	return f.SaveAs(path)
}

func writeFindingsSheet(f *excelize.File, findings []models.Finding, headerStyle int, severityStyles map[models.Severity]int) error {
	idx, err := f.GetSheetIndex(sheetFindings)
	if err != nil || idx == -1 {
		if _, err := f.NewSheet(sheetFindings); err != nil {
			return err
		}
	}
	if err := writeHeaders(f, sheetFindings, findingsColumns, headerStyle); err != nil {
		return err
	}

	for i, row := range findings {
		r := i + 2
		values := []any{
			row.ID, row.Title, string(row.Severity), string(row.Module), string(row.Claimability),
			row.AffectedARN, row.AccountID, row.AccountName, row.OUPath, row.Team, row.Hostname,
			row.MonthlyDirectCost, row.MonthlyRiskCost, row.Impact, row.Recommendation, row.RemediationCmd,
			row.ScanID, compactJSON(row.Evidence),
		}
		if err := writeRow(f, sheetFindings, r, values); err != nil {
			return err
		}
		if styleID, ok := severityStyles[row.Severity]; ok {
			cell, _ := excelize.CoordinatesToCellName(3, r) // Severity col
			if err := f.SetCellStyle(sheetFindings, cell, cell, styleID); err != nil {
				return err
			}
		}
	}

	setColumnWidths(f, sheetFindings, []float64{
		14, 28, 12, 16, 14, 46, 14, 20, 24, 18, 24, 18, 18, 30, 34, 34, 20, 42,
	})
	return nil
}

func writeCostSummarySheet(f *excelize.File, findings []models.Finding, headerStyle int) error {
	_, err := f.NewSheet(sheetCostSummary)
	if err != nil {
		return err
	}
	if err := writeHeaders(f, sheetCostSummary, costSummaryColumns, headerStyle); err != nil {
		return err
	}

	type key struct {
		AccountID   string
		AccountName string
		Module      string
	}
	type agg struct {
		Count int
		Direct float64
		Risk   float64
	}
	rollup := make(map[key]agg)
	for _, row := range findings {
		k := key{AccountID: row.AccountID, AccountName: row.AccountName, Module: string(row.Module)}
		a := rollup[k]
		a.Count++
		a.Direct += row.MonthlyDirectCost
		a.Risk += row.MonthlyRiskCost
		rollup[k] = a
	}

	keys := make([]key, 0, len(rollup))
	for k := range rollup {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].AccountID == keys[j].AccountID {
			if keys[i].AccountName == keys[j].AccountName {
				return keys[i].Module < keys[j].Module
			}
			return keys[i].AccountName < keys[j].AccountName
		}
		return keys[i].AccountID < keys[j].AccountID
	})

	rowNum := 2
	var totalCount int
	var totalDirect, totalRisk float64
	for _, k := range keys {
		a := rollup[k]
		values := []any{k.AccountID, k.AccountName, k.Module, a.Count, a.Direct, a.Risk}
		if err := writeRow(f, sheetCostSummary, rowNum, values); err != nil {
			return err
		}
		totalCount += a.Count
		totalDirect += a.Direct
		totalRisk += a.Risk
		rowNum++
	}

	totalValues := []any{"TOTAL", "", "", totalCount, totalDirect, totalRisk}
	if err := writeRow(f, sheetCostSummary, rowNum, totalValues); err != nil {
		return err
	}
	_ = f.SetCellStyle(sheetCostSummary, "A"+strconv.Itoa(rowNum), "F"+strconv.Itoa(rowNum), headerStyle)
	setColumnWidths(f, sheetCostSummary, []float64{14, 20, 20, 14, 24, 22})
	return nil
}

func writeTrustReportSheet(f *excelize.File, findings []models.Finding, headerStyle int) error {
	_, err := f.NewSheet(sheetTrustReport)
	if err != nil {
		return err
	}
	if err := writeHeaders(f, sheetTrustReport, trustColumns, headerStyle); err != nil {
		return err
	}

	var trustFindings []models.Finding
	for _, item := range findings {
		if item.Module == models.ModuleExternalAccess {
			trustFindings = append(trustFindings, item)
		}
	}
	sortFindings(trustFindings)

	for i, row := range trustFindings {
		r := i + 2
		roleARN := row.AffectedARN
		roleName := pickString(row.Evidence, "role_name")
		if roleName == "" {
			roleName = lastARNSegment(roleARN)
		}
		values := []any{
			roleARN,
			roleName,
			pickString(row.Evidence, "external_principal"),
			pickString(row.Evidence, "principal_type"),
			pickString(row.Evidence, "external_account_id"),
			string(row.Severity),
			pickInt(row.Evidence, "days_since_used", -1),
			pickString(row.Evidence, "verdict"),
			pickString(row.Evidence, "reason"),
			pickString(row.Evidence, "admin_eval_state"),
			pickBool(row.Evidence, "unknown_vendor"),
			row.Recommendation,
			row.AccountID,
			row.AccountName,
			row.ScanID,
		}
		if err := writeRow(f, sheetTrustReport, r, values); err != nil {
			return err
		}
	}
	setColumnWidths(f, sheetTrustReport, []float64{44, 22, 30, 16, 18, 12, 14, 22, 34, 16, 14, 34, 14, 20, 18})
	return nil
}

func buildStyles(f *excelize.File) (int, map[models.Severity]int, error) {
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"2F5597"}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	if err != nil {
		return 0, nil, err
	}
	criticalStyle, err := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFC7CE"}},
		Font: &excelize.Font{Color: "9C0006"},
	})
	if err != nil {
		return 0, nil, err
	}
	highStyle, err := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FCE4D6"}},
		Font: &excelize.Font{Color: "A64D00"},
	})
	if err != nil {
		return 0, nil, err
	}
	mediumStyle, err := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFF2CC"}},
		Font: &excelize.Font{Color: "7F6000"},
	})
	if err != nil {
		return 0, nil, err
	}
	styles := map[models.Severity]int{
		models.SeverityCritical: criticalStyle,
		models.SeverityHigh:     highStyle,
		models.SeverityMedium:   mediumStyle,
	}
	return headerStyle, styles, nil
}

func writeHeaders(f *excelize.File, sheet string, columns []string, styleID int) error {
	for i, name := range columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, name); err != nil {
			return err
		}
	}
	end, _ := excelize.CoordinatesToCellName(len(columns), 1)
	return f.SetCellStyle(sheet, "A1", end, styleID)
}

func writeRow(f *excelize.File, sheet string, row int, values []any) error {
	for i, v := range values {
		cell, _ := excelize.CoordinatesToCellName(i+1, row)
		if err := f.SetCellValue(sheet, cell, v); err != nil {
			return err
		}
	}
	return nil
}

func setColumnWidths(f *excelize.File, sheet string, widths []float64) {
	for i, w := range widths {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetColWidth(sheet, col, col, w)
	}
}

func sortFindings(rows []models.Finding) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].AffectedARN == rows[j].AffectedARN {
			if rows[i].ID == rows[j].ID {
				return rows[i].Title < rows[j].Title
			}
			return rows[i].ID < rows[j].ID
		}
		return rows[i].AffectedARN < rows[j].AffectedARN
	})
}

func compactJSON(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func pickString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

func pickInt(m map[string]any, key string, fallback int) int {
	if m == nil {
		return fallback
	}
	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return fallback
	}
}

func pickBool(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	v, _ := m[key].(bool)
	return v
}

func lastARNSegment(arn string) string {
	if idx := strings.LastIndex(arn, "/"); idx >= 0 && idx+1 < len(arn) {
		return arn[idx+1:]
	}
	return arn
}
