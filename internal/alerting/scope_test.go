package alerting

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Zero0x00/cloudrift/internal/models"
)

func TestRuleAppliesToScan(t *testing.T) {
	rule := AlertRule{
		Scope: RuleScope{ScanIDs: []string{"  scan-a ", "scan-b"}},
	}
	if !RuleAppliesToScan(rule, "scan-a") {
		t.Fatal("expected scan-a to match")
	}
	if !RuleAppliesToScan(rule, "scan-b") {
		t.Fatal("expected scan-b to match")
	}
	if RuleAppliesToScan(rule, "scan-c") {
		t.Fatal("expected scan-c to be excluded")
	}
	empty := AlertRule{}
	if !RuleAppliesToScan(empty, "anything") {
		t.Fatal("empty scope should allow all scans")
	}
}

func TestFilterFindingsByAccountScope(t *testing.T) {
	rule := AlertRule{
		Scope: RuleScope{AccountIDs: []string{"111", "222"}},
	}
	in := []models.Finding{
		{AccountID: "111", Title: "a"},
		{AccountID: "333", Title: "b"},
		{AccountID: " 222 ", Title: "c"},
	}
	out := FilterFindingsByAccountScope(rule, in)
	if len(out) != 2 {
		t.Fatalf("want 2 findings, got %d", len(out))
	}
	all := AlertRule{}
	if len(FilterFindingsByAccountScope(all, in)) != 3 {
		t.Fatal("empty account scope should keep all")
	}
}

func TestNormalizeScopeIDs(t *testing.T) {
	got := normalizeScopeIDs([]string{" a ", "", "b"})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unexpected %v", got)
	}
	if normalizeScopeIDs(nil) != nil {
		t.Fatal("nil in should be nil out")
	}
}

func TestAlertRuleJSONDeliveryFields(t *testing.T) {
	tm := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	ok := true
	r := AlertRule{
		ID:                "r1",
		Name:              "n",
		Type:              RuleScanCompletion,
		Enabled:           true,
		Channel:           Channel{Type: ChannelSlackWebhook, SlackWebhookURL: "https://example"},
		CreatedAt:         tm,
		UpdatedAt:         tm,
		LastDeliveryAt:    &tm,
		LastDeliveryOK:    &ok,
		LastDeliveryError: "",
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var back AlertRule
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.LastDeliveryError != "" {
		t.Fatal("expected empty error string")
	}
	if back.LastDeliveryOK == nil || !*back.LastDeliveryOK {
		t.Fatal("expected last_delivery_ok true")
	}
}
