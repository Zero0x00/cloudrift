package alerting

import (
	"fmt"
	"strings"
)

// RoutingCatalog holds account→team bindings and team Slack destinations.
// Persisted as _alerting/routing.json next to rules.json.
//
// Security: Slack webhook URLs are stored in plain text on disk. Treat the output directory as
// sensitive in production; encryption or secret backends are a future hardening step.
type RoutingCatalog struct {
	// DefaultTeamID is optional: used when no explicit rule webhook and no account→team match.
	// Powerful fallback but risky if overused — many alerts can collapse to one "security" channel
	// and undermine ownership. Prefer account_teams bindings keyed by real AWS account IDs.
	DefaultTeamID string `json:"default_team_id,omitempty"`
	Teams         []TeamSlackDestination `json:"teams,omitempty"`
	AccountTeams  []AccountTeamBinding   `json:"account_teams,omitempty"`
}

type TeamSlackDestination struct {
	TeamID          string `json:"team_id"`
	DisplayName     string `json:"display_name,omitempty"`
	SlackWebhookURL string `json:"slack_webhook_url"`
}

type AccountTeamBinding struct {
	AccountID string `json:"account_id"`
	TeamID    string `json:"team_id"`
}

func (c RoutingCatalog) teamByID() map[string]TeamSlackDestination {
	out := make(map[string]TeamSlackDestination, len(c.Teams))
	for _, t := range c.Teams {
		id := strings.TrimSpace(t.TeamID)
		if id == "" {
			continue
		}
		out[id] = t
	}
	return out
}

func (c RoutingCatalog) accountToTeam() map[string]string {
	out := make(map[string]string, len(c.AccountTeams))
	for _, b := range c.AccountTeams {
		a := strings.TrimSpace(b.AccountID)
		t := strings.TrimSpace(b.TeamID)
		if a == "" || t == "" {
			continue
		}
		out[a] = t
	}
	return out
}

// TeamForAccount returns team_id for an AWS account id (or other binding key).
func (c RoutingCatalog) TeamForAccount(accountID string) string {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return ""
	}
	return c.accountToTeam()[accountID]
}

// LookupTeam returns the team row if present.
func (c RoutingCatalog) LookupTeam(teamID string) (TeamSlackDestination, bool) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return TeamSlackDestination{}, false
	}
	t, ok := c.teamByID()[teamID]
	return t, ok
}

func validSlackURL(url string) bool {
	u := strings.TrimSpace(url)
	return u != "" && strings.HasPrefix(u, "https://")
}

// ValidateRoutingCatalog returns an error if the catalog is inconsistent.
func ValidateRoutingCatalog(c RoutingCatalog) error {
	teamIDs := c.teamByID()
	for _, b := range c.AccountTeams {
		a := strings.TrimSpace(b.AccountID)
		tid := strings.TrimSpace(b.TeamID)
		if a == "" {
			return fmt.Errorf("account_teams: account_id is required")
		}
		if tid == "" {
			return fmt.Errorf("account_teams: team_id is required for account %q", a)
		}
		if _, ok := teamIDs[tid]; !ok {
			return fmt.Errorf("account_teams: unknown team_id %q for account %q", tid, a)
		}
	}
	for _, t := range c.Teams {
		tid := strings.TrimSpace(t.TeamID)
		if tid == "" {
			return fmt.Errorf("teams: team_id is required")
		}
		if !validSlackURL(t.SlackWebhookURL) {
			return fmt.Errorf("teams: team %q needs a valid https:// Slack webhook URL", tid)
		}
	}
	if def := strings.TrimSpace(c.DefaultTeamID); def != "" {
		if _, ok := teamIDs[def]; !ok {
			return fmt.Errorf("default_team_id %q is not defined in teams", def)
		}
	}
	return nil
}
