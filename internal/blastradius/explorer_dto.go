package blastradius

import (
	"sort"
	"strings"

	"cloudrift/internal/api/schema"
)

// BuildExplorerPayload shapes a curated 3D-friendly graph with highlights.
func BuildExplorerPayload(
	s schema.BlastRadiusSummary,
	focusID string,
	mode BlastMode,
	findingID string,
	g *workingGraph,
) schema.BlastExplorerResponse {
	if g == nil {
		entityID := ""
		principalID := ""
		if s.RootType == schema.BlastRootExternalEntity {
			entityID = s.SourceEntityID
		}
		if s.RootType == schema.BlastRootPrincipal {
			principalID = s.SourcePrincipalID
		}
		return schema.BlastExplorerResponse{
			Focus: schema.BlastFocus{
				RootID:      s.RootID,
				RootType:    s.RootType,
				FindingID:   findingID,
				EntityID:    entityID,
				PrincipalID: principalID,
				Mode:        s.Mode,
				BlastMode:   string(mode),
			},
			Summary: s,
			Nodes:   nil,
			Edges:   nil,
			Display: schema.BlastDisplayHints{},
		}
	}
	cn, ce := markCritical(focusID, g, mode)
	ids := make([]string, 0, len(g.Nodes))
	for id := range g.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var nodes []schema.BlastGraphNode
	for _, id := range ids {
		if len(nodes) >= V1ExplorerNodeCap {
			break
		}
		n := g.Nodes[id]
		impact := nodeImpactScore(id, n, g, cn)
		st := firstStringProp(n.Props, "asset_type", "account_id", "title")
		isExternal := strings.EqualFold(firstStringProp(n.Props, "asset_type"), "external_principal")
		nodes = append(nodes, schema.BlastGraphNode{
			ID:              id,
			Label:           displayLabel(n),
			Type:            n.NType,
			Subtype:         st,
			AccountID:       firstStringProp(n.Props, "account_id"),
			SeverityOrTier:  firstStringProp(n.Props, "severity", "asset_type"),
			IsFocus:         id == focusID || (findingID != "" && id == "finding:"+findingID),
			IsCriticalPath:  cn[id],
			IsReachable:     true,
			IsExternal:      isExternal,
			ImpactScore:     impact,
			DisplayNameHint: st,
		})
	}
	eids := make([]string, 0, len(g.Edges))
	for eid := range g.Edges {
		eids = append(eids, eid)
	}
	sort.Strings(eids)
	var edges []schema.BlastGraphEdge
	for _, eid := range eids {
		if len(edges) >= V1ExplorerEdgeCap {
			break
		}
		e := g.Edges[eid]
		stype := semanticEdgeType(g, e)
		edges = append(edges, schema.BlastGraphEdge{
			ID:             e.ID,
			Source:         e.Src,
			Target:         e.Tgt,
			Type:           stype,
			Label:          stype,
			IsCriticalPath: ce[e.ID] || e.Type == "TRUSTS" || stype == "CROSS_ACCOUNT_ASSUME_ROLE" || stype == "EXTERNAL_TRUST",
			Explanation:    edgeExpl(stype),
		})
	}
	highlightNodes := make([]string, 0, 4)
	for id, on := range cn {
		if on {
			highlightNodes = append(highlightNodes, id)
		}
	}
	pathVariants, selectedPathID, highlightPathIDs := buildPathVariants(focusID, g, mode)
	return schema.BlastExplorerResponse{
		Focus: schema.BlastFocus{
			RootID:      s.RootID,
			RootType:    s.RootType,
			FindingID:   findingID,
			EntityID:    s.SourceEntityID,
			PrincipalID: s.SourcePrincipalID,
			Mode:        s.Mode,
			BlastMode:   string(mode),
		},
		Summary: s,
		Nodes:   nodes,
		Edges:   edges,
		// attack_path only: compact curated chain variants for fast operator comparison.
		PathVariants:   pathVariants,
		SelectedPathID: selectedPathID,
		Display: schema.BlastDisplayHints{
			DefaultFocusID:   focusID,
			HighlightNodeIDs: dedupeStr(highlightNodes, 20),
			HighlightEdgeIDs: highlightEdgeIDs(ce, g),
			HighlightPathIDs: highlightPathIDs,
		},
	}
}

func displayLabel(n rawNode) string {
	if t, _ := n.Props["name"].(string); strings.TrimSpace(t) != "" {
		return t
	}
	if t, _ := n.Props["arn"].(string); t != "" {
		return t
	}
	if t, _ := n.Props["id"].(string); t != "" {
		return t
	}
	if t, _ := n.Props["account_id"].(string); t != "" {
		return "account " + t
	}
	return n.ID
}

