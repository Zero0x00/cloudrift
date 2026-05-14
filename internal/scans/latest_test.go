package scans

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Zero0x00/cloudrift/internal/models"
)

func writeScanDir(t *testing.T, root, dirName string, meta models.ScanSnapshot, findings []models.Finding) {
	t.Helper()
	p := filepath.Join(root, dirName)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	mb, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "scan-metadata.json"), mb, 0o644); err != nil {
		t.Fatal(err)
	}
	fb, err := json.Marshal(findings)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "findings.json"), fb, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveLatestScanID_PrefersMetadataTimestampOverLexicalOrder(t *testing.T) {
	root := t.TempDir()
	older := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	// Lexicographically last would be "zzz"; timestamp says "aaa" is newer.
	writeScanDir(t, root, "zzz", models.ScanSnapshot{ScanID: "zzz", Timestamp: older}, nil)
	writeScanDir(t, root, "aaa", models.ScanSnapshot{ScanID: "aaa", Timestamp: newer}, nil)

	got, err := ResolveLatestScanID(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != "aaa" {
		t.Fatalf("latest: want directory aaa (newer metadata), got %q", got)
	}
}

func TestResolveLatestScanID_TieBreakDirectoryNameAscending(t *testing.T) {
	root := t.TempDir()
	ts := time.Date(2022, 3, 3, 0, 0, 0, 0, time.UTC)
	writeScanDir(t, root, "scan-b", models.ScanSnapshot{ScanID: "scan-b", Timestamp: ts}, nil)
	writeScanDir(t, root, "scan-a", models.ScanSnapshot{ScanID: "scan-a", Timestamp: ts}, nil)

	got, err := ResolveLatestScanID(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != "scan-a" {
		t.Fatalf("tie-break: want scan-a (ascending dir name), got %q", got)
	}
}

func TestResolveLatestScanID_SkipsMalformedDirectories(t *testing.T) {
	root := t.TempDir()
	writeScanDir(t, root, "good", models.ScanSnapshot{
		ScanID:    "good",
		Timestamp: time.Unix(100, 0).UTC(),
	}, nil)
	bad := filepath.Join(root, "bad-json")
	if err := os.MkdirAll(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "scan-metadata.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "findings.json"), []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}
	missingFindings := filepath.Join(root, "no-findings")
	if err := os.MkdirAll(missingFindings, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := models.ScanSnapshot{ScanID: "x", Timestamp: time.Unix(200, 0).UTC()}
	mb, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(missingFindings, "scan-metadata.json"), mb, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveLatestScanID(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != "good" {
		t.Fatalf("want only good scan, got %q", got)
	}
}

func TestResolveLatestScanIDWithLoader_UsesInjectedLoader(t *testing.T) {
	ids := []string{"x", "y"}
	loader := func(outputDir, dirName string) (*models.ScanSnapshot, []models.Finding, error) {
		switch dirName {
		case "x":
			return &models.ScanSnapshot{ScanID: "x", Timestamp: time.Unix(1, 0).UTC()}, nil, nil
		case "y":
			return &models.ScanSnapshot{ScanID: "y", Timestamp: time.Unix(2, 0).UTC()}, nil, nil
		default:
			return nil, nil, os.ErrNotExist
		}
	}
	got, err := resolveLatestScanIDWithLoader("ignored", ids, loader)
	if err != nil {
		t.Fatal(err)
	}
	if got != "y" {
		t.Fatalf("got %q", got)
	}
}
