package queryv2

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"cloudrift/internal/blastradius"
	"cloudrift/internal/config"
	"cloudrift/internal/graph"
	"cloudrift/internal/models"
	"cloudrift/internal/scans"
)

type Service struct {
	outDir string
	cfg    *config.Config
	blast  *blastradius.Service
}

func NewService(outputDir string, cfg *config.Config, blast *blastradius.Service) *Service {
	return &Service{outDir: outputDir, cfg: cfg, blast: blast}
}

func (s *Service) Execute(ctx context.Context, req QueryRequest) (QueryResponse, error) {
	scanInput := strings.TrimSpace(req.ScanID)
	if scanInput == "" {
		scanInput = "latest"
	}
	scanID, err := scans.ResolveScanDirectoryName(s.outDir, scanInput)
	if err != nil {
		return QueryResponse{}, fmt.Errorf("resolve scan scope: %w", err)
	}
	_, findings, err := scans.LoadScanArtifacts(s.outDir, scanID)
	if err != nil {
		return QueryResponse{}, fmt.Errorf("load scan artifacts: %w", err)
	}

	plan := buildPlan(req, scanID)
	resp := QueryResponse{
		Intent:          plan.Intent,
		AnswerType:      plan.AnswerType,
		ScanID:          scanID,
		DomainUsed:      true,
		SupportingFacts: make([]SupportingFact, 0, 8),
		RelatedObjects:  make([]RelatedObject, 0, 8),
	}

	// Semantic retrieval reuses existing graph RAG index when available and bounded.
	var semanticHits []graph.RAGRetrievalHit
	if plan.NeedsSemantic {
		semanticHits = s.semanticRetrieve(ctx, plan, req.Query)
		if len(semanticHits) > 0 {
			resp.SemanticUsed = true
		}
	}

	switch plan.Intent {
	case IntentBlastRadius, IntentRiskExplanation:
		s.composeBlastOrRisk(ctx, &resp, plan, findings, semanticHits)
	case IntentExternalReach:
		s.composeExternalReach(&resp, plan, findings)
	case IntentTrustChains:
		s.composeTrustChains(&resp, findings)
	case IntentPrioritizeFixes:
		s.composePrioritizeFixes(&resp, plan, findings)
	case IntentLargestImpact:
		s.composeLargestImpact(ctx, &resp, findings)
	case IntentOwnership:
		s.composeOwnership(&resp, plan, findings)
	case IntentRemediation:
		s.composeRemediation(&resp, findings, semanticHits)
	default:
		s.composeSummary(&resp, findings)
	}

	if strings.TrimSpace(resp.Answer) == "" {
		resp.Answer = "No high-confidence answer could be composed from the current scan scope."
		resp.Confidence = "low"
		resp.SupportLevel = "low"
	}
	if resp.Confidence == "" {
		resp.Confidence = "medium"
	}
	if resp.SupportLevel == "" {
		resp.SupportLevel = "medium"
	}
	return resp, nil
}

func (s *Service) semanticRetrieve(ctx context.Context, plan Plan, query string) []graph.RAGRetrievalHit {
	if s == nil || s.blast == nil || s.blast.Driver == nil || s.cfg == nil {
		return nil
	}
	embed, pm, err := graph.NewEmbeddingProvider(s.cfg)
	if err != nil {
		return nil
	}
	rowReader := graph.NewDriverRowReader(s.blast.Driver, "")
	r, err := graph.RetrieveFindingContext(ctx, graph.RAGRetrievalInput{
		QueryText: query,
		ScanID:    plan.ScanID,
		TopK:      5,
	}, rowReader, embed, pm)
	if err != nil || r == nil {
		return nil
	}
	return r.Hits
}

