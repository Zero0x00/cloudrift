package remediator

import (
	"encoding/json"
	"fmt"
	"strings"

	"cloudrift/internal/models"
)

func BuildRemediation(f models.Finding) (string, string, []byte) {
	cmd := remediationCommandFor(f)
	md := fmt.Sprintf("## %s\n\n- Severity: %s\n- Account: %s\n- Hostname: %s\n- Claimability: %s\n\n### Evidence\n- ARN: `%s`\n- Fingerprint: `%v`\n\n### Impact\n%s\n\n### Recommendation\n%s\n\n### Command\n`%s`\n",
		f.Title, f.Severity, f.AccountID, f.Hostname, f.Claimability, f.AffectedARN, f.Evidence["fingerprint"], f.Impact, f.Recommendation, cmd)

	payload := map[string]any{
		"id":             f.ID,
		"title":          f.Title,
		"severity":       f.Severity,
		"account_id":     f.AccountID,
		"hostname":       f.Hostname,
		"claimability":   f.Claimability,
		"affected_arn":   f.AffectedARN,
		"recommendation": f.Recommendation,
		"impact":         f.Impact,
		"command":        cmd,
	}
	b, _ := json.MarshalIndent(payload, "", "  ")
	return cmd, md, b
}

func remediationCommandFor(f models.Finding) string {
	switch {
	case strings.Contains(strings.ToLower(f.AffectedARN), "route53"):
		return fmt.Sprintf("aws route53 change-resource-record-sets --hosted-zone-id <zone-id> --change-batch file://delete-%s.json", f.ID)
	case strings.Contains(strings.ToLower(f.AffectedARN), "cloudfront"):
		return "aws cloudfront update-distribution --id <distribution-id> --if-match <etag> --distribution-config file://patched-config.json"
	default:
		return "aws resourcegroupstaggingapi get-resources --tag-filters Key=Owner,Values=<team> # TODO: choose service-specific remediation"
	}
}
