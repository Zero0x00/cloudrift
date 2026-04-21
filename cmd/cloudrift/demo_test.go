package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cloudrift/internal/graph"
	"cloudrift/internal/models"
)

func TestGenerateDemoScan_WritesExpectedArtifacts(t *testing.T) {
	root := t.TempDir()
	scanPath, err := generateDemoScan(root, time.Date(2026, 4, 19, 12, 34, 56, 0, time.UTC), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(scanPath, "demo-20260419T123456Z") {
		t.Fatalf("unexpected scan directory name: %s", scanPath)
	}

	requiredPaths := []string{
		filepath.Join(scanPath, "scan-metadata.json"),
		filepath.Join(scanPath, "findings.json"),
		filepath.Join(scanPath, "relationships.json"),
		filepath.Join(scanPath, "assets", "edge.json"),
		filepath.Join(scanPath, "assets", "iam_and_external.json"),
		filepath.Join(scanPath, "assets", "infrastructure_core.json"),
	}
	for _, p := range requiredPaths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected %s: %v", p, err)
		}
	}

	var meta models.ScanSnapshot
	readJSON(t, filepath.Join(scanPath, "scan-metadata.json"), &meta)
	var findings []models.Finding
	readJSON(t, filepath.Join(scanPath, "findings.json"), &findings)

	if got, want := meta.FindingCount, len(findings); got != want {
		t.Fatalf("finding_count mismatch: got %d want %d", got, want)
	}
	critical := 0
	high := 0
	for _, f := range findings {
		if f.Severity == models.SeverityCritical {
			critical++
		}
		if f.Severity == models.SeverityHigh {
			high++
		}
	}
	if meta.CriticalCount != critical {
		t.Fatalf("critical_count mismatch: got %d want %d", meta.CriticalCount, critical)
	}
	if meta.HighCount != high {
		t.Fatalf("high_count mismatch: got %d want %d", meta.HighCount, high)
	}
}

func TestGenerateDemoScan_CompatibleWithGraphLoader(t *testing.T) {
	root := t.TempDir()
	scanPath, err := generateDemoScan(root, time.Date(2026, 4, 19, 1, 2, 3, 0, time.UTC), "")
	if err != nil {
		t.Fatal(err)
	}

	meta, assets, rels, findings, err := loadScanArtifactsForGraph(scanPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) == 0 || len(rels) == 0 || len(findings) == 0 {
		t.Fatalf("expected non-empty graph artifacts, got assets=%d rels=%d findings=%d", len(assets), len(rels), len(findings))
	}

	foundTrusts := false
	for _, rel := range rels {
		if rel.RelType == models.RelTrusts {
			foundTrusts = true
			break
		}
	}
	if !foundTrusts {
		t.Fatal("expected at least one TRUSTS relationship")
	}

	stmts := graph.CompileWriteScan(meta, assets, rels, findings)
	if len(stmts) == 0 {
		t.Fatal("expected non-empty graph statements")
	}
}

func TestDemoCommand_RegistersGenerateSubcommand(t *testing.T) {
	root := newRootCommand()
	demo, _, err := root.Find([]string{"demo"})
	if err != nil || demo == nil {
		t.Fatalf("demo command not found: %v", err)
	}
	generate, _, err := root.Find([]string{"demo", "generate"})
	if err != nil || generate == nil {
		t.Fatalf("demo generate command not found: %v", err)
	}
	if generate.Flags().Lookup("neo4j") == nil {
		t.Fatal("expected demo generate to have --neo4j flag")
	}
	if generate.Flags().Lookup("scan-id") == nil {
		t.Fatal("expected demo generate to have --scan-id flag")
	}
}

func readJSON(t *testing.T, path string, out any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
}
