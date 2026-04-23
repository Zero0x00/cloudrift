package alerting

import (
	"os"
	"path/filepath"
	"testing"
)

func writeMinimalScan(t *testing.T, outputDir, scanID string) {
	t.Helper()
	scanDir := filepath.Join(outputDir, scanID)
	if err := os.MkdirAll(scanDir, 0o755); err != nil {
		t.Fatal(err)
	}
	findingsPath := filepath.Join(scanDir, "findings.json")
	if err := os.WriteFile(findingsPath, []byte(`[
	  {"id":"f1","title":"x","severity":"high","account_id":"111111111111","affected_arn":"arn:aws:iam::111111111111:user/x"}
	]`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEvaluator_ScanScopeSkipsDisallowedScan(t *testing.T) {
	dir := t.TempDir()
	writeMinimalScan(t, dir, "scan-allowed")
	writeMinimalScan(t, dir, "scan-other")

	ev := NewEvaluator(dir, "http://127.0.0.1:8080")
	rule := AlertRule{
		ID:   "r1",
		Name: "scoped",
		Type: RuleScanCompletion,
		Scope: RuleScope{
			ScanIDs: []string{"scan-allowed"},
		},
		Channel: Channel{Type: ChannelSlackWebhook, SlackWebhookURL: "https://hooks.slack.com/services/x/y/z"},
	}

	res, err := ev.Evaluate(rule, "scan-other")
	if err != nil {
		t.Fatal(err)
	}
	if res.Triggered {
		t.Fatal("expected non-triggering scope skip")
	}
	if res.Context.Metadata == nil || res.Context.Metadata["scope_scan_excluded"] != true {
		t.Fatalf("expected scope_scan_excluded metadata, got %#v", res.Context.Metadata)
	}
}

func TestEvaluator_AccountScopeFiltersFindings(t *testing.T) {
	dir := t.TempDir()
	writeMinimalScan(t, dir, "scan-1")

	ev := NewEvaluator(dir, "http://127.0.0.1:8080")
	rule := AlertRule{
		ID:   "r2",
		Name: "acct",
		Type: RuleReclaimableThreshold,
		Scope: RuleScope{
			AccountIDs: []string{"999999999999"},
		},
		Threshold: Threshold{CountMin: 1},
		Channel:   Channel{Type: ChannelSlackWebhook, SlackWebhookURL: "https://hooks.slack.com/services/x/y/z"},
	}

	res, err := ev.Evaluate(rule, "scan-1")
	if err != nil {
		t.Fatal(err)
	}
	if res.Triggered {
		t.Fatal("expected no trigger when scoped account has no findings")
	}
	v := res.Context.Metadata["findings_after_account_scope"]
	switch n := v.(type) {
	case int:
		if n != 0 {
			t.Fatalf("expected 0 scoped findings, got %d", n)
		}
	case float64:
		if n != 0 {
			t.Fatalf("expected 0 scoped findings, got %v", n)
		}
	default:
		t.Fatalf("unexpected findings_after_account_scope type %T val %#v", v, v)
	}
}
