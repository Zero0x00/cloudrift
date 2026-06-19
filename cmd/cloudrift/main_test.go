package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Zero0x00/cloudrift/internal/config"
	"github.com/Zero0x00/cloudrift/internal/models"
	"github.com/Zero0x00/cloudrift/internal/pipeline"
)

func TestRunScanAndReport(t *testing.T) {
	dir := t.TempDir()
	// Exercise the scan→report wiring via the pipeline with a no-AWS source.
	if _, err := pipeline.Run(context.TODO(), config.Default(), dir, "test", fakeScanSource{}, pipeline.Options{}); err != nil {
		t.Fatal(err)
	}
	if err := runReport(dir, "latest", "table", ""); err != nil {
		t.Fatal(err)
	}
}

func TestRunReportJSONWritesFile(t *testing.T) {
	dir := t.TempDir()
	scanID := "testscan"
	path := filepath.Join(dir, scanID)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	f := []models.Finding{{ID: "abc", Title: "T", AccountID: "1", Hostname: "a.example.com"}}
	if err := writeFindings(filepath.Join(path, "findings.json"), f); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(path, "report.json")
	if err := runReport(dir, scanID, "json", reportPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("expected report file: %v", err)
	}
}

func TestResolveScanID_RejectsUnsafePaths(t *testing.T) {
	dir := t.TempDir()
	_, err := resolveScanID(dir, "../escape")
	if err == nil {
		t.Fatal("expected error for unsafe scan id")
	}
	if !strings.Contains(err.Error(), "invalid scan id") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := resolveScanID(dir, "ok-scan_1.2"); err != nil {
		t.Fatalf("expected safe id: %v", err)
	}
}
