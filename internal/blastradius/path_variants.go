package blastradius

import (
	"sort"
	"strings"

	"github.com/Zero0x00/cloudrift/internal/api/schema"
)

const (
	maxPathDepth      = 6
	maxRawCandidates  = 24
	maxVariantCount   = 3 // primary + up to 2 alternates
	primaryVariantID  = "primary"
	alternateIDPrefix = "alternate-"
)

type pathCandidate struct {
	nodeIDs   []string
	edgeIDs   []string
	semantics []string
	score     int
}

func buildPathVariants(focusID string, g *workingGraph, mode BlastMode) ([]schema.BlastPathVariant, string, []string) {
	if g == nil || mode != ModeAttackPath || strings.TrimSpace(focusID) == "" {
		return nil, "", nil
	}
	cands := collectPathCandidates(focusID, g)
	if len(cands) == 0 {
		return nil, "", nil
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score > cands[j].score
		}
		if len(cands[i].edgeIDs) != len(cands[j].edgeIDs) {
			return len(cands[i].edgeIDs) < len(cands[j].edgeIDs)
		}
		return strings.Join(cands[i].edgeIDs, ",") < strings.Join(cands[j].edgeIDs, ",")
	})

	selected := make([]pathCandidate, 0, maxVariantCount)
	seenPath := map[string]struct{}{}
	for _, c := range cands {
		key := strings.Join(c.edgeIDs, "|")
		if key == "" {
			continue
		}
		if _, ok := seenPath[key]; ok {
			continue
		}
		seenPath[key] = struct{}{}
		if len(selected) == 0 {
			selected = append(selected, c)
			if len(selected) >= maxVariantCount {
				break
			}
			continue
		}
		// Distinctness: different first pivot edge/node OR materially different semantic chain.
		if !isDistinctPath(selected[0], c) {
			continue
		}
		selected = append(selected, c)
		if len(selected) >= maxVariantCount {
			break
		}
	}
	if len(selected) == 0 {
		return nil, "", nil
	}

	out := make([]schema.BlastPathVariant, 0, len(selected))
	pathIDs := make([]string, 0, len(selected))
	for i, c := range selected {
		id := primaryVariantID
		kind := "primary"
		if i > 0 {
			id = alternateIDPrefix + itoaU(i)
			kind = "alternate"
		}
		out = append(out, schema.BlastPathVariant{
			ID:                id,
			Label:             pathLabel(kind, c.semantics),
			Kind:              kind,
			Summary:           pathSummary(c),
			NodeIDs:           c.nodeIDs,
			EdgeIDs:           c.edgeIDs,
			DominantSemantics: topSemantics(c.semantics, 3),
			RiskHint:          pathRiskHint(c.semantics),
		})
		pathIDs = append(pathIDs, id)
	}
	return out, primaryVariantID, pathIDs
}

func collectPathCandidates(focusID string, g *workingGraph) []pathCandidate {
	out := make([]pathCandidate, 0, maxRawCandidates)
	var walk func(cur string, depth int, visited map[string]struct{}, nodes []string, edges []string, semantics []string, score int)
	walk = func(cur string, depth int, visited map[string]struct{}, nodes []string, edges []string, semantics []string, score int) {
		if len(out) >= maxRawCandidates || depth >= maxPathDepth {
			return
		}
		next := outgoingEdges(g, cur)
		if len(next) == 0 {
			if len(edges) > 0 {
				out = append(out, pathCandidate{
					nodeIDs:   append([]string(nil), nodes...),
					edgeIDs:   append([]string(nil), edges...),
					semantics: append([]string(nil), semantics...),
					score:     score,
				})
			}
			return
		}
		advanced := false
		for _, e := range next {
			if _, seen := visited[e.Tgt]; seen {
				continue
			}
			advanced = true
			sem := semanticEdgeType(g, e)
			ns := append(nodes, e.Tgt)
			es := append(edges, e.ID)
			ss := append(semantics, sem)
			nextVisited := cloneVisited(visited)
			nextVisited[e.Tgt] = struct{}{}
			walk(e.Tgt, depth+1, nextVisited, ns, es, ss, score+edgeSignalScore(sem)+targetNodeScore(g, e.Tgt))
			if len(out) >= maxRawCandidates {
				return
			}
		}
		if !advanced && len(edges) > 0 {
			out = append(out, pathCandidate{
				nodeIDs:   append([]string(nil), nodes...),
				edgeIDs:   append([]string(nil), edges...),
				semantics: append([]string(nil), semantics...),
				score:     score,
			})
		}
	}

	vis := map[string]struct{}{focusID: {}}
	walk(focusID, 0, vis, []string{focusID}, nil, nil, 0)
	return out
}

