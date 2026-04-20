package main

import (
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"cloudrift/dashboard"
	"cloudrift/internal/api"
	"cloudrift/internal/config"
)

// dashboardStart is swapped in tests to avoid binding a real listener.
var dashboardStart = api.StartServer

func newDashboardCommand(cfgPath *string) *cobra.Command {
	var (
		port        = 8080
		openBrowser bool
		scanID      string
		outputDir   string
	)

	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Serve Cloudrift dashboard (API + embedded UI)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if port < 1 || port > 65535 {
				return fmt.Errorf("invalid --port: must be between 1 and 65535")
			}
			if outputDir == "" {
				cfg, err := config.Load(*cfgPath)
				if err != nil {
					return err
				}
				outputDir = cfg.Output.OutputDir
			}
			staticFS, err := dashboardStaticFS()
			if err != nil {
				return err
			}
			if openBrowser {
				go func() {
					// Brief delay so the listener is usually up before the browser connects.
					time.Sleep(200 * time.Millisecond)
					tryOpenDashboard(port, scanID)
				}()
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Cloudrift dashboard listening on http://%s\n", net.JoinHostPort("0.0.0.0", strconv.Itoa(port)))
			if scanID != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Default scan context (browser open URL): scan_id=%q\n", scanID)
			}
			return dashboardStart(port, outputDir, *cfgPath, staticFS)
		},
	}
	cmd.Flags().IntVar(&port, "port", port, "HTTP listen port")
	cmd.Flags().BoolVar(&openBrowser, "open", false, "Open dashboard in the default browser after startup")
	cmd.Flags().StringVar(&scanID, "scan-id", "", "Optional default scan id (added to open URL as scan_id query param)")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Directory containing scan output (default: config output_dir)")
	return cmd
}

func dashboardStaticFS() (fs.FS, error) {
	// Subtree so URL paths match Vite output (/assets/..., index.html at root).
	return fs.Sub(dashboard.Dist, "dist")
}

func tryOpenDashboard(port int, scanID string) {
	u := url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
		Path:   "/",
	}
	if scanID != "" {
		q := u.Query()
		q.Set("scan_id", scanID)
		u.RawQuery = q.Encode()
	}
	_ = openURL(u.String())
}

func openURL(target string) error {
	u, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid dashboard URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("refusing to open non-http URL scheme %q", u.Scheme)
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return fmt.Errorf("dashboard URL host is empty")
	}
	ip := net.ParseIP(host)
	if ip != nil && !ip.IsLoopback() {
		return fmt.Errorf("refusing to open non-loopback URL host %q", host)
	}
	if ip == nil && !strings.EqualFold(host, "localhost") {
		return fmt.Errorf("refusing to open non-localhost URL host %q", host)
	}

	switch runtime.GOOS {
	case "darwin":
		// #nosec G204 -- command and argument vector are fixed; target was parsed and restricted above.
		return exec.Command("open", target).Start()
	case "windows":
		// #nosec G204 -- command and argument vector are fixed; target was parsed and restricted above.
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
	default:
		// #nosec G204 -- command and argument vector are fixed; target was parsed and restricted above.
		return exec.Command("xdg-open", target).Start()
	}
}
