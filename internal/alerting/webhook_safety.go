package alerting

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// Webhook delivery is an SSRF surface: rule/routing webhook URLs are operator-supplied and
// POSTed to verbatim. These helpers constrain delivery to https endpoints that do not target
// internal/metadata hosts, block redirects, and validate the *resolved* connection IP at dial
// time (defeating DNS-rebinding to a private address).

// allowedWebhookURL reports whether a webhook URL is safe to send to: https scheme, a real
// host, not localhost, and not an internal IP literal (loopback/private/link-local — which
// includes the cloud metadata endpoint 169.254.169.254).
func allowedWebhookURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return false
	}
	host := u.Hostname()
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil && disallowedIP(ip) {
		return false
	}
	return true
}

func disallowedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified()
}

// newSafeWebhookClient builds an HTTP client that refuses redirects and rejects connections to
// internal IPs at dial time (so a hostname that resolves to a private/metadata address fails).
func newSafeWebhookClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout: timeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				host = address
			}
			if ip := net.ParseIP(host); ip != nil && disallowedIP(ip) {
				return fmt.Errorf("blocked connection to internal address %s", host)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   timeout,
			ResponseHeaderTimeout: timeout,
		},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse // do not follow redirects (avoid bouncing to internal hosts)
		},
	}
}
