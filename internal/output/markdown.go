package output

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cloudrift/internal/models"
)

func WriteMarkdown(path string, findings []models.Finding) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# Cloudrift Findings\n\n")
	for _, f := range findings {
		b.WriteString(fmt.Sprintf("## %s\n- Severity: %s\n- Module: %s\n- Hostname: `%s`\n- Account: `%s`\n- Claimability: `%s`\n- Direct cost: `$%.2f`\n- Risk-adjusted cost: `$%.2f`\n- Impact: %s\n- Recommendation: %s\n\n",
			f.Title, f.Severity, f.Module, f.Hostname, f.AccountID, f.Claimability, f.MonthlyDirectCost, f.MonthlyRiskCost, f.Impact, f.Recommendation))
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
