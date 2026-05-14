package alerting

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Zero0x00/cloudrift/internal/models"
	"github.com/Zero0x00/cloudrift/internal/scorers"
)

func totalMonthlyRiskUSD(findings []models.Finding) float64 {
	var t float64
	for _, f := range findings {
		t += f.MonthlyRiskCost
	}
	return t
}

func topFindingByPriority(findings []models.Finding) (models.Finding, bool) {
	if len(findings) == 0 {
		return models.Finding{}, false
	}
	best := findings[0]
	sb := scorers.PriorityScore(best)
	for i := 1; i < len(findings); i++ {
		f := findings[i]
		sf := scorers.PriorityScore(f)
		if scorers.PriorityLess(f, best, sf, sb) {
			best = f
			sb = sf
		}
	}
	return best, true
}

func ownerOneLiner(f models.Finding) string {
	if t := strings.TrimSpace(f.Team); t != "" {
		return t
	}
	if n := strings.TrimSpace(f.AccountName); n != "" {
		return n
	}
	if id := strings.TrimSpace(f.AccountID); id != "" {
		return id
	}
	return ""
}

func shortenTitle(title string, max int) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "(untitled)"
	}
	if len(title) <= max {
		return title
	}
	if max <= 3 {
		return title[:max]
	}
	return title[:max-1] + "…"
}

func riskTrendLine(prevRisk, currRisk float64) string {
	if prevRisk <= 0 && currRisk <= 0 {
		return ""
	}
	if prevRisk <= 0 {
		return "Prior scan had no modeled risk baseline to compare."
	}
	ratio := currRisk / prevRisk
	switch {
	case ratio > 1.08:
		return "Modeled monthly risk is up vs the prior scan — review posture."
	case ratio < 0.92:
		return "Modeled monthly risk is down vs the prior scan."
	default:
		return "Modeled monthly risk is broadly in line with the prior scan."
	}
}

// routingHintAccountIDsFromFindings returns distinct account_id values ordered by
// finding count (desc) for team-based routing hints. Only includes non-empty AccountID.
//
// NOTE: This encodes "dominant account by volume" for routing hints only; it does not fan out
// to every account present in findings.
func routingHintAccountIDsFromFindings(findings []models.Finding) []string {
	counts := map[string]int{}
	for _, f := range findings {
		id := strings.TrimSpace(f.AccountID)
		if id == "" {
			continue
		}
		counts[id]++
	}
	type kv struct {
		id string
		n  int
	}
	list := make([]kv, 0, len(counts))
	for id, n := range counts {
		list = append(list, kv{id: id, n: n})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].n == list[j].n {
			return list[i].id < list[j].id
		}
		return list[i].n > list[j].n
	})
	out := make([]string, 0, len(list))
	for _, e := range list {
		out = append(out, e.id)
	}
	return out
}

func topInternalAccountForFindings(findings []models.Finding) string {
	if len(findings) == 0 {
		return ""
	}
	counts := map[string]int{}
	for _, f := range findings {
		k := strings.TrimSpace(f.AccountID)
		if k == "" {
			k = strings.TrimSpace(f.AccountName)
		}
		if k == "" {
			continue
		}
		counts[k]++
	}
	return topMapKey(counts)
}

func topRemediationLine(findings []models.Finding) string {
	if len(findings) == 0 {
		return ""
	}
	counts := map[string]int{}
	for _, f := range findings {
		k := strings.TrimSpace(f.RemediationCmd)
		if k == "" {
			k = strings.TrimSpace(f.Impact)
		}
		if k == "" {
			continue
		}
		if len(k) > 72 {
			k = k[:69] + "…"
		}
		counts[k]++
	}
	top := topMapKey(counts)
	if top == "" {
		return ""
	}
	return fmt.Sprintf("Largest remediation bucket: %s.", top)
}
