package queryv2

import "strings"

func buildPlan(req QueryRequest, resolvedScanID string) Plan {
	q := strings.ToLower(strings.TrimSpace(req.Query))
	plan := Plan{
		Intent:        IntentSummary,
		NeedsDomain:   true,
		NeedsSemantic: true,
		AnswerType:    "summary",
		ScanID:        resolvedScanID,
		AccountID:     strings.TrimSpace(req.AccountID),
		FindingID:     strings.TrimSpace(req.FindingID),
		EntityID:      strings.TrimSpace(req.EntityID),
		PrincipalID:   strings.TrimSpace(req.PrincipalID),
		ModeHint:      strings.TrimSpace(req.ModeHint),
	}

	switch {
	case strings.Contains(q, "blast radius"):
		plan.Intent = IntentBlastRadius
		plan.NeedsGraph = true
		plan.NeedsSemantic = false
		plan.AnswerType = "relationship"
	case strings.Contains(q, "why") && strings.Contains(q, "risky"):
		plan.Intent = IntentRiskExplanation
		plan.NeedsGraph = true
		plan.NeedsSemantic = true
		plan.AnswerType = "explanation"
	case strings.Contains(q, "external") && strings.Contains(q, "reach"):
		plan.Intent = IntentExternalReach
		plan.NeedsGraph = true
		plan.NeedsSemantic = false
		plan.AnswerType = "relationship"
	case strings.Contains(q, "cross-account") && strings.Contains(q, "trust"):
		plan.Intent = IntentTrustChains
		plan.NeedsGraph = true
		plan.NeedsSemantic = false
		plan.AnswerType = "relationship"
	case strings.Contains(q, "fix first"):
		plan.Intent = IntentPrioritizeFixes
		plan.NeedsGraph = false
		plan.NeedsSemantic = false
		plan.AnswerType = "prioritization"
	case strings.Contains(q, "largest reachable impact") || strings.Contains(q, "largest impact"):
		plan.Intent = IntentLargestImpact
		plan.NeedsGraph = true
		plan.NeedsSemantic = false
		plan.AnswerType = "prioritization"
	case strings.Contains(q, "who owns"):
		plan.Intent = IntentOwnership
		plan.NeedsGraph = false
		plan.NeedsSemantic = false
		plan.AnswerType = "ownership"
	case strings.Contains(q, "remediation") || strings.Contains(q, "what should i do"):
		plan.Intent = IntentRemediation
		plan.NeedsGraph = false
		plan.NeedsSemantic = true
		plan.AnswerType = "remediation"
	}

	if plan.FindingID != "" || plan.EntityID != "" || plan.PrincipalID != "" {
		plan.NeedsGraph = true
	}
	return plan
}
