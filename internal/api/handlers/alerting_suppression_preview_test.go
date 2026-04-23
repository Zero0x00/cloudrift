package handlers

import (
	"testing"
	"time"

	"cloudrift/internal/alerting"
)

func TestToSchemaSuppressionPreview(t *testing.T) {
	t.Parallel()
	anchor := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	until := anchor.Add(30 * time.Minute)
	dec := alerting.CooldownDecision{
		Active:        true,
		AnchorEventID: "evt-old",
		AnchorTime:    anchor,
		Until:         until,
		Reason:        "within_per_rule_cooldown_after_delivered_alert",
	}
	out := toSchemaSuppressionPreview(30, true, dec, "")
	if !out.WouldSuppress || out.CooldownMinutes != 30 {
		t.Fatalf("unexpected: %#v", out)
	}
	if out.ReferenceEventID != "evt-old" || out.ActiveUntil == nil || !out.ActiveUntil.Equal(until) {
		t.Fatalf("anchor/until: %#v", out)
	}
	off := toSchemaSuppressionPreview(0, true, alerting.CooldownDecision{}, "cooldown disabled (0 minutes)")
	if off.WouldSuppress || off.CooldownMinutes != 0 {
		t.Fatalf("expected off: %#v", off)
	}
}