func (s *Service) composeBlastOrRisk(ctx context.Context, out *QueryResponse, plan Plan, findings []models.Finding, semantic []graph.RAGRetrievalHit) {
	if s == nil || s.blast == nil {
		out.Answer = "Blast analysis service is unavailable."
		out.Notes = append(out.Notes, "graph retrieval unavailable")
		out.Confidence = "low"
		return
	}
	mode := blastradius.ModeBlastRadius
	if strings.EqualFold(plan.ModeHint, "attack_path") {
		mode = blastradius.ModeAttackPath
	}
	switch {
	case plan.FindingID != "":
		sum, _, _, reason := s.blast.FindingBlast(ctx, plan.ScanID, plan.FindingID, mode)
		out.GraphUsed = sum.GraphAvailable
		out.Answer = sum.SummaryText
		out.RecommendedAction = append(out.RecommendedAction, sum.RecommendedActionLabel)
		out.SupportingFacts = append(out.SupportingFacts,
			SupportingFact{Label: "reachable_resources", Value: itoa(sum.ReachableResourceCount), Source: "graph"},
			SupportingFact{Label: "reachable_accounts", Value: itoa(sum.ReachableAccountsCount), Source: "graph"},
			SupportingFact{Label: "dominant_motif", Value: sum.DominantMotif, Source: "graph"},
		)
		out.RelatedObjects = append(out.RelatedObjects, RelatedObject{
			Type: "finding", ID: plan.FindingID, URL: "/findings?scan_id=" + plan.ScanID,
		})
		if !sum.GraphAvailable {
			out.Notes = append(out.Notes, "graph unavailable: "+string(reason))
		}
	case plan.PrincipalID != "":
		sum, _, reason := s.blast.PrincipalBlastByID(ctx, plan.ScanID, plan.PrincipalID, mode)
		out.GraphUsed = sum.GraphAvailable
		out.Answer = sum.SummaryText
		out.RecommendedAction = append(out.RecommendedAction, sum.RecommendedActionLabel)
		out.SupportingFacts = append(out.SupportingFacts,
			SupportingFact{Label: "principal_id", Value: plan.PrincipalID, Source: "domain"},
			SupportingFact{Label: "dominant_motif", Value: sum.DominantMotif, Source: "graph"},
		)
		if !sum.GraphAvailable {
			out.Notes = append(out.Notes, "graph unavailable: "+string(reason))
		}
	case plan.EntityID != "":
		sum, _, reason := s.blast.ExternalEntityBlast(ctx, plan.ScanID, plan.EntityID, mode)
		out.GraphUsed = sum.GraphAvailable
		out.Answer = sum.SummaryText
		out.RecommendedAction = append(out.RecommendedAction, sum.RecommendedActionLabel)
		out.SupportingFacts = append(out.SupportingFacts,
			SupportingFact{Label: "entity_id", Value: plan.EntityID, Source: "domain"},
			SupportingFact{Label: "reachable_accounts", Value: itoa(sum.ReachableAccountsCount), Source: "graph"},
		)
		if !sum.GraphAvailable {
			out.Notes = append(out.Notes, "graph unavailable: "+string(reason))
		}
	default:
		// Fallback to top risky finding blast context.
		top := topFindings(findings, plan.AccountID, 1)
		if len(top) == 0 {
			out.Answer = "No findings available in the selected scan scope."
			return
		}
		sum, _, _, reason := s.blast.FindingBlast(ctx, plan.ScanID, top[0].ID, mode)
		out.GraphUsed = sum.GraphAvailable
		out.Answer = sum.SummaryText
		out.RecommendedAction = append(out.RecommendedAction, sum.RecommendedActionLabel)
		if !sum.GraphAvailable {
			out.Notes = append(out.Notes, "graph unavailable: "+string(reason))
		}
	}
	if len(semantic) > 0 {
		h := semantic[0]
		out.SupportingFacts = append(out.SupportingFacts, SupportingFact{
			Label: "semantic_context", Value: h.Title, Source: "semantic",
		})
	}
}

