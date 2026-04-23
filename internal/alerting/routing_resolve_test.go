package alerting

import "testing"

func TestResolveDeliveryTarget_ExplicitOverridesTeam(t *testing.T) {
	cat := RoutingCatalog{
		Teams: []TeamSlackDestination{
			{TeamID: "sec", DisplayName: "Sec", SlackWebhookURL: "https://hooks.slack.com/services/TEAM/AAA/BBB"},
		},
		AccountTeams: []AccountTeamBinding{{AccountID: "111111111111", TeamID: "sec"}},
	}
	rule := AlertRule{
		Channel: Channel{
			Type:            ChannelSlackWebhook,
			DisplayName:     "#oncall",
			SlackWebhookURL: "https://hooks.slack.com/services/EXPL/CCC/DDD",
		},
	}
	d, ch := ResolveDeliveryTarget(RoutingInput{
		Rule:           rule,
		HintAccountIDs: []string{"111111111111"},
	}, cat)
	if d.Source != RoutingSourceExplicitRule || !d.Valid {
		t.Fatalf("want explicit valid, got %+v", d)
	}
	if ch.SlackWebhookURL != rule.Channel.SlackWebhookURL {
		t.Fatalf("channel should use explicit URL, got %q", ch.SlackWebhookURL)
	}
}

func TestResolveDeliveryTarget_TeamFromHintAccount(t *testing.T) {
	cat := RoutingCatalog{
		Teams: []TeamSlackDestination{
			{TeamID: "platform", DisplayName: "Platform", SlackWebhookURL: "https://hooks.slack.com/services/T/P/L"},
		},
		AccountTeams: []AccountTeamBinding{{AccountID: "999988887777", TeamID: "platform"}},
	}
	rule := AlertRule{Channel: Channel{Type: ChannelSlackWebhook}}
	d, ch := ResolveDeliveryTarget(RoutingInput{
		Rule:           rule,
		HintAccountIDs: []string{"999988887777"},
	}, cat)
	if d.Source != RoutingSourceTeamAccount || !d.Valid {
		t.Fatalf("got %+v", d)
	}
	if d.TeamID != "platform" || d.ResolvedAccountID != "999988887777" {
		t.Fatalf("team/account metadata %+v", d)
	}
	if ch.SlackWebhookURL != cat.Teams[0].SlackWebhookURL {
		t.Fatal("expected team webhook on channel")
	}
	if RoutingModeForDestination(d) != RoutingModeTeamSlack {
		t.Fatalf("mode %s", RoutingModeForDestination(d))
	}
}

func TestResolveDeliveryTarget_DefaultTeamWhenNoMatch(t *testing.T) {
	cat := RoutingCatalog{
		DefaultTeamID: "fallback",
		Teams: []TeamSlackDestination{
			{TeamID: "fallback", SlackWebhookURL: "https://hooks.slack.com/services/F/A/L"},
		},
	}
	rule := AlertRule{Channel: Channel{Type: ChannelSlackWebhook}}
	d, ch := ResolveDeliveryTarget(RoutingInput{
		Rule:           rule,
		HintAccountIDs: []string{"unknown-account"},
	}, cat)
	if d.Source != RoutingSourceTeamDefault || !d.Valid {
		t.Fatalf("got %+v", d)
	}
	if ch.SlackWebhookURL == "" {
		t.Fatal("expected default team webhook")
	}
}

func TestResolveDeliveryTarget_UnresolvedEmptyCatalog(t *testing.T) {
	rule := AlertRule{Channel: Channel{Type: ChannelSlackWebhook}}
	d, ch := ResolveDeliveryTarget(RoutingInput{Rule: rule, HintAccountIDs: []string{"123"}}, RoutingCatalog{})
	if d.Valid || d.Source != RoutingSourceUnresolved {
		t.Fatalf("got %+v", d)
	}
	if ch.SlackWebhookURL != "" {
		t.Fatal("expected empty channel URL")
	}
}

func TestResolveDeliveryTarget_InvalidExplicitDoesNotFallThrough(t *testing.T) {
	cat := RoutingCatalog{
		Teams: []TeamSlackDestination{
			{TeamID: "sec", SlackWebhookURL: "https://hooks.slack.com/services/T/E/A"},
		},
		AccountTeams: []AccountTeamBinding{{AccountID: "1", TeamID: "sec"}},
	}
	rule := AlertRule{
		Channel: Channel{Type: ChannelSlackWebhook, SlackWebhookURL: "http://insecure.example/hook"},
	}
	d, _ := ResolveDeliveryTarget(RoutingInput{Rule: rule, HintAccountIDs: []string{"1"}}, cat)
	if d.Valid {
		t.Fatal("invalid explicit should not fall through to team")
	}
	if d.Source != RoutingSourceExplicitRule {
		t.Fatalf("source %q", d.Source)
	}
}

func TestValidateRoutingCatalog_AccountUnknownTeam(t *testing.T) {
	err := ValidateRoutingCatalog(RoutingCatalog{
		AccountTeams: []AccountTeamBinding{{AccountID: "1", TeamID: "missing"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDestinationFromEventMetadata_RoundTrip(t *testing.T) {
	m := map[string]any{
		"routing_source":              RoutingSourceTeamAccount,
		"destination_label":           "Platform",
		"routing_detail":              "detail",
		"destination_valid":           true,
		"routing_team_id":             "platform",
		"routing_resolved_account_id": "111",
	}
	d, ok := DestinationFromEventMetadata(m)
	if !ok || d.Source != RoutingSourceTeamAccount || d.TeamID != "platform" {
		t.Fatalf("%+v ok=%v", d, ok)
	}
}
