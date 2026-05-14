package blastradius

import (
	"strings"

	"github.com/Zero0x00/cloudrift/internal/models"
)

// PrivilegeSignals captures permission-visibility evidence reused by blast semantics.
type PrivilegeSignals struct {
	Classification   string
	CanAssumeRole    bool
	IAMWriteAccess   bool
	AdminLike        bool
	HasExternalTrust bool
	HasExternalRoot  bool
	Confidence       string
}

type graphMotifs struct {
	TrustEdges              int
	CrossAccountAssumeEdges int
	ExternalTrustEdges      int
	BroadResourceAccess     bool
}

func privilegeSignalsFromFinding(f *models.Finding) PrivilegeSignals {
	if f == nil {
		return PrivilegeSignals{}
	}
	s := PrivilegeSignals{
		HasExternalTrust: strings.TrimSpace(strEv(f.Evidence, "external_principal")) != "",
	}
	if pv, ok := f.Evidence["permission_visibility"].(map[string]any); ok && pv != nil {
		s.Classification = strings.ToLower(strings.TrimSpace(strEv(pv, "classification")))
		if c, ok2 := pv["capabilities"].(map[string]any); ok2 && c != nil {
			if b, ok3 := c["admin_like"].(bool); ok3 && b {
				s.AdminLike = true
			}
			if b, ok3 := c["can_assume_role"].(bool); ok3 && b {
				s.CanAssumeRole = true
			}
			if b, ok3 := c["iam_write_access"].(bool); ok3 && b {
				s.IAMWriteAccess = true
			}
		}
	}
	if b, ok := f.Evidence["admin_like"].(bool); ok && b {
		s.AdminLike = true
	}
	if s.AdminLike || s.IAMWriteAccess || s.CanAssumeRole || s.Classification != "" || s.HasExternalTrust {
		s.Confidence = "high"
	}
	return s
}

func combineSignals(in []PrivilegeSignals) PrivilegeSignals {
	out := PrivilegeSignals{}
	for _, s := range in {
		if out.Classification == "" || rankClassification(s.Classification) > rankClassification(out.Classification) {
			out.Classification = s.Classification
		}
		out.CanAssumeRole = out.CanAssumeRole || s.CanAssumeRole
		out.IAMWriteAccess = out.IAMWriteAccess || s.IAMWriteAccess
		out.AdminLike = out.AdminLike || s.AdminLike
		out.HasExternalTrust = out.HasExternalTrust || s.HasExternalTrust
		out.HasExternalRoot = out.HasExternalRoot || s.HasExternalRoot
		if confidenceRank(s.Confidence) > confidenceRank(out.Confidence) {
			out.Confidence = s.Confidence
		}
	}
	return out
}

func confidenceRank(c string) int {
	switch strings.ToLower(strings.TrimSpace(c)) {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func rankClassification(c string) int {
	switch strings.ToLower(strings.TrimSpace(c)) {
	case "admin":
		return 4
	case "privileged":
		return 3
	case "scoped":
		return 2
	case "limited":
		return 1
	default:
		return 0
	}
}

func detectGraphMotifs(g *workingGraph) graphMotifs {
	m := graphMotifs{}
	if g == nil {
		return m
	}
	m.BroadResourceAccess = g.countReachableResources() >= 12 || g.countAccountsTouched() >= 4
	for _, e := range g.Edges {
		if e.Type != "TRUSTS" {
			continue
		}
		m.TrustEdges++
		if isCrossAccountEdge(g, e) {
			m.CrossAccountAssumeEdges++
		}
		if isExternalTrustEdge(g, e) {
			m.ExternalTrustEdges++
		}
	}
	return m
}

func escalationPossibleFromSignals(g *workingGraph, signals PrivilegeSignals) bool {
	m := detectGraphMotifs(g)
	if signals.AdminLike || signals.IAMWriteAccess {
		return true
	}
	if rankClassification(signals.Classification) >= rankClassification("privileged") && m.TrustEdges > 0 {
		return true
	}
	if m.CrossAccountAssumeEdges > 0 {
		return true
	}
	if m.TrustEdges > 0 {
		return true
	}
	if m.ExternalTrustEdges > 0 && (signals.AdminLike || rankClassification(signals.Classification) >= rankClassification("privileged")) {
		return true
	}
	return signals.HasExternalTrust && m.TrustEdges > 0
}

func isCrossAccountEdge(g *workingGraph, e rawEdge) bool {
	src := accountForNode(g, e.Src)
	dst := accountForNode(g, e.Tgt)
	return src != "" && dst != "" && src != dst
}

func isExternalTrustEdge(g *workingGraph, e rawEdge) bool {
	if g == nil {
		return false
	}
	return isExternalNode(g.Nodes[e.Src]) || isExternalNode(g.Nodes[e.Tgt])
}

func isExternalNode(n rawNode) bool {
	if t, _ := n.Props["asset_type"].(string); strings.EqualFold(t, "external_principal") {
		return true
	}
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(n.ID)), "arn:aws:iam::") && strings.HasSuffix(strings.ToLower(strings.TrimSpace(n.ID)), ":root")
}

func accountForNode(g *workingGraph, id string) string {
	if strings.HasPrefix(id, "account:") {
		return strings.TrimPrefix(id, "account:")
	}
	if g != nil {
		if n, ok := g.Nodes[id]; ok {
			if a, _ := n.Props["account_id"].(string); strings.TrimSpace(a) != "" {
				return strings.TrimSpace(a)
			}
		}
	}
	// Best-effort parse from IAM-style ARN.
	parts := strings.Split(id, ":")
	if len(parts) >= 6 && strings.EqualFold(parts[0], "arn") && strings.EqualFold(parts[2], "iam") {
		return strings.TrimSpace(parts[4])
	}
	return ""
}

// privilegeSignalsForPrincipalRoot enriches a principal-root with evidence from scan findings.
// Matching precedence:
//   - direct affected ARN match => high confidence
//   - evidence.role_arn match => medium confidence
func privilegeSignalsForPrincipalRoot(findings []models.Finding, principalARN string) PrivilegeSignals {
	arn := strings.TrimSpace(principalARN)
	if arn == "" {
		return PrivilegeSignals{Confidence: "none"}
	}
	direct := make([]PrivilegeSignals, 0, 4)
	indirect := make([]PrivilegeSignals, 0, 4)
	for i := range findings {
		f := &findings[i]
		fSig := privilegeSignalsFromFinding(f)
		if strings.EqualFold(strings.TrimSpace(f.AffectedARN), arn) {
			fSig.Confidence = "high"
			direct = append(direct, fSig)
			continue
		}
		if strings.EqualFold(strings.TrimSpace(strEv(f.Evidence, "role_arn")), arn) {
			if confidenceRank(fSig.Confidence) < confidenceRank("medium") {
				fSig.Confidence = "medium"
			}
			indirect = append(indirect, fSig)
		}
	}
	if len(direct) > 0 {
		return combineSignals(direct)
	}
	if len(indirect) > 0 {
		return combineSignals(indirect)
	}
	return PrivilegeSignals{Confidence: "none"}
}