func (s *Service) composeExternalReach(out *QueryResponse, plan Plan, findings []models.Finding) {
	target := strings.ToLower(strings.TrimSpace(plan.AccountID))
	matches := make([]models.Finding, 0, 8)
	for _, f := range findings {
		if !strings.EqualFold(string(f.Module), "external_access") {
			continue
		}
		if target != "" && !strings.EqualFold(f.AccountID, target) {
			continue
		}
		matches = append(matches, f)
	}
	sort.Slice(matches, func(i, j int) bool { return riskScore(matches[i]) > riskScore(matches[j]) })
	if len(matches) == 0 {
		out.Answer = "No external-access findings matched the requested scope."
		out.Confidence = "medium"
		return
	}
	top := matches[0]
	out.Answer = fmt.Sprintf("External access can reach %d scoped finding(s); highest-risk path is `%s` in account `%s`.", len(matches), top.ID, top.AccountID)
	out.SupportingFacts = append(out.SupportingFacts,
		SupportingFact{Label: "external_findings", Value: itoa(len(matches)), Source: "domain"},
		SupportingFact{Label: "top_finding", Value: top.Title, Source: "domain"},
	)
	out.RelatedObjects = append(out.RelatedObjects, RelatedObject{
		Type: "finding", ID: top.ID, URL: "/findings?scan_id=" + plan.ScanID,
	})
	out.RecommendedAction = append(out.RecommendedAction, "Review trust relationship and remove stale or broad external principal access.")
	out.Confidence = "high"
	out.SupportLevel = "high"
}

func (s *Service) composeTrustChains(out *QueryResponse, findings []models.Finding) {
	matches := make([]models.Finding, 0, 8)
	for _, f := range findings {
		if strings.EqualFold(string(f.Module), "external_access") {
			matches = append(matches, f)
		}
	}
	sort.Slice(matches, func(i, j int) bool { return riskScore(matches[i]) > riskScore(matches[j]) })
	if len(matches) == 0 {
		out.Answer = "No cross-account trust findings were detected in this scan."
		return
	}
	limit := 3
	if len(matches) < limit {
		limit = len(matches)
	}
	out.Answer = fmt.Sprintf("Detected %d risky trust-chain candidate(s); top %d are prioritized for investigation.", len(matches), limit)
	for i := 0; i < limit; i++ {
		out.SupportingFacts = append(out.SupportingFacts, SupportingFact{
			Label: "trust_chain_candidate", Value: matches[i].ID + " · " + matches[i].AccountID, Source: "domain",
		})
	}
	out.RecommendedAction = append(out.RecommendedAction, "Open blast explorer in attack-path mode for the top chain and validate pivot feasibility.")
	out.Confidence = "medium"
	out.SupportLevel = "medium"
}

func (s *Service) composePrioritizeFixes(out *QueryResponse, plan Plan, findings []models.Finding) {
	top := topFindings(findings, plan.AccountID, 5)
	if len(top) == 0 {
		out.Answer = "No findings available to prioritize in this scope."
		return
	}
	out.Answer = fmt.Sprintf("Prioritize %d finding(s) first based on Cloudrift risk cost, severity, and claimability.", len(top))
	for _, f := range top {
		out.SupportingFacts = append(out.SupportingFacts, SupportingFact{
			Label: f.ID, Value: fmt.Sprintf("%s · risk $%.2f/mo", f.Severity, f.MonthlyRiskCost), Source: "domain",
		})
		out.RelatedObjects = append(out.RelatedObjects, RelatedObject{Type: "finding", ID: f.ID, URL: "/findings?scan_id=" + plan.ScanID})
	}
	out.RecommendedAction = append(out.RecommendedAction, "Use remediation groups/top fixes to batch these items by shared action.")
	out.Confidence = "high"
	out.SupportLevel = "high"
}

