package blastradius

import (
	"sort"
	"strconv"
	"strings"

	"cloudrift/internal/api/schema"
)

// BuildSummaryPayload turns a working graph + root metadata into the API DTO.
func BuildSummaryPayload(
	rootKind schema.BlastRootKind,
	rootID string,
	scanID string,
	mode BlastMode,
	g *workingGraph,
	focusARN string,
	focusFindingID string,
	signals PrivilegeSignals,
	graphAvailable bool,
	reason UnavailableReason,
) schema.BlastRadiusSummary {
	esc := escalationPossibleFromSignals(g, signals)
	accC := 0
	resC := 0
	if g != nil {
		resC = g.countReachableResources()
		accC = g.countAccountsTouched()
	}
	if !graphAvailable {
		accC, resC = 0, 0
	}
	summary := schema.BlastRadiusSummary{
		RootType:               rootKind,
		RootID:                 rootID,
		ScanID:                 scanID,
		Mode:                   string(mode),
		ReachableResourceCount: resC,
		ReachableAccountsCount: accC,
		TopResourceTypes:       []string{},
		TopImpactedAccounts:    []string{},
		TopImpactedResources:   []string{},
		EscalationPossible:     esc,
		SummaryText:            "",
		RecommendedActionLabel: defaultActionLabel(esc, graphAvailable),
		GraphAvailable:         graphAvailable,
		GraphUnavailableReason: string(reason),
	}
	if g != nil && graphAvailable {
		summary.TopResourceTypes = g.topAssetTypes(6)
		summary.TopImpactedAccounts = topImpactedAccounts(g, 6)
		summary.TopImpactedResources = topImpactedARNS(g, focusARN, 8, signals)
		summary.RecommendedActionLabel = actionLabelForGraph(g, signals, esc)
	}
	summary.SummaryText = buildSummaryNarrative(&summary, focusARN, focusFindingID, graphAvailable, reason, signals)
	return summary
}

func defaultActionLabel(esc, graphOK bool) string {
	if !graphOK {
		return "Connect Neo4j and export a scan to enable graph reachability"
	}
	if esc {
		return "Review cross-account trust and privilege pivots in scope"
	}
	return "Harden the focal resource and re-scan to validate reachability shrink"
}

func topImpactedARNS(g *workingGraph, focus string, n int, signals PrivilegeSignals) []string {
	type scored struct {
		arn  string
		risk float64
	}
	var ss []scored
	for id, node := range g.Nodes {
		if node.NType != "asset" {
			_ = id
			continue
		}
		if id == focus {
			continue
		}
		rk := 1.0
		t, _ := node.Props["asset_type"].(string)
		if strings.EqualFold(t, "iam_role") {
			rk += 2.0
		}
		if strings.EqualFold(t, "external_principal") {
			rk += 1.5
		}
		rk += edgeWeightedRisk(g, id)
		if signals.AdminLike {
			rk += 0.4
		}
		if signals.IAMWriteAccess {
			rk += 0.6
		}
		if rankClassification(signals.Classification) >= rankClassification("privileged") {
			rk += 0.5
		}
		ss = append(ss, scored{arn: id, risk: rk})
	}
	sort.Slice(ss, func(i, j int) bool {
		if ss[i].risk != ss[j].risk {
			return ss[i].risk > ss[j].risk
		}
		return ss[i].arn < ss[j].arn
	})
	out := make([]string, 0, n)
	for i := 0; i < len(ss) && len(out) < n; i++ {
		out = append(out, ss[i].arn)
	}
	return out
}

func topImpactedAccounts(g *workingGraph, n int) []string {
	type scored struct {
		account string
		score   float64
	}
	if g == nil {
		return nil
	}
	accScore := map[string]float64{}
	for id, node := range g.Nodes {
		acct := accountForNode(g, id)
		if acct == "" {
			continue
		}
		s := 0.7
		if node.NType == "account" {
			s += 0.6
		}
		if t, _ := node.Props["asset_type"].(string); strings.EqualFold(t, "iam_role") {
			s += 0.8
		}
		accScore[acct] += s + edgeWeightedRisk(g, id)*0.4
	}
	var ss []scored
	for a, s := range accScore {
		ss = append(ss, scored{account: a, score: s})
	}
	sort.Slice(ss, func(i, j int) bool {
		if ss[i].score != ss[j].score {
			return ss[i].score > ss[j].score
		}
		return ss[i].account < ss[j].account
	})
	out := make([]string, 0, n)
	for i := 0; i < len(ss) && len(out) < n; i++ {
		out = append(out, ss[i].account)
	}
	return out
}

