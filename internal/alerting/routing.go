package alerting

import (
	"fmt"
	"strings"
)

// Routing sources describe how a delivery target was chosen.
const (
	RoutingSourceExplicitRule = "explicit_rule"
	RoutingSourceTeamAccount  = "team_account"
	RoutingSourceTeamDefault  = "team_default"
	RoutingSourceUnresolved   = "unresolved"
)

// Routing modes are API/UI summaries (subset overlaps source).
const (
	RoutingModeExplicitSlack = "explicit_slack"
	RoutingModeTeamSlack   = "team_slack"
	RoutingModeTeamDefault = "team_default"
	RoutingModeUnresolved  = "unresolved"
)

// DestinationResolution is the outcome of resolving where an alert should be sent.
// It is separate from evaluation (what happened in the scan).
type DestinationResolution struct {
	Source            string `json:"source"`
	Label             string `json:"label"`
	Detail            string `json:"detail"`
	Valid             bool   `json:"valid"`
	TeamID            string `json:"team_id,omitempty"`
	ResolvedAccountID string `json:"resolved_account_id,omitempty"`
}

// RoutingInput carries rule + optional account hints from evaluation (ordered).
type RoutingInput struct {
	Rule             AlertRule
	HintAccountIDs   []string
}

// ResolveDestination preserves backward compatibility: explicit rule webhook only
// (no catalog, no hints). Used where scan context is unavailable.
func ResolveDestination(rule AlertRule) DestinationResolution {
	d, _ := ResolveDeliveryTarget(RoutingInput{Rule: rule}, RoutingCatalog{})
	return d
}

// ResolveDeliveryTarget picks Slack delivery target with precedence:
//  1) Valid https:// webhook on the rule (explicit override)
//  2) First matching account in orderedAccountHints with a catalog binding to a team that has a valid webhook
//  3) default_team_id in catalog when set
//  4) unresolved (Valid=false)
//
// NOTE: This is single-destination routing (one alert → one Slack webhook). Multi-account fan-out
// (same alert to several teams) is intentionally not implemented — dominant account wins by hint order.
//
// Hint order (see orderedAccountHints): evaluation-derived routing_hint_account_ids first (dominant
// account by finding volume from the evaluator), then rule scope account_ids. That is an explicit policy:
// "dominant account routing" for v1, not neutral infrastructure behavior.
//
// API rule list enrichment resolves without scan signals (scope accounts only), so its label can differ
// from preview/events — callers should label that "estimated" vs "resolved from scan" in UX.
func ResolveDeliveryTarget(in RoutingInput, cat RoutingCatalog) (DestinationResolution, Channel) {
	rule := in.Rule
	ch := Channel{
		Type:            rule.Channel.Type,
		DisplayName:     strings.TrimSpace(rule.Channel.DisplayName),
		SlackWebhookURL: strings.TrimSpace(rule.Channel.SlackWebhookURL),
	}
	if ch.Type == "" {
		ch.Type = ChannelSlackWebhook
	}

	explicitURL := strings.TrimSpace(rule.Channel.SlackWebhookURL)
	if explicitURL != "" {
		label := strings.TrimSpace(rule.Channel.DisplayName)
		if label == "" {
			label = "Slack incoming webhook"
		}
		if validSlackURL(explicitURL) {
			return DestinationResolution{
				Source: RoutingSourceExplicitRule,
				Label:  label,
				Detail: "Uses the Slack webhook configured on this rule (explicit override). Team routing is skipped.",
				Valid:  true,
			}, ch
		}
		return DestinationResolution{
			Source:            RoutingSourceExplicitRule,
			Label:             label,
			Detail:            "Rule has a webhook URL but it must be https:// to send. Fix the URL or clear it to use team routing.",
			Valid:             false,
			ResolvedAccountID: "",
		}, Channel{Type: ch.Type, DisplayName: ch.DisplayName, SlackWebhookURL: ""}
	}

	// No explicit URL: team / default catalog path
	hints := orderedAccountHints(in)
	teams := cat.teamByID()
	a2t := cat.accountToTeam()

	for _, acct := range hints {
		acct = strings.TrimSpace(acct)
		if acct == "" {
			continue
		}
		tid := strings.TrimSpace(a2t[acct])
		if tid == "" {
			continue
		}
		team, ok := teams[tid]
		if !ok || !validSlackURL(team.SlackWebhookURL) {
			continue
		}
		lbl := strings.TrimSpace(team.DisplayName)
		if lbl == "" {
			lbl = tid
		}
		eff := Channel{
			Type:            ChannelSlackWebhook,
			DisplayName:     lbl,
			SlackWebhookURL: strings.TrimSpace(team.SlackWebhookURL),
		}
		return DestinationResolution{
			Source:            RoutingSourceTeamAccount,
			Label:             lbl,
			Detail:            fmt.Sprintf("No webhook on the rule; routed via account %s → team %s (catalog mapping).", acct, tid),
			Valid:             true,
			TeamID:            tid,
			ResolvedAccountID: acct,
		}, eff
	}

	if def := strings.TrimSpace(cat.DefaultTeamID); def != "" {
		team, ok := teams[def]
		if ok && validSlackURL(team.SlackWebhookURL) {
			lbl := strings.TrimSpace(team.DisplayName)
			if lbl == "" {
				lbl = def
			}
			eff := Channel{
				Type:            ChannelSlackWebhook,
				DisplayName:     lbl,
				SlackWebhookURL: strings.TrimSpace(team.SlackWebhookURL),
			}
			return DestinationResolution{
				Source: RoutingSourceTeamDefault,
				Label:  lbl,
				Detail: fmt.Sprintf("No rule webhook and no matching account hint; using catalog default_team_id %q.", def),
				Valid:  true,
				TeamID: def,
			}, eff
		}
	}

	return DestinationResolution{
		Source: RoutingSourceUnresolved,
		Label:  "No Slack destination",
		Detail: "Add an https:// webhook on the rule, or configure account→team bindings (and team webhooks), or set default_team_id in the routing catalog.",
		Valid:  false,
	}, Channel{Type: ChannelSlackWebhook, DisplayName: ch.DisplayName, SlackWebhookURL: ""}
}

