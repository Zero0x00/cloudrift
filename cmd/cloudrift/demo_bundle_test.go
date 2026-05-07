package main

import "testing"

func TestFindingsForScan_demoBundleSize(t *testing.T) {
	got := findingsForScan("demo")
	if len(got) != 18 {
		t.Fatalf("bundled demo: want 18 findings, got %d", len(got))
	}
	for _, f := range got {
		if f.ScanID != "demo" {
			t.Fatalf("finding %q: want scan_id demo, got %q", f.ID, f.ScanID)
		}
	}
}

func TestFindingsForScan_timestampedUsesProgrammatic(t *testing.T) {
	got := findingsForScan("demo-20260101T000000Z")
	if len(got) != 28 {
		t.Fatalf("programmatic demo: want 28 findings, got %d", len(got))
	}
}