func edgeWeightedRisk(g *workingGraph, nodeID string) float64 {
	if g == nil {
		return 0
	}
	weight := 0.0
	for _, e := range g.Edges {
		if e.Src != nodeID && e.Tgt != nodeID {
			continue
		}
		switch e.Type {
		case "TRUSTS":
			weight += 2.5
		case "POINTS_TO", "FRONTS":
			weight += 1.2
		case "USES_CERT":
			weight += 0.6
		case "OWNED_BY":
			weight += 0.3
		}
	}
	return weight
}

func actionLabelForGraph(g *workingGraph, signals PrivilegeSignals, esc bool) string {
	if g == nil {
		return defaultActionLabel(esc, false)
	}
	m := detectGraphMotifs(g)
	adminLike := signals.AdminLike || rankClassification(signals.Classification) >= rankClassification("privileged")
	switch {
	case m.ExternalTrustEdges > 0 && m.CrossAccountAssumeEdges > 0 && adminLike:
		return "Constrain external trust on privileged roles and require explicit owner re-approval"
	case m.CrossAccountAssumeEdges > 0:
		return "Tighten cross-account assume-role chains and scope pivot-role permissions"
	case signals.IAMWriteAccess && adminLike:
		return "Constrain IAM-write/admin-like pivot principals before reducing broader access"
	case m.ExternalTrustEdges > 0:
		return "Review external trust chains and limit cross-account assume-role paths"
	case m.TrustEdges > 0:
		return "Review trust pivots and verify least-privilege role boundaries"
	case m.BroadResourceAccess:
		return "Reduce broad resource-access spread from this root to shrink blast radius"
	default:
		return defaultActionLabel(esc, true)
	}
}

func buildSummaryNarrative(
	s *schema.BlastRadiusSummary,
	focusARN, findingID string,
	graphOK bool,
	reason UnavailableReason,
	signals PrivilegeSignals,
) string {
	if !graphOK {
		base := fmtUnavailableSummary(reason) + " " + s.RecommendedActionLabel + "."
		if s.RootType == schema.BlastRootPrincipal {
			conf := strings.ToLower(strings.TrimSpace(signals.Confidence))
			if conf == "" {
				conf = "none"
			}
			return base + " Privilege-evidence confidence: " + conf + "."
		}
		return base
	}
	var b strings.Builder
	if s.RootType == schema.BlastRootPrincipal {
		b.WriteString("Principal blast radius for ")
		b.WriteString(s.RootID)
		b.WriteString(" in scan ")
		b.WriteString(s.ScanID)
		b.WriteString(". ")
	} else {
		b.WriteString("Blast-radius view for ")
		b.WriteString(string(s.RootType))
		b.WriteString(" ")
		b.WriteString(s.RootID)
		b.WriteString(" in scan ")
		b.WriteString(s.ScanID)
		b.WriteString(". ")
	}
	if s.EscalationPossible {
		b.WriteString("High-signal pivot motifs (trust, cross-account assume-role, or privilege-heavy principals) indicate realistic lateral movement in this bounded path set. ")
	} else {
		b.WriteString("No strong trust/privilege pivot motifs were observed in this bounded neighborhood; impact remains limited to shown relationships. ")
	}
	b.WriteString("Reachability highlights ")
	b.WriteString(itoaU(s.ReachableResourceCount))
	b.WriteString(" resource node(s) across ")
	b.WriteString(itoaU(s.ReachableAccountsCount))
	b.WriteString(" account(s).")
	if s.RootType == schema.BlastRootPrincipal {
		b.WriteString(" This principal can reach the nodes shown in this scoped view.")
	}
	if signals.IAMWriteAccess || signals.AdminLike || rankClassification(signals.Classification) >= rankClassification("privileged") {
		b.WriteString(" Permission visibility marks this root as privilege-heavy")
		if signals.IAMWriteAccess {
			b.WriteString(" (IAM write)")
		}
		if signals.AdminLike {
			b.WriteString(" (admin-like)")
		}
		b.WriteString(".")
	}
	if s.RootType == schema.BlastRootPrincipal {
		conf := strings.ToLower(strings.TrimSpace(signals.Confidence))
		if conf == "" {
			conf = "none"
		}
		b.WriteString(" Privilege-evidence confidence: ")
		b.WriteString(conf)
		b.WriteString(".")
	}
	if focusARN != "" {
		b.WriteString(" Focal resource: ")
		b.WriteString(focusARN)
		b.WriteString(".")
	}
	if findingID != "" {
		b.WriteString(" Finding: ")
		b.WriteString(findingID)
		b.WriteString(".")
	}
	return b.String()
}

func itoaU(n int) string {
	if n < 0 {
		return "0"
	}
	return strconv.Itoa(n)
}
