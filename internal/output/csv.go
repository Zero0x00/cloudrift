package output

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Zero0x00/cloudrift/internal/models"
)

func WriteCSV(path string, findings []models.Finding) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"id", "severity", "module", "hostname", "account_id", "claimability", "monthly_direct_cost_usd", "monthly_risk_cost_usd"})
	for _, item := range findings {
		_ = w.Write([]string{
			sanitizeCSVCell(item.ID),
			sanitizeCSVCell(string(item.Severity)),
			sanitizeCSVCell(string(item.Module)),
			sanitizeCSVCell(item.Hostname),
			sanitizeCSVCell(item.AccountID),
			sanitizeCSVCell(string(item.Claimability)),
			strconv.FormatFloat(item.MonthlyDirectCost, 'f', 2, 64),
			strconv.FormatFloat(item.MonthlyRiskCost, 'f', 2, 64),
		})
	}
	return w.Error()
}

func sanitizeCSVCell(s string) string {
	if s == "" {
		return s
	}
	trimmed := strings.TrimLeft(s, " \t")
	if strings.HasPrefix(trimmed, "=") || strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "@") {
		return "'" + s
	}
	return s
}
