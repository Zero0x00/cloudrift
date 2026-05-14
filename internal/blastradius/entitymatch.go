package blastradius

import (
	"fmt"
	"strings"

	"github.com/Zero0x00/cloudrift/internal/models"
)

const entSep = string('\x1e')

// roleARNFromFinding mirrors trustedRoleKey (handlers) for role-centric graph roots.
func roleARNFromFinding(f models.Finding) string {
	r := strings.TrimSpace(strEv(f.Evidence, "role_arn"))
	if r != "" {
		return r
	}
	r = strings.TrimSpace(f.AffectedARN)
	if r != "" {
		return r
	}
	return ""
}

func strEv(ev map[string]any, k string) string {
	if ev == nil {
		return ""
	}
	v, ok := ev[k]
	if !ok || v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func normPart(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	return s
}

// MatchExternalEntityFindings returns findings for external_access matching the same aggregate key
// as GET /api/scans/.../external-entities.
func MatchExternalEntityFindings(findings []models.Finding, ep, pt, ea string) []models.Finding {
	wantKey := normPart(ep) + entSep + strings.ToLower(normPart(pt)) + entSep + normPart(ea)
	var out []models.Finding
	for _, f := range findings {
		if !strings.EqualFold(string(f.Module), "external_access") {
			continue
		}
		got := normPart(strEv(f.Evidence, "external_principal")) + entSep +
			strings.ToLower(normPart(strEv(f.Evidence, "principal_type"))) + entSep +
			normPart(strEv(f.Evidence, "external_account_id"))
		if got == wantKey {
			out = append(out, f)
		}
	}
	return out
}

// RoleARNsFromFindings de-duplicates role ARNs for graph entry points.
func RoleARNsFromFindings(fs []models.Finding) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, f := range fs {
		rn := roleARNFromFinding(f)
		if rn == "" {
			continue
		}
		if _, ok := seen[rn]; ok {
			continue
		}
		seen[rn] = struct{}{}
		out = append(out, rn)
	}
	return out
}
