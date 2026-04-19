package scans

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestResolveScanDirectoryName_LatestAndSafe(t *testing.T) {
	dir := t.TempDir()
	_, err := ResolveScanDirectoryName(dir, "latest")
	if err != nil {
		t.Fatalf("empty tree: %v", err)
	}
	got, err := ResolveScanDirectoryName(dir, "my-scan_1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "my-scan_1" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveScanDirectoryName_RejectsUnsafe(t *testing.T) {
	dir := t.TempDir()
	_, err := ResolveScanDirectoryName(dir, "../x")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid scan id") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestResolveScanDirectoryName_TrimsSpace(t *testing.T) {
	dir := t.TempDir()
	got, err := ResolveScanDirectoryName(dir, "  ab  ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "ab" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadScanArtifacts_RejectsUnsafeDirName(t *testing.T) {
	dir := t.TempDir()
	_, _, err := LoadScanArtifacts(dir, "..")
	if !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("expected os.ErrInvalid, got %v", err)
	}
}
