package scorers

import (
	"testing"

	"github.com/Zero0x00/cloudrift/internal/models"
)

func TestPriorityScoreSeverityContribution(t *testing.T) {
	base := models.Finding{Claimability: models.ClaimUnknown, MonthlyRiskCost: 0}
	critical := base
	critical.Severity = models.SeverityCritical
	medium := base
	medium.Severity = models.SeverityMedium
	if PriorityScore(critical) <= PriorityScore(medium) {
		t.Fatalf("critical should score above medium: %v <= %v", PriorityScore(critical), PriorityScore(medium))
	}
}

func TestPriorityScoreClaimabilityContribution(t *testing.T) {
	base := models.Finding{Severity: models.SeverityHigh, MonthlyRiskCost: 0}
	reclaim := base
	reclaim.Claimability = models.ClaimReclaimable
	unknown := base
	unknown.Claimability = models.ClaimUnknown
	if PriorityScore(reclaim) <= PriorityScore(unknown) {
		t.Fatalf("reclaimable should score above unknown: %v <= %v", PriorityScore(reclaim), PriorityScore(unknown))
	}
}

func TestPriorityScoreCostContribution(t *testing.T) {
	base := models.Finding{Severity: models.SeverityMedium, Claimability: models.ClaimUnknown}
	lowCost := base
	lowCost.MonthlyRiskCost = 10
	highCost := base
	highCost.MonthlyRiskCost = 500
	if PriorityScore(highCost) <= PriorityScore(lowCost) {
		t.Fatalf("higher risk cost should score above lower: %v <= %v", PriorityScore(highCost), PriorityScore(lowCost))
	}
}

func TestPriorityScoreExternalExposureContribution(t *testing.T) {
	plain := models.Finding{
		ID:           "plain",
		Severity:     models.SeverityHigh,
		Module:       models.ModuleOrphanedEdge,
		Claimability: models.ClaimUnknown,
	}
	external := models.Finding{
		ID:           "ext",
		Severity:     models.SeverityHigh,
		Module:       models.ModuleExternalAccess,
		Claimability: models.ClaimUnknown,
		Evidence: map[string]any{
			"verdict":         "stale_review_now",
			"principal_type":  "",
			"unknown_vendor":  true,
			"permission_visibility": map[string]any{
				"classification": "privileged",
				"capabilities":   map[string]any{"admin_like": true},
			},
		},
	}
	if PriorityScore(external) <= PriorityScore(plain) {
		t.Fatalf("external signal-rich finding should score above plain finding: %v <= %v", PriorityScore(external), PriorityScore(plain))
	}
}

func TestPriorityLessDeterministicTieBreak(t *testing.T) {
	a := models.Finding{ID: "a", Severity: models.SeverityHigh, MonthlyRiskCost: 10}
	b := models.Finding{ID: "b", Severity: models.SeverityHigh, MonthlyRiskCost: 10}
	if !PriorityLess(a, b, 100, 100) {
		t.Fatal("expected lexicographic ID tie-break when all else equal")
	}
}