func (s *Service) composeLargestImpact(ctx context.Context, out *QueryResponse, findings []models.Finding) {
	top := topFindings(findings, "", 3)
	if len(top) == 0 {
		out.Answer = "No findings were available for impact ranking."
		return
	}
	out.Answer = "Largest reachable impact is concentrated in the highest-priority findings below."
	for _, f := range top {
		out.SupportingFacts = append(out.SupportingFacts, SupportingFact{
			Label: "impact_candidate", Value: f.ID + " · " + f.Title, Source: "domain",
		})
	}
	// Optional graph enrichment for the top item.
	if s != nil && s.blast != nil {
		sum, _, _, _ := s.blast.FindingBlast(ctx, out.ScanID, top[0].ID, blastradius.ModeBlastRadius)
		if sum.GraphAvailable {
			out.GraphUsed = true
			out.SupportingFacts = append(out.SupportingFacts, SupportingFact{
				Label: "reachable_resources_top", Value: itoa(sum.ReachableResourceCount), Source: "graph",
			})
		}
	}
}

func (s *Service) composeOwnership(out *QueryResponse, plan Plan, findings []models.Finding) {
	top := topFindings(findings, plan.AccountID, 1)
	if len(top) == 0 {
		out.Answer = "No ownership signal could be derived from this scope."
		return
	}
	f := top[0]
	team := strings.TrimSpace(f.Team)
	if team == "" {
		team = "unassigned"
	}
	out.Answer = fmt.Sprintf("The riskiest exposure in scope is `%s`, currently owned by team `%s`.", f.ID, team)
	out.SupportingFacts = append(out.SupportingFacts,
		SupportingFact{Label: "owner_team", Value: team, Source: "domain"},
		SupportingFact{Label: "account", Value: f.AccountID, Source: "domain"},
	)
	out.RecommendedAction = append(out.RecommendedAction, "Assign remediation owner and enforce SLA based on severity.")
}

func (s *Service) composeRemediation(out *QueryResponse, findings []models.Finding, semantic []graph.RAGRetrievalHit) {
	top := topFindings(findings, "", 3)
	if len(top) == 0 {
		out.Answer = "No remediation targets found in current scope."
		return
	}
	out.Answer = "Top remediation actions are derived from highest-risk findings."
	for _, f := range top {
		action := strings.TrimSpace(f.Recommendation)
		if action == "" {
			action = "Review finding and apply least-privilege + cleanup remediation."
		}
		out.RecommendedAction = append(out.RecommendedAction, action)
	}
	if len(semantic) > 0 {
		out.SupportingFacts = append(out.SupportingFacts, SupportingFact{
			Label: "semantic_remediation_context", Value: semantic[0].Recommendation, Source: "semantic",
		})
	}
}

func (s *Service) composeSummary(out *QueryResponse, findings []models.Finding) {
	top := topFindings(findings, "", 1)
	if len(top) == 0 {
		out.Answer = "No findings available for this scan scope."
		return
	}
	out.Answer = fmt.Sprintf("Scan has %d finding(s). Highest-risk item is `%s` (%s).", len(findings), top[0].ID, top[0].Severity)
	out.SupportingFacts = append(out.SupportingFacts, SupportingFact{
		Label: "finding_count", Value: itoa(len(findings)), Source: "domain",
	})
}

func topFindings(findings []models.Finding, accountID string, limit int) []models.Finding {
	filtered := make([]models.Finding, 0, len(findings))
	for _, f := range findings {
		if strings.TrimSpace(accountID) != "" && !strings.EqualFold(f.AccountID, accountID) {
			continue
		}
		filtered = append(filtered, f)
	}
	sort.Slice(filtered, func(i, j int) bool { return riskScore(filtered[i]) > riskScore(filtered[j]) })
	if limit <= 0 || len(filtered) <= limit {
		return filtered
	}
	return filtered[:limit]
}

func riskScore(f models.Finding) float64 {
	base := f.MonthlyRiskCost
	switch strings.ToLower(string(f.Severity)) {
	case "critical":
		base += 1000
	case "high":
		base += 500
	case "medium":
		base += 200
	case "low":
		base += 50
	}
	switch strings.ToLower(string(f.Claimability)) {
	case "reclaimable":
		base += 100
	case "dangling":
		base += 75
	}
	return base
}

func itoa(v int) string {
	return fmt.Sprintf("%d", v)
}