func firstStringProp(m map[string]any, keys ...string) string {
	for i := 0; i < len(keys); i++ {
		if v, ok := m[keys[i]].(string); ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func edgeExpl(t string) string {
	switch t {
	case "ASSUME_ROLE":
		return "IAM trust (assume-role or vendor trust) — treat as a privilege-pivot surface."
	case "CROSS_ACCOUNT_ASSUME_ROLE":
		return "Cross-account trust pivot between principals."
	case "EXTERNAL_TRUST":
		return "External principal trust path into internal roles."
	case "IAM_WRITE":
		return "IAM control-plane write/reconfiguration access path."
	case "ESCALATES_TO":
		return "Privilege transition to a higher-impact node."
	case "CAN_ACCESS":
		return "Permission-derived access path from principal to resource."
	case "OWNED_BY":
		return "Resource ownership in an AWS account."
	case "RESOURCE_ACCESS":
		return "Infra points-to relationship (e.g. DNS, routing)."
	case "CERT_LINK":
		return "Certificate or TLS association."
	default:
		return t
	}
}

func markCritical(focus string, g *workingGraph, mode BlastMode) (nodes map[string]bool, edges map[string]bool) {
	nodes = make(map[string]bool)
	edges = make(map[string]bool)
	if g == nil {
		return
	}
	// Focal
	if focus != "" {
		nodes[focus] = true
	}
	// All TRUSTS and their endpoints
	for id, e := range g.Edges {
		_ = id
		if e.Type == "TRUSTS" {
			edges[e.ID] = true
			nodes[e.Src] = true
			nodes[e.Tgt] = true
		}
	}
	// One hop from focus: highlight neighbor edges
	if focus != "" {
		for _, e := range g.Edges {
			if e.Src == focus || e.Tgt == focus {
				edges[e.ID] = true
				nodes[e.Src] = true
				nodes[e.Tgt] = true
			}
		}
	}
	if mode == ModeAttackPath && focus != "" {
		// Add simple directed chain emphasis from focus.
		cur := focus
		for i := 0; i < 6; i++ {
			nextEdge, ok := highestSignalOutgoing(g, cur)
			if !ok {
				break
			}
			edges[nextEdge.ID] = true
			nodes[nextEdge.Src] = true
			nodes[nextEdge.Tgt] = true
			cur = nextEdge.Tgt
		}
	}
	return nodes, edges
}

func highestSignalOutgoing(g *workingGraph, src string) (rawEdge, bool) {
	bestScore := -1
	best := rawEdge{}
	for _, e := range g.Edges {
		if e.Src != src {
			continue
		}
		score := 1
		switch semanticEdgeType(g, e) {
		case "EXTERNAL_TRUST":
			score = 9
		case "CROSS_ACCOUNT_ASSUME_ROLE":
			score = 8
		case "ASSUME_ROLE":
			score = 7
		case "IAM_WRITE":
			score = 6
		case "RESOURCE_ACCESS":
			score = 4
		case "CERT_LINK":
			score = 2
		}
		if score > bestScore {
			bestScore = score
			best = e
		}
	}
	return best, bestScore >= 0
}

func semanticEdgeType(g *workingGraph, e rawEdge) string {
	switch e.Type {
	case "TRUSTS":
		if isExternalTrustEdge(g, e) {
			return "EXTERNAL_TRUST"
		}
		if isCrossAccountEdge(g, e) {
			return "CROSS_ACCOUNT_ASSUME_ROLE"
		}
		return "ASSUME_ROLE"
	case "POINTS_TO", "FRONTS":
		src := g.Nodes[e.Src]
		dst := g.Nodes[e.Tgt]
		srcType, _ := src.Props["asset_type"].(string)
		dstType, _ := dst.Props["asset_type"].(string)
		if strings.EqualFold(srcType, "iam_role") && (strings.Contains(strings.ToLower(dstType), "iam") || strings.EqualFold(dstType, "iam_role")) {
			return "IAM_WRITE"
		}
		return "RESOURCE_ACCESS"
	case "USES_CERT":
		return "CERT_LINK"
	case "OWNED_BY":
		return "OWNED_BY"
	default:
		return e.Type
	}
}

func nodeImpactScore(id string, n rawNode, g *workingGraph, critical map[string]bool) float64 {
	score := 0.4
	if n.NType == "finding" {
		score = 1.0
	}
	if n.NType == "account" {
		score = 0.35
	}
	if t, _ := n.Props["asset_type"].(string); strings.EqualFold(t, "iam_role") {
		score += 0.35
	}
	if t, _ := n.Props["asset_type"].(string); strings.EqualFold(t, "external_principal") {
		score += 0.25
	}
	if strings.EqualFold(firstStringProp(n.Props, "classification"), "admin") || strings.EqualFold(firstStringProp(n.Props, "classification"), "privileged") {
		score += 0.2
	}
	if critical[id] {
		score += 0.25
	}
	if g != nil {
		for _, e := range g.Edges {
			if e.Src != id && e.Tgt != id {
				continue
			}
			switch e.Type {
			case "TRUSTS":
				score += 0.2
			case "POINTS_TO", "FRONTS":
				score += 0.08
			case "USES_CERT":
				score += 0.03
			}
		}
	}
	return score
}

func highlightEdgeIDs(ce map[string]bool, g *workingGraph) []string {
	var out []string
	for id, on := range ce {
		if on {
			out = append(out, id)
		}
	}
	_ = g
	return dedupeStr(out, 30)
}

func dedupeStr(in []string, cap int) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
		if len(out) >= cap {
			break
		}
	}
	return out
}

