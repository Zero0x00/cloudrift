package alerting

import "testing"

func TestResolveDestination_SlackExplicit(t *testing.T) {
	r := AlertRule{
		Channel: Channel{
			Type:            ChannelSlackWebhook,
			DisplayName:     "#sec-ops",
			SlackWebhookURL: "https://hooks.slack.com/services/AAA/BBB/CCC",
		},
	}
	d := ResolveDestination(r)
	if d.Source != RoutingSourceExplicitRule {
		t.Fatalf("source %q", d.Source)
	}
	if d.Label != "#sec-ops" {
		t.Fatalf("label %q", d.Label)
	}
	if !d.Valid {
		t.Fatal("expected valid webhook")
	}
	if d.Detail == "" {
		t.Fatal("expected detail")
	}
}

func TestResolveDestination_MissingWebhook(t *testing.T) {
	r := AlertRule{
		Channel: Channel{Type: ChannelSlackWebhook, DisplayName: "ops"},
	}
	d := ResolveDestination(r)
	if d.Valid {
		t.Fatal("expected invalid")
	}
	if d.Source != RoutingSourceUnresolved {
		t.Fatalf("source %q", d.Source)
	}
}

func TestResolveDestination_DefaultLabel(t *testing.T) {
	r := AlertRule{
		Channel: Channel{
			Type:            ChannelSlackWebhook,
			SlackWebhookURL: "https://hooks.slack.com/services/x/y/z",
		},
	}
	d := ResolveDestination(r)
	if d.Label != "Slack incoming webhook" {
		t.Fatalf("got %q", d.Label)
	}
}
