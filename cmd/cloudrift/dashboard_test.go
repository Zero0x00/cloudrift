package main

import (
	"io/fs"
	"testing"

	"github.com/spf13/cobra"
)

func TestDashboardCommandFlags(t *testing.T) {
	cfg := ""
	root := &cobra.Command{Use: "cloudrift"}
	root.AddCommand(newDashboardCommand(&cfg))
	dash, _, err := root.Find([]string{"dashboard"})
	if err != nil {
		t.Fatal(err)
	}
	if dash == nil {
		t.Fatal("dashboard command not found")
	}
	flags := dash.Flags()
	for _, name := range []string{"port", "open", "scan-id", "output-dir"} {
		if flags.Lookup(name) == nil {
			t.Fatalf("missing flag %q", name)
		}
	}
}

func TestDashboardCommandInvalidPort(t *testing.T) {
	cfg := ""
	cmd := newDashboardCommand(&cfg)
	if err := cmd.ParseFlags([]string{"--port", "0"}); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, nil); err == nil {
		t.Fatal("expected error for port 0")
	}
}

func TestDashboardCommandInvokesStartServer(t *testing.T) {
	cfg := ""
	var (
		gotPort     int
		gotOutput   string
		gotConfig   string
		gotStaticFS fs.FS
	)
	orig := dashboardStart
	dashboardStart = func(port int, outputDir, configPath string, staticFS fs.FS) error {
		gotPort = port
		gotOutput = outputDir
		gotConfig = configPath
		gotStaticFS = staticFS
		return nil
	}
	t.Cleanup(func() { dashboardStart = orig })

	cmd := newDashboardCommand(&cfg)
	if err := cmd.ParseFlags([]string{"--port", "9443", "--output-dir", "/tmp/cloudrift-out", "--scan-id", "demo"}); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if gotPort != 9443 {
		t.Fatalf("port: got %d", gotPort)
	}
	if gotOutput != "/tmp/cloudrift-out" {
		t.Fatalf("output dir: got %q", gotOutput)
	}
	if gotConfig != "" {
		t.Fatalf("expected empty config path by default, got %q", gotConfig)
	}
	if gotStaticFS == nil {
		t.Fatal("expected non-nil static fs")
	}
}

func TestDashboardStaticFSServesIndex(t *testing.T) {
	staticFS, err := dashboardStaticFS()
	if err != nil {
		t.Fatal(err)
	}
	b, err := fs.ReadFile(staticFS, "index.html")
	if err != nil {
		t.Fatalf("embedded index.html: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("empty index.html")
	}
}

func TestOpenFlagParses(t *testing.T) {
	cfg := ""
	cmd := newDashboardCommand(&cfg)
	if err := cmd.ParseFlags([]string{"--open"}); err != nil {
		t.Fatal(err)
	}
	v, err := cmd.Flags().GetBool("open")
	if err != nil || !v {
		t.Fatalf("open: %v err=%v", v, err)
	}
}

func TestScanIDFlagParses(t *testing.T) {
	cfg := ""
	cmd := newDashboardCommand(&cfg)
	if err := cmd.ParseFlags([]string{"--scan-id", "demo-123"}); err != nil {
		t.Fatal(err)
	}
	s, err := cmd.Flags().GetString("scan-id")
	if err != nil || s != "demo-123" {
		t.Fatalf("scan-id: %q err=%v", s, err)
	}
}

func TestOpenURLRejectsNonLocalhost(t *testing.T) {
	if err := openURL("http://example.com"); err == nil {
		t.Fatal("expected error for non-localhost URL")
	}
}
