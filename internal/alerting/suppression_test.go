package alerting

import (
	"testing"
	"time"
)

func TestDecideCooldownSuppression_zeroMeansOff(t *testing.T) {
	t.Parallel()
	ev := AlertEvent{
		ID:        "evt-1",
		RuleID:    "rule-a",
		Triggered: true,
		CreatedAt: time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC),
		Delivery: DeliveryResult{
			Provider:  "slack",
			Attempted: true,
			Success:   true,
		},
	}
	now := ev.CreatedAt.Add(1 * time.Minute)
	dec := DecideCooldownSuppression("rule-a", 0, now, []AlertEvent{ev})
	if dec.Active {
		t.Fatalf("expected no suppression when cooldown is 0")
	}
}

func TestDecideCooldownSuppression_suppressesWithinWindow(t *testing.T) {
	t.Parallel()
	anchor := AlertEvent{
		ID:        "anchor",
		RuleID:    "rule-a",
		Triggered: true,
		CreatedAt: time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC),
		Delivery: DeliveryResult{
			Provider:  "slack",
			Attempted: true,
			Success:   true,
		},
	}
	events := []AlertEvent{anchor}
	now := anchor.CreatedAt.Add(5 * time.Minute)
	dec := DecideCooldownSuppression("rule-a", 60, now, events)
	if !dec.Active {
		t.Fatalf("expected suppression within 60m window")
	}
	if dec.AnchorEventID != anchor.ID {
		t.Fatalf("anchor id: got %q want %q", dec.AnchorEventID, anchor.ID)
	}
	if !dec.Until.Equal(anchor.CreatedAt.Add(60 * time.Minute)) {
		t.Fatalf("until: got %v want %v", dec.Until, anchor.CreatedAt.Add(60*time.Minute))
	}
	if dec.Reason != suppressionReasonCooldown {
		t.Fatalf("reason: got %q", dec.Reason)
	}
}

func TestDecideCooldownSuppression_expiresAfterWindow(t *testing.T) {
	t.Parallel()
	anchor := AlertEvent{
		ID:        "anchor",
		RuleID:    "rule-a",
		Triggered: true,
		CreatedAt: time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC),
		Delivery: DeliveryResult{
			Provider:  "slack",
			Attempted: true,
			Success:   true,
		},
	}
	now := anchor.CreatedAt.Add(61 * time.Minute)
	dec := DecideCooldownSuppression("rule-a", 60, now, []AlertEvent{anchor})
	if dec.Active {
		t.Fatalf("expected cooldown expired")
	}
}

func TestIsCooldownAnchor_ignoresTestsAndSuppressed(t *testing.T) {
	t.Parallel()
	if isCooldownAnchor(AlertEvent{RuleID: "r", Triggered: true, ForcedTestSend: true, Delivery: DeliveryResult{Provider: "slack", Attempted: true, Success: true}}) {
		t.Fatal("test send must not anchor")
	}
	if isCooldownAnchor(AlertEvent{RuleID: "r", Triggered: true, Suppressed: true, Delivery: DeliveryResult{Provider: "slack", Attempted: true, Success: true}}) {
		t.Fatal("suppressed must not anchor")
	}
	if isCooldownAnchor(AlertEvent{RuleID: "r", Triggered: true, Delivery: DeliveryResult{Provider: "slack", Attempted: true, Success: false}}) {
		t.Fatal("failed delivery must not anchor")
	}
	if !isCooldownAnchor(AlertEvent{RuleID: "r", Triggered: true, Delivery: DeliveryResult{Provider: "slack", Attempted: true, Success: true}}) {
		t.Fatal("successful delivery should anchor")
	}
}

func TestValidateCooldownMinutes(t *testing.T) {
	t.Parallel()
	if err := validateCooldownMinutes(-1); err == nil {
		t.Fatal("expected error")
	}
	if err := validateCooldownMinutes(maxCooldownMinutes + 1); err == nil {
		t.Fatal("expected error")
	}
	if err := validateCooldownMinutes(30); err != nil {
		t.Fatal(err)
	}
}
