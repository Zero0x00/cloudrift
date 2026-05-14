package queryv2

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Zero0x00/cloudrift/internal/models"
)

func TestBuildPlanRoutesIntent(t *testing.T) {
	p := buildPlan(QueryRequest{Query: "What is the blast radius of this finding?"}, "scan-a")
	if p.Intent != IntentBlastRadius || !p.NeedsGraph {
		t.Fatalf("unexpected plan: %+v", p)
	}
}

func TestServicePrioritizeFixesDomainOnly(t *testing.T) {
	outDir := t.TempDir()
	scanID := "scan-a"
	scanPath := filepath.Join(outDir, scanID)
	if err := os.MkdirAll(scanPath, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := models.ScanSnapshot{ScanID: scanID, Timestamp: time.Now().UTC()}
	metaB, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(scanPath, "scan-metadata.json"), metaB, 0o644); err != nil {
		t.Fatal(err)
	}
	findings := []models.Finding{
		{ID: "f1", Severity: models.SeverityCritical, MonthlyRiskCost: 1000, Recommendation: "fix f1"},
		{ID: "f2", Severity: models.SeverityHigh, MonthlyRiskCost: 100, Recommendation: "fix f2"},
	}
	fb, _ := json.Marshal(findings)
	if err := os.WriteFile(filepath.Join(scanPath, "findings.json"), fb, 0o644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(outDir, nil, nil)
	resp, err := svc.Execute(context.Background(), QueryRequest{
		Query:  "what should i fix first",
		ScanID: scanID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Intent != IntentPrioritizeFixes {
		t.Fatalf("intent=%s", resp.Intent)
	}
	if len(resp.RecommendedAction) == 0 {
		t.Fatal("expected recommended actions")
	}
	if resp.GraphUsed {
		t.Fatal("expected graph to be unused in domain-only mode")
	}
}
