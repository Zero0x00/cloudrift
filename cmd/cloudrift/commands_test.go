package main

import "testing"

func TestRootCommandContainsFoundationCommandsOnly(t *testing.T) {
	root := newRootCommand()
	commands := root.Commands()
	got := map[string]bool{}
	for _, cmd := range commands {
		got[cmd.Name()] = true
	}

	for _, required := range []string{"scan", "report", "query", "version", "dashboard"} {
		if !got[required] {
			t.Fatalf("missing required command %q", required)
		}
	}
	for _, forbidden := range []string{"diff", "remediate"} {
		if got[forbidden] {
			t.Fatalf("unexpected command %q", forbidden)
		}
	}

	dash, _, err := root.Find([]string{"dashboard"})
	if err != nil || dash == nil {
		t.Fatalf("dashboard command: %v", err)
	}
	for _, deprecated := range []string{"addr", "dev", "vite-url", "dashboard-dir"} {
		if dash.Flags().Lookup(deprecated) != nil {
			t.Fatalf("deprecated dashboard flag %q should not be registered", deprecated)
		}
	}
}

func TestScanCommandRegistersNeo4jFlag(t *testing.T) {
	root := newRootCommand()
	scan, _, err := root.Find([]string{"scan"})
	if err != nil || scan == nil {
		t.Fatalf("scan command: %v", err)
	}
	if scan.Flags().Lookup("neo4j") == nil {
		t.Fatal("expected scan to register --neo4j flag")
	}
}