// BuildExplorerExpansionDelta builds an incremental node/edge payload for one-hop expansion.
func BuildExplorerExpansionDelta(
	expandedFromNodeID string,
	base *workingGraph,
	delta *workingGraph,
	mode BlastMode,
) schema.BlastExplorerExpansionResponse {
	if delta == nil || len(delta.Edges) == 0 {
		return schema.BlastExplorerExpansionResponse{
			ExpandedFromNodeID: expandedFromNodeID,
			ExpansionApplied:   false,
		}
	}
	// Keep only truly new nodes/edges relative to existing base context.
	newNodes := make(map[string]rawNode)
	for id, n := range delta.Nodes {
		if base == nil {
			newNodes[id] = n
			continue
		}
		if _, ok := base.Nodes[id]; !ok {
			newNodes[id] = n
		}
	}
	newEdges := make(map[string]rawEdge)
	for id, e := range delta.Edges {
		if base == nil {
			newEdges[id] = e
			continue
		}
		if _, ok := base.Edges[id]; !ok {
			newEdges[id] = e
		}
	}
	if len(newEdges) == 0 {
		return schema.BlastExplorerExpansionResponse{
			ExpandedFromNodeID: expandedFromNodeID,
			ExpansionApplied:   false,
		}
	}

	nodeIDs := make([]string, 0, len(newNodes))
	for id := range newNodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)
	nodes := make([]schema.BlastGraphNode, 0, len(nodeIDs))
	for _, id := range nodeIDs {
		n := newNodes[id]
		st := firstStringProp(n.Props, "asset_type", "account_id", "title")
		nodes = append(nodes, schema.BlastGraphNode{
			ID:              id,
			Label:           displayLabel(n),
			Type:            n.NType,
			Subtype:         st,
			AccountID:       firstStringProp(n.Props, "account_id"),
			SeverityOrTier:  firstStringProp(n.Props, "severity", "asset_type"),
			IsFocus:         id == expandedFromNodeID,
			IsCriticalPath:  false,
			IsReachable:     true,
			IsExternal:      strings.EqualFold(firstStringProp(n.Props, "asset_type"), "external_principal"),
			ImpactScore:     nodeImpactScore(id, n, delta, map[string]bool{}),
			DisplayNameHint: st,
		})
	}

	edgeIDs := make([]string, 0, len(newEdges))
	for id := range newEdges {
		edgeIDs = append(edgeIDs, id)
	}
	sort.Strings(edgeIDs)
	edges := make([]schema.BlastGraphEdge, 0, len(edgeIDs))
	for _, id := range edgeIDs {
		e := newEdges[id]
		sem := semanticEdgeType(delta, e)
		edges = append(edges, schema.BlastGraphEdge{
			ID:             e.ID,
			Source:         e.Src,
			Target:         e.Tgt,
			Type:           sem,
			Label:          sem,
			IsCriticalPath: mode == ModeAttackPath && (sem == "ASSUME_ROLE" || sem == "CROSS_ACCOUNT_ASSUME_ROLE" || sem == "EXTERNAL_TRUST"),
			Explanation:    edgeExpl(sem),
		})
	}

	newNodeIDs := make([]string, 0, len(nodes))
	for _, n := range nodes {
		newNodeIDs = append(newNodeIDs, n.ID)
	}
	newEdgeIDs := make([]string, 0, len(edges))
	for _, e := range edges {
		newEdgeIDs = append(newEdgeIDs, e.ID)
	}
	return schema.BlastExplorerExpansionResponse{
		ExpandedFromNodeID: expandedFromNodeID,
		ExpansionApplied:   true,
		Nodes:              nodes,
		Edges:              edges,
		Display: schema.BlastDisplayHints{
			HighlightNodeIDs: dedupeStr(newNodeIDs, 20),
			HighlightEdgeIDs: dedupeStr(newEdgeIDs, 20),
		},
	}
}