func outgoingEdges(g *workingGraph, src string) []rawEdge {
	out := make([]rawEdge, 0, 6)
	for _, e := range g.Edges {
		if e.Src == src {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		si := edgeSignalScore(semanticEdgeType(g, out[i]))
		sj := edgeSignalScore(semanticEdgeType(g, out[j]))
		if si != sj {
			return si > sj
		}
		if out[i].Tgt != out[j].Tgt {
			return out[i].Tgt < out[j].Tgt
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func edgeSignalScore(sem string) int {
	switch sem {
	case "CROSS_ACCOUNT_ASSUME_ROLE":
		return 11
	case "EXTERNAL_TRUST":
		return 10
	case "ASSUME_ROLE":
		return 8
	case "IAM_WRITE":
		return 7
	case "RESOURCE_ACCESS":
		return 4
	case "CERT_LINK":
		return 2
	case "OWNED_BY":
		return 1
	default:
		return 1
	}
}

func targetNodeScore(g *workingGraph, nodeID string) int {
	n, ok := g.Nodes[nodeID]
	if !ok {
		return 0
	}
	score := 0
	if n.NType == "account" {
		score += 2
	}
	if strings.EqualFold(firstStringProp(n.Props, "asset_type"), "iam_role") {
		score += 3
	}
	if strings.EqualFold(firstStringProp(n.Props, "classification"), "admin") {
		score += 3
	} else if strings.EqualFold(firstStringProp(n.Props, "classification"), "privileged") {
		score += 2
	}
	return score
}

func isDistinctPath(primary pathCandidate, alt pathCandidate) bool {
	if len(primary.edgeIDs) == 0 || len(alt.edgeIDs) == 0 {
		return false
	}
	// Different first pivot edge implies distinct path branch.
	if primary.edgeIDs[0] != alt.edgeIDs[0] {
		return true
	}
	// Different first pivot node implies distinct path branch.
	if len(primary.nodeIDs) > 1 && len(alt.nodeIDs) > 1 && primary.nodeIDs[1] != alt.nodeIDs[1] {
		return true
	}
	// Materially different semantic chain.
	return strings.Join(primary.semantics, "|") != strings.Join(alt.semantics, "|")
}

func pathLabel(kind string, semantics []string) string {
	if kind == "primary" {
		return "Primary path"
	}
	dom := topSemantics(semantics, 1)
	if len(dom) == 0 {
		return "Alternate path"
	}
	switch dom[0] {
	case "CROSS_ACCOUNT_ASSUME_ROLE":
		return "Alternate: cross-account role pivot"
	case "EXTERNAL_TRUST":
		return "Alternate: external trust branch"
	case "IAM_WRITE":
		return "Alternate: IAM-write chain"
	case "ASSUME_ROLE":
		return "Alternate: assume-role branch"
	default:
		return "Alternate path"
	}
}

func pathSummary(c pathCandidate) string {
	if len(c.edgeIDs) == 0 {
		return "No chain."
	}
	return "Chain length " + itoaU(len(c.edgeIDs)) + " with " + strings.Join(topSemantics(c.semantics, 2), " + ")
}

func pathRiskHint(semantics []string) string {
	for _, sem := range topSemantics(semantics, 3) {
		switch sem {
		case "CROSS_ACCOUNT_ASSUME_ROLE":
			return "Reduce cross-account role trust on pivot roles."
		case "EXTERNAL_TRUST":
			return "Constrain external trust into privileged internal roles."
		case "IAM_WRITE":
			return "Constrain IAM-write capability on pivot principals."
		case "ASSUME_ROLE":
			return "Tighten assume-role trust boundaries and conditions."
		}
	}
	return ""
}

func topSemantics(in []string, limit int) []string {
	counts := map[string]int{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			counts[s]++
		}
	}
	type kv struct {
		k string
		v int
	}
	arr := make([]kv, 0, len(counts))
	for k, v := range counts {
		arr = append(arr, kv{k: k, v: v})
	}
	sort.Slice(arr, func(i, j int) bool {
		if arr[i].v != arr[j].v {
			return arr[i].v > arr[j].v
		}
		return edgeSignalScore(arr[i].k) > edgeSignalScore(arr[j].k)
	})
	out := make([]string, 0, limit)
	for _, item := range arr {
		out = append(out, item.k)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func cloneVisited(in map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(in)+1)
	for k := range in {
		out[k] = struct{}{}
	}
	return out
}
