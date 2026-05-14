package output

import (
	"bytes"
	"fmt"
	"text/tabwriter"

	"github.com/Zero0x00/cloudrift/internal/models"
)

func RenderTable(findings []models.Finding) string {
	buf := &bytes.Buffer{}
	w := tabwriter.NewWriter(buf, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "Hostname\tAccount\tService\tVerdict\tSeverity\tMonthly Waste")
	for _, f := range findings {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t$%.2f\n",
			f.Hostname,
			f.AccountID,
			serviceFromFinding(f),
			string(f.Claimability),
			string(f.Severity),
			f.MonthlyRiskCost,
		)
	}
	_ = w.Flush()
	return buf.String()
}

func serviceFromFinding(f models.Finding) string {
	if s, ok := f.Evidence["target_service"].(string); ok && s != "" {
		return s
	}
	return "unknown"
}
