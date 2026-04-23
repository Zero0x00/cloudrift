package alerting

import (
	"fmt"
	"time"
)

// Cooldown v1 policy (per rule_id only; no fingerprinting):
//
//   - Automatic post-scan delivery: if the rule triggers and cooldown_minutes > 0, compare
//     now against the most recent prior event for the same rule that:
//     (a) was not a test send (forced_test_send == false),
//     (b) was not itself suppressed,
//     (c) had triggered == true,
//     (d) had a successful attempted delivery (Slack send succeeded).
//   - If that anchor falls within the cooldown window, skip Slack and record a suppressed event.
//   - Preview never reads/writes suppression state beyond listing events (read-only); it reports
//     whether a real automatic send would be suppressed.
//   - Manual Test always bypasses suppression when delivery would proceed, but records
//     cooldown_bypassed in metadata when a real run would have been suppressed. Test sends
//     never become the cooldown anchor (they do not "advance" the cooldown clock).
//
// Suppression is intentionally separate from evaluation and routing.

const (
	maxCooldownMinutes = 43200 // 30 days — sanity cap for API/store

	suppressionReasonCooldown = "within_per_rule_cooldown_after_delivered_alert"
)

// CooldownDecision is the outcome of cooldown inspection for one evaluation moment.
type CooldownDecision struct {
	Active bool

	// AnchorEventID is the delivered production event that started the cooldown window (if any).
	AnchorEventID string
	AnchorTime    time.Time

	Until time.Time

	Reason string
}

// isCooldownAnchor returns true if this historical event should reset/establish the cooldown
// window for automatic (non-test) sends.
func isCooldownAnchor(ev AlertEvent) bool {
	if ev.RuleID == "" || !ev.Triggered || ev.ForcedTestSend || ev.Suppressed {
		return false
	}
	if !ev.Delivery.Attempted || !ev.Delivery.Success {
		return false
	}
	// Only count real Slack deliveries (not synthetic "none" success rows).
	if ev.Delivery.Provider != "slack" {
		return false
	}
	return true
}

// findCooldownAnchor scans events newest-first (as returned by ListEvents) and returns the
// first anchor for ruleID.
func findCooldownAnchor(events []AlertEvent, ruleID string) *AlertEvent {
	for i := range events {
		ev := &events[i]
		if ev.RuleID != ruleID {
			continue
		}
		if isCooldownAnchor(*ev) {
			return ev
		}
	}
	return nil
}

// DecideCooldownSuppression returns whether an automatic send should be suppressed right now.
// now is typically UTC; events should be newest-first.
func DecideCooldownSuppression(ruleID string, cooldownMinutes int, now time.Time, events []AlertEvent) CooldownDecision {
	if cooldownMinutes <= 0 || ruleID == "" {
		return CooldownDecision{}
	}
	anchor := findCooldownAnchor(events, ruleID)
	if anchor == nil {
		return CooldownDecision{}
	}
	window := time.Duration(cooldownMinutes) * time.Minute
	until := anchor.CreatedAt.Add(window)
	if !now.Before(until) {
		return CooldownDecision{}
	}
	return CooldownDecision{
		Active:        true,
		AnchorEventID: anchor.ID,
		AnchorTime:    anchor.CreatedAt,
		Until:         until,
		Reason:        suppressionReasonCooldown,
	}
}

func validateCooldownMinutes(m int) error {
	if m < 0 {
		return fmt.Errorf("cooldown_minutes must be >= 0")
	}
	if m > maxCooldownMinutes {
		return fmt.Errorf("cooldown_minutes must be <= %d", maxCooldownMinutes)
	}
	return nil
}
