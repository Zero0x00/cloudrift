package alerting

import "testing"

func TestAllowedWebhookURL(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		// Allowed: https to public hosts.
		{"https://hooks.slack.com/services/T/B/X", true},
		{"https://example.com/hook", true},
		{"https://example", true},
		// Rejected: wrong scheme.
		{"http://hooks.slack.com/services/T/B/X", false},
		{"http://insecure.example/hook", false},
		{"ftp://hooks.slack.com/x", false},
		// Rejected: internal / metadata / loopback targets (SSRF).
		{"https://127.0.0.1/x", false},
		{"https://localhost/x", false},
		{"https://169.254.169.254/latest/meta-data/", false}, // cloud metadata endpoint
		{"https://10.0.0.5/x", false},
		{"https://192.168.1.10/x", false},
		{"https://172.16.0.1/x", false},
		{"https://[::1]/x", false},
		{"https://0.0.0.0/x", false},
		// Rejected: malformed / empty.
		{"", false},
		{"   ", false},
		{"https://", false},
	}
	for _, tc := range cases {
		if got := allowedWebhookURL(tc.url); got != tc.want {
			t.Errorf("allowedWebhookURL(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}