func orderedAccountHints(in RoutingInput) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	for _, id := range in.HintAccountIDs {
		add(id)
	}
	for _, id := range normalizeScopeIDs(in.Rule.Scope.AccountIDs) {
		add(id)
	}
	return out
}

// RoutingModeForDestination maps resolution to a compact API mode string.
func RoutingModeForDestination(d DestinationResolution) string {
	switch d.Source {
	case RoutingSourceExplicitRule:
		if d.Valid {
			return RoutingModeExplicitSlack
		}
		return RoutingModeUnresolved
	case RoutingSourceTeamAccount:
		return RoutingModeTeamSlack
	case RoutingSourceTeamDefault:
		return RoutingModeTeamDefault
	default:
		return RoutingModeUnresolved
	}
}

// DestinationFromEventMetadata rebuilds resolution from persisted event metadata.
func DestinationFromEventMetadata(m map[string]any) (DestinationResolution, bool) {
	if m == nil {
		return DestinationResolution{}, false
	}
	src, _ := m["routing_source"].(string)
	if src == "" {
		return DestinationResolution{}, false
	}
	label, _ := m["destination_label"].(string)
	detail, _ := m["routing_detail"].(string)
	var valid bool
	switch v := m["destination_valid"].(type) {
	case bool:
		valid = v
	case float64:
		valid = v != 0
	}
	teamID, _ := m["routing_team_id"].(string)
	acct, _ := m["routing_resolved_account_id"].(string)
	return DestinationResolution{
		Source:            src,
		Label:             label,
		Detail:            detail,
		Valid:             valid,
		TeamID:            teamID,
		ResolvedAccountID: acct,
	}, true
}
