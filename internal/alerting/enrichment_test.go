package alerting

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Zero0x00/cloudrift/internal/api/schema"
	"github.com/Zero0x00/cloudrift/internal/blastradius"
	"github.com/Zero0x00/cloudrift/internal/models"
)

type fakeBlastProvider struct {
	summaries map[string]schema.BlastRadiusSummary
}

func (f fakeBlastProvider) FindingBlastSummary(_ context.Context, _ string, findingID string, _ blastradius.BlastMode) (schema.BlastRadiusSummary, error) {
	if s, ok := f.summaries[findingID]; ok {
		return s, nil
	}
	return schema.BlastRadiusSummary{}, nil
}

func TestEvaluatorNewCritical_addsBlastEnrichment(t *testing.T) {
	dir := t.TempDir()
	writeScanBundle(t, dir, "scan-old", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), []models.Finding{
		{ID: "f-old", Title: "legacy high", Severity: models.SeverityHigh, AccountID: "111111111111", AffectedARN: "arn:aws:iam::111111111111:role/Old"},
	})
	writeScanBundle(t, dir, "scan-new", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), []models.Finding{
		{ID: "f-new", Title: "new critical role trust", Severity: models.SeverityCritical, AccountID: "111111111111", AffectedARN: "arn:aws:iam::111111111111:role/New"},
	})
	ev := NewEvaluator(dir, "http://127.0.0.1:8080", fakeBlastProvider{
		summaries: map[string]schema.BlastRadiusSummary{
			"f-new": {
				GraphAvailable:         true,
				ReachableResourceCount: 14,
				ReachableAccountsCount: 3,
				EscalationPossible:     true,
				TopImpactedAccounts:    []string{"prod"},
				DominantMotif:          "CROSS_ACCOUNT_ASSUME_ROLE",
				RecommendedActionLabel: "Review cross-account trust path",
				SummaryText:            "Cross-account trust pivot detected",
			},
		},
	})
	rule := AlertRule{
		ID:      "r1",
		Name:    "new critical",
		Type:    RuleNewCriticalFindings,
		Enabled: true,
		Channel: Channel{Type: ChannelSlackWebhook, SlackWebhookURL: "https://hooks.slack.com/services/x/y/z"},
	}
	res, err := ev.Evaluate(rule, "scan-new")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Triggered {
		t.Fatalf("expected trigger for new critical")
	}
	if res.Context.BlastSummary == nil {
		t.Fatalf("expected blast enrichment summary")
	}
	if len(res.Context.Payload.Bullets) == 0 || !strings.Contains(strings.ToLower(strings.Join(res.Context.Payload.Bullets, " ")), "can reach") {
		t.Fatalf("expected blast-enriched bullet, got %#v", res.Context.Payload.Bullets)
	}
	if !strings.EqualFold(res.Context.Payload.ActionLabel, "Review cross-account trust path") {
		t.Fatalf("expected action label from blast summary, got %q", res.Context.Payload.ActionLabel)
	}
	if !strings.EqualFold(res.Context.BlastSummary.DominantMotif, "CROSS_ACCOUNT_ASSUME_ROLE") {
		t.Fatalf("expected structured motif propagation, got %q", res.Context.BlastSummary.DominantMotif)
	}
}

func TestDominantMotifFromSummary_prefersStructuredField(t *testing.T) {
	sum := schema.BlastRadiusSummary{
		DominantMotif:          "IAM_WRITE",
		RecommendedActionLabel: "Some unrelated wording",
		SummaryText:            "narrative text changed and does not mention trust motifs",
	}
	if got := dominantMotifFromSummary(sum); got != "IAM_WRITE" {
		t.Fatalf("expected structured motif preference, got %q", got)
	}
}

func TestEvaluatorBlastEnrichment_skipsWhenGraphUnavailable(t *testing.T) {
	dir := t.TempDir()
	writeScanBundle(t, dir, "scan-single", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), []models.Finding{
		{ID: "f-reclaim", Title: "idle role", Severity: models.SeverityHigh, Claimability: models.ClaimReclaimable, MonthlyRiskCost: 120, AccountID: "111111111111", AffectedARN: "arn:aws:iam::111111111111:role/Idle"},
	})
	ev := NewEvaluator(dir, "http://127.0.0.1:8080", fakeBlastProvider{
		summaries: map[string]schema.BlastRadiusSummary{
			"f-reclaim": {GraphAvailable: false},
		},
	})
	rule := AlertRule{
		ID:        "r2",
		Name:      "reclaimable",
		Type:      RuleReclaimableThreshold,
		Enabled:   true,
		Threshold: Threshold{CountMin: 1},
		Channel:   Channel{Type: ChannelSlackWebhook, SlackWebhookURL: "https://hooks.slack.com/services/x/y/z"},
	}
	res, err := ev.Evaluate(rule, "scan-single")
	if err != nil {
		t.Fatal(err)
	}
	if res.Context.BlastSummary != nil {
		t.Fatalf("expected no blast summary when graph unavailable")
	}
	if v, ok := res.Context.Metadata["blast_graph_available"].(bool); !ok || v {
		t.Fatalf("expected blast_graph_available=false metadata, got %#v", res.Context.Metadata["blast_graph_available"])
	}
}

func TestFormatSlackBlocks_limitsBullets(t *testing.T) {
	blocks := formatSlackBlocks(AlertPayload{
		Title:    "x",
		Severity: SeverityCritical,
		Summary:  "y",
		Bullets:  []string{"a", "b", "c", "d"},
	})
	raw, err := json.Marshal(blocks)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), "• ") != 3 {
		t.Fatalf("expected only 3 bullet lines in slack blocks, got: %s", string(raw))
	}
}

func writeScanBundle(t *testing.T, outputDir, scanID string, ts time.Time, findings []models.Finding) {
	t.Helper()
	scanDir := filepath.Join(outputDir, scanID)
	if err := os.MkdirAll(scanDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := models.ScanSnapshot{
		ScanID:       scanID,
		Timestamp:    ts,
		FindingCount: len(findings),
	}
	metaRaw, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(scanDir, "scan-metadata.json"), metaRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	findingsRaw, _ := json.Marshal(findings)
	if err := os.WriteFile(filepath.Join(scanDir, "findings.json"), findingsRaw, 0o644); err != nil {
		t.Fatal(err)
	}
}
