package blastradius

import (
	"context"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"github.com/Zero0x00/cloudrift/internal/api/schema"
	"github.com/Zero0x00/cloudrift/internal/models"
	"github.com/Zero0x00/cloudrift/internal/scans"
)

type ExpansionRootContext struct {
	RootType schema.BlastRootKind
	RootID   string
}

// Service wires optional Neo4j read access to scan JSON. When Driver is nil, APIs still return
// structured, explainable fallbacks.
type Service struct {
	Driver neo4j.DriverWithContext
	DBName string
	OutDir string
}

// NewService returns a Service; driver may be nil.
func NewService(driver neo4j.DriverWithContext, outputDir string) *Service {
	return &Service{Driver: driver, OutDir: outputDir}
}

// FindingBlast loads JSON and optionally Neo4j expansion.
func (s *Service) FindingBlast(ctx context.Context, scanID, findingID string, mode BlastMode) (schema.BlastRadiusSummary, *workingGraph, *models.Finding, UnavailableReason) {
	_, findings, err := scans.LoadScanArtifacts(s.OutDir, scanID)
	if err != nil {
		return schema.BlastRadiusSummary{}, nil, nil, ReasonUnknownRoot
	}
	var found *models.Finding
	for i := range findings {
		if findings[i].ID == findingID {
			found = &findings[i]
			break
		}
	}
	if found == nil {
		return schema.BlastRadiusSummary{}, nil, nil, ReasonUnknownRoot
	}
	signals := privilegeSignalsFromFinding(found)

	if s == nil || s.Driver == nil {
		return fallbackSummary(schema.BlastRootFinding, findingID, scanID, found, signals, ReasonNeo4jDisabled), nil, found, ReasonNeo4jDisabled
	}

	arn := strings.TrimSpace(found.AffectedARN)
	row, err := loadFindingContext(ctx, s.Driver, s.DBName, scanID, findingID)
	if err != nil {
		return fallbackSummary(schema.BlastRootFinding, findingID, scanID, found, signals, ReasonNeo4jConnectError), nil, found, ReasonNeo4jConnectError
	}
	if row != nil && row.AffectedAsset != nil && strings.TrimSpace(*row.AffectedAsset) != "" {
		arn = strings.TrimSpace(*row.AffectedAsset)
	}
	if arn == "" {
		sum := fallbackSummary(schema.BlastRootFinding, findingID, scanID, found, signals, ReasonNoGraphProjection)
		return sum, nil, found, ReasonNoGraphProjection
	}

	g, err := expandFromAsset(ctx, s.Driver, s.DBName, arn, scanID, V1MaxHops, mode)
	if err != nil {
		return fallbackSummary(schema.BlastRootFinding, findingID, scanID, found, signals, ReasonNeo4jConnectError), nil, found, ReasonNeo4jConnectError
	}
	g.AddFindingNode(findingID, found.Title, scanID, string(found.Severity), found.AffectedARN)
	g.ensureMinimalNode(arn)
	g.ensureMinimalNode("finding:" + findingID)
	g.addEdge("finding:"+findingID, arn, "AFFECTS")

	sum := BuildSummaryPayload(
		schema.BlastRootFinding, findingID, scanID, mode, g, arn, findingID, signals, true, ReasonNone)
	sum.FocalResourceARN = arn
	sum.SourceFindingID = findingID
	return sum, g, found, ReasonNone
}

func fallbackSummary(
	kind schema.BlastRootKind, rootID, scanID string,
	f *models.Finding,
	signals PrivilegeSignals,
	reason UnavailableReason,
) schema.BlastRadiusSummary {
	s := schema.BlastRadiusSummary{
		RootType:               kind,
		RootID:                 rootID,
		ScanID:                 scanID,
		Mode:                   "blast_radius",
		EscalationPossible:     signals.AdminLike || signals.IAMWriteAccess || signals.HasExternalTrust,
		GraphAvailable:         false,
		GraphUnavailableReason: string(reason),
		ReachableResourceCount: 0,
		ReachableAccountsCount: 0,
		SummaryText:            "",
		RecommendedActionLabel: "Connect Neo4j and export a scan to enable graph reachability",
		SourceFindingID:        f.ID,
		FocalResourceARN:       f.AffectedARN,
	}
	s.SummaryText = s.RecommendedActionLabel + ". " + buildStaticFindingNarrative(f)
	return s
}

func buildStaticFindingNarrative(f *models.Finding) string {
	if f == nil {
		return ""
	}
	if f.Impact != "" {
		return clip(f.Impact, 400)
	}
	return "Why this finding matters: treat " + f.Title + " as a scoped operational risk. Graph reachability is optional; use prioritization, ownership, and remediation in Cloudrift."
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ExternalEntityBlast builds blast radius for a URL-safe external entity id.
func (s *Service) ExternalEntityBlast(
	ctx context.Context, scanID, entityID string, mode BlastMode,
) (schema.BlastRadiusSummary, *workingGraph, UnavailableReason) {
	_, _, _, ok := DecodeExternalEntityID(entityID)
	if !ok {
		sum := BuildSummaryPayload(
			schema.BlastRootExternalEntity, entityID, scanID, mode, nil, "", "", PrivilegeSignals{}, false, ReasonUnknownRoot,
		)
		sum.SourceEntityID = entityID
		return sum, nil, ReasonUnknownRoot
	}
	_, findings, err := scans.LoadScanArtifacts(s.OutDir, scanID)
	if err != nil {
		return schema.BlastRadiusSummary{}, nil, ReasonUnknownRoot
	}
	ep, pt, ea, _ := DecodeExternalEntityID(entityID)
	matched := MatchExternalEntityFindings(findings, ep, pt, ea)
	arns := RoleARNsFromFindings(matched)
	signalList := make([]PrivilegeSignals, 0, len(matched))
	for i := range matched {
		signalList = append(signalList, privilegeSignalsFromFinding(&matched[i]))
	}
	aggSignals := combineSignals(signalList)
	if s == nil || s.Driver == nil {
		return BuildSummaryPayload(schema.BlastRootExternalEntity, entityID, scanID, mode, nil, "", "", aggSignals, false, ReasonNeo4jDisabled), nil, ReasonNeo4jDisabled
	}
	if len(arns) == 0 {
		sum := BuildSummaryPayload(schema.BlastRootExternalEntity, entityID, scanID, mode, nil, "", "", aggSignals, false, ReasonNoGraphProjection)
		sum.SourceEntityID = entityID
		return sum, nil, ReasonNoGraphProjection
	}
	merged := newWorkingGraph()
	for _, arn := range arns {
		g, xerr := expandFromAsset(ctx, s.Driver, s.DBName, arn, scanID, V1MaxHops, mode)
		if xerr != nil {
			return BuildSummaryPayload(schema.BlastRootExternalEntity, entityID, scanID, mode, nil, arn, "", aggSignals, false, ReasonNeo4jConnectError), nil, ReasonNeo4jConnectError
		}
		mergeWorking(merged, g)
	}
	sum := BuildSummaryPayload(schema.BlastRootExternalEntity, entityID, scanID, mode, merged, arns[0], "", aggSignals, true, ReasonNone)
	sum.SourceEntityID = entityID
	return sum, merged, ReasonNone
}

// PrincipalBlast is asset-ARN centered (usually an IAM role).
func (s *Service) PrincipalBlast(ctx context.Context, scanID, principalARN string, mode BlastMode) (schema.BlastRadiusSummary, *workingGraph, UnavailableReason) {
	if s == nil {
		return BuildSummaryPayload(schema.BlastRootPrincipal, principalARN, scanID, mode, nil, "", "", PrivilegeSignals{Confidence: "none"}, false, ReasonNeo4jDisabled), nil, ReasonNeo4jDisabled
	}
	if strings.TrimSpace(principalARN) == "" {
		return schema.BlastRadiusSummary{}, nil, ReasonUnknownRoot
	}
	_, findings, _ := scans.LoadScanArtifacts(s.OutDir, scanID)
	principalSignals := privilegeSignalsForPrincipalRoot(findings, principalARN)
	if s.Driver == nil {
		return BuildSummaryPayload(schema.BlastRootPrincipal, principalARN, scanID, mode, nil, "", "", principalSignals, false, ReasonNeo4jDisabled), nil, ReasonNeo4jDisabled
	}
	g, err := expandFromAsset(ctx, s.Driver, s.DBName, principalARN, scanID, V1MaxHops, mode)
	if err != nil {
		return BuildSummaryPayload(schema.BlastRootPrincipal, principalARN, scanID, mode, nil, principalARN, "", principalSignals, false, ReasonNeo4jConnectError), nil, ReasonNeo4jConnectError
	}
	sum := BuildSummaryPayload(schema.BlastRootPrincipal, principalARN, scanID, mode, g, principalARN, "", principalSignals, true, ReasonNone)
	sum.SourcePrincipalARN = principalARN
	sum.SourcePrincipalID = EncodePrincipalID(principalARN, "role", "")
	return sum, g, ReasonNone
}

// PrincipalBlastByID decodes principal_id and delegates to PrincipalBlast.
func (s *Service) PrincipalBlastByID(ctx context.Context, scanID, principalID string, mode BlastMode) (schema.BlastRadiusSummary, *workingGraph, UnavailableReason) {
	arn, pType, accountID, ok := DecodePrincipalID(principalID)
	if !ok || strings.TrimSpace(arn) == "" {
		sum := BuildSummaryPayload(schema.BlastRootPrincipal, principalID, scanID, mode, nil, "", "", PrivilegeSignals{}, false, ReasonUnknownRoot)
		sum.SourcePrincipalID = principalID
		return sum, nil, ReasonUnknownRoot
	}
	sum, g, reason := s.PrincipalBlast(ctx, scanID, arn, mode)
	sum.SourcePrincipalID = principalID
	if strings.TrimSpace(sum.SourcePrincipalARN) == "" {
		sum.SourcePrincipalARN = arn
	}
	if strings.TrimSpace(accountID) != "" {
		sum.TopImpactedAccounts = append([]string{accountID}, sum.TopImpactedAccounts...)
	}
	_ = pType
	return sum, g, reason
}

// ExplorerFromFinding returns the curated explorer DTO.
func (s *Service) ExplorerFromFinding(ctx context.Context, scanID, findingID string, mode BlastMode) schema.BlastExplorerResponse {
	summary, g, f, re := s.FindingBlast(ctx, scanID, findingID, mode)
	arn := summary.FocalResourceARN
	if arn == "" && f != nil {
		arn = f.AffectedARN
	}
	_ = re
	if g == nil {
		return BuildExplorerPayload(summary, "finding:"+findingID, mode, findingID, g)
	}
	return BuildExplorerPayload(summary, arn, mode, findingID, g)
}

// ExplorerFromEntity returns the explorer for an external entity id.
func (s *Service) ExplorerFromEntity(ctx context.Context, scanID, entityID string, mode BlastMode) schema.BlastExplorerResponse {
	sum, g, _ := s.ExternalEntityBlast(ctx, scanID, entityID, mode)
	focus := "entity:" + entityID
	if g != nil {
		for id, n := range g.Nodes {
			if n.NType == "asset" {
				focus = id
				break
			}
			_ = id
		}
	}
	return BuildExplorerPayload(sum, focus, mode, "", g)
}

// ExplorerFromPrincipalID returns the explorer for an encoded principal id.
func (s *Service) ExplorerFromPrincipalID(ctx context.Context, scanID, principalID string, mode BlastMode) schema.BlastExplorerResponse {
	sum, g, _ := s.PrincipalBlastByID(ctx, scanID, principalID, mode)
	focus := sum.SourcePrincipalARN
	if strings.TrimSpace(focus) == "" {
		focus = "principal:" + principalID
	}
	return BuildExplorerPayload(sum, focus, mode, "", g)
}

// ExplorerExpandOneHop returns a bounded one-hop graph delta from a node within the current root context.
func (s *Service) ExplorerExpandOneHop(
	ctx context.Context,
	scanID string,
	root ExpansionRootContext,
	nodeID string,
	mode BlastMode,
) schema.BlastExplorerExpansionResponse {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return schema.BlastExplorerExpansionResponse{
			ExpandedFromNodeID: nodeID,
			ExpansionApplied:   false,
			ExpansionReason:    "invalid_node_id",
		}
	}
	if s == nil || s.Driver == nil {
		return schema.BlastExplorerExpansionResponse{
			ExpandedFromNodeID:     nodeID,
			ExpansionApplied:       false,
			GraphUnavailable:       true,
			GraphUnavailableReason: string(ReasonNeo4jDisabled),
			ExpansionReason:        "graph_unavailable",
		}
	}
	_, base, reason := s.resolveRootGraph(ctx, scanID, root, mode)
	if base == nil {
		return schema.BlastExplorerExpansionResponse{
			ExpandedFromNodeID:     nodeID,
			ExpansionApplied:       false,
			GraphUnavailable:       true,
			GraphUnavailableReason: string(reason),
			ExpansionReason:        "graph_unavailable",
		}
	}
	if _, ok := base.Nodes[nodeID]; !ok {
		return schema.BlastExplorerExpansionResponse{
			ExpandedFromNodeID: nodeID,
			ExpansionApplied:   false,
			ExpansionReason:    "node_not_in_root_context",
		}
	}
	deltaGraph, triples, err := expandOneHopFromNode(ctx, s.Driver, s.DBName, nodeID, scanID, mode)
	if err != nil {
		return schema.BlastExplorerExpansionResponse{
			ExpandedFromNodeID:     nodeID,
			ExpansionApplied:       false,
			GraphUnavailable:       true,
			GraphUnavailableReason: string(ReasonNeo4jConnectError),
			ExpansionReason:        "graph_unavailable",
		}
	}
	if len(triples) == 0 {
		return schema.BlastExplorerExpansionResponse{
			ExpandedFromNodeID: nodeID,
			ExpansionApplied:   false,
			ExpansionReason:    "no_additional_high_signal_neighbors",
		}
	}
	resp := BuildExplorerExpansionDelta(nodeID, base, deltaGraph, mode)
	if !resp.ExpansionApplied {
		resp.ExpansionReason = "no_additional_high_signal_neighbors"
	}
	return resp
}

func (s *Service) resolveRootGraph(
	ctx context.Context,
	scanID string,
	root ExpansionRootContext,
	mode BlastMode,
) (schema.BlastRadiusSummary, *workingGraph, UnavailableReason) {
	switch root.RootType {
	case schema.BlastRootFinding:
		sum, g, _, reason := s.FindingBlast(ctx, scanID, root.RootID, mode)
		return sum, g, reason
	case schema.BlastRootExternalEntity:
		sum, g, reason := s.ExternalEntityBlast(ctx, scanID, root.RootID, mode)
		return sum, g, reason
	case schema.BlastRootPrincipal:
		sum, g, reason := s.PrincipalBlastByID(ctx, scanID, root.RootID, mode)
		return sum, g, reason
	default:
		return schema.BlastRadiusSummary{}, nil, ReasonUnknownRoot
	}
}
