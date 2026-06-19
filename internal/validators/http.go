package validators

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Zero0x00/cloudrift/internal/config"
	"github.com/Zero0x00/cloudrift/internal/models"
)

type ValidationResult struct {
	DNSStatus        string `json:"dns_status"`
	HTTPStatus       int    `json:"http_status"`
	TLSValid         bool   `json:"tls_valid"`
	CDNDetected      bool   `json:"cdn_detected"`
	CDNVendor        string `json:"cdn_vendor"`
	ErrorFingerprint string `json:"error_fingerprint"`
	// Claimable is true when the matched fingerprint indicates the backing name is
	// takeover-able (e.g. a deleted S3 bucket or an unclaimed third-party app/site),
	// as opposed to a merely misconfigured-but-AWS-controlled endpoint.
	Claimable   bool   `json:"claimable"`
	BodySnippet string `json:"body_snippet"`
}

type DNSResolver interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
}

func ValidateAssets(ctx context.Context, nodes []models.AssetNode, concurrency int, noHTTP bool, timeout time.Duration, userAgent string) map[string]ValidationResult {
	if concurrency < 1 {
		concurrency = 1
	}
	return validateAssetsWithClients(ctx, nodes, concurrency, noHTTP, timeout, userAgent, net.DefaultResolver, &http.Client{Timeout: timeout}, &net.Dialer{Timeout: timeout})
}

func ValidateAssetsWithConfig(ctx context.Context, cfg *config.Config, nodes []models.AssetNode, noHTTP bool) map[string]ValidationResult {
	return ValidateAssets(ctx, nodes, cfg.Scan.HTTPProbeConcurrency, noHTTP, time.Duration(cfg.Scan.HTTPTimeoutSeconds)*time.Second, cfg.Scan.UserAgent)
}

func validateAssetsWithClients(
	ctx context.Context,
	nodes []models.AssetNode,
	concurrency int,
	noHTTP bool,
	timeout time.Duration,
	userAgent string,
	resolver DNSResolver,
	client *http.Client,
	dialer *net.Dialer,
) map[string]ValidationResult {
	results := make(map[string]ValidationResult, len(nodes))
	sem := make(chan struct{}, concurrency)
	ch := make(chan struct {
		arn string
		res ValidationResult
	}, len(nodes))

	for _, n := range nodes {
		n := n
		go func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			res := validateNode(ctx, n, noHTTP, timeout, userAgent, resolver, client, dialer)
			ch <- struct {
				arn string
				res ValidationResult
			}{arn: n.ARN, res: res}
		}()
	}
	for range nodes {
		r := <-ch
		results[r.arn] = r.res
	}
	return results
}

func validateNode(
	ctx context.Context,
	node models.AssetNode,
	noHTTP bool,
	timeout time.Duration,
	userAgent string,
	resolver DNSResolver,
	client *http.Client,
	dialer *net.Dialer,
) ValidationResult {
	host, targetURL, scheme := targetForProbe(node)
	if host == "" {
		return ValidationResult{DNSStatus: "unknown"}
	}
	if _, err := resolver.LookupHost(ctx, host); err != nil {
		if dnsErr, ok := err.(*net.DNSError); ok {
			switch {
			case dnsErr.IsNotFound:
				return ValidationResult{DNSStatus: "nxdomain", ErrorFingerprint: "dns_nxdomain"}
			case dnsErr.IsTimeout:
				return ValidationResult{DNSStatus: "timeout"}
			}
		}
		return ValidationResult{DNSStatus: "servfail"}
	}
	res := ValidationResult{DNSStatus: "resolved"}
	if noHTTP {
		return res
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, targetURL, nil)
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		req, _ = http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		req.Header.Set("User-Agent", userAgent)
		resp, err = client.Do(req)
	}
	if err != nil {
		return res
	}
	defer resp.Body.Close()
	res.HTTPStatus = resp.StatusCode
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	res.BodySnippet = string(snippet)
	if sig, ok := matchTakeoverSignature(resp.StatusCode, resp.Header.Get("Server"), res.BodySnippet); ok {
		res.ErrorFingerprint = sig.ID
		res.Claimable = sig.Claimable
	}
	if strings.Contains(strings.ToLower(host), "cloudfront.net") {
		res.CDNDetected = true
		res.CDNVendor = "cloudfront"
	}
	if scheme == "https" {
		conn, err := tls.DialWithDialer(dialer, "tcp", host+":443", &tls.Config{ServerName: host})
		if err == nil {
			res.TLSValid = true
			_ = conn.Close()
		}
	}
	return res
}

func targetForProbe(node models.AssetNode) (host, targetURL, scheme string) {
	if raw, ok := node.Properties["probe_url"].(string); ok && raw != "" {
		if u, err := url.Parse(raw); err == nil && u.Host != "" {
			return u.Hostname(), raw, u.Scheme
		}
	}
	host = node.Name
	scheme = "https"
	if s, ok := node.Properties["scheme"].(string); ok && s != "" {
		scheme = strings.ToLower(s)
	}
	targetURL = scheme + "://" + host
	return host, targetURL, scheme
}

// TakeoverSignature is one entry in the extensible fingerprint catalog. A response
// matches when every specified condition holds (status, Server substring, and any one
// of the body substrings). Claimable distinguishes a takeover-able backing name from a
// merely misconfigured-but-controlled endpoint.
type TakeoverSignature struct {
	ID         string
	Service    string
	Status     int      // 0 = match any status
	ServerHas  string   // lowercased substring required in the Server header (empty = ignore)
	BodyHasAny []string // response matches if the body contains ANY of these (empty = ignore)
	Claimable  bool
}

// takeoverSignatures is the ordered fingerprint catalog. First match wins, so place
// more specific signatures before broader ones. Add new edge/third-party takeover
// surfaces here — detection coverage scales by extending this table.
var takeoverSignatures = []TakeoverSignature{
	// AWS-native
	{ID: "s3_bucket_deleted", Service: "s3", BodyHasAny: []string{"<Code>NoSuchBucket</Code>"}, Claimable: true},
	{ID: "s3_bucket_exists_private", Service: "s3", Status: 403, ServerHas: "s3", Claimable: false},
	{ID: "cloudfront_origin_error", Service: "cloudfront", BodyHasAny: []string{"The request could not be satisfied"}, Claimable: false},
	{ID: "aws_endpoint_controlled", Service: "aws", BodyHasAny: []string{"<Code>InvalidClientTokenId</Code>"}, Claimable: false},
	{ID: "apigateway_missing_mapping", Service: "apigateway", Status: 403, BodyHasAny: []string{`{"message":"Forbidden"}`}, Claimable: false},
	// Third-party CNAME takeover surfaces (org points docs.example.com at a SaaS via Route 53).
	// NOTE: body-only fingerprints can false-positive; a follow-up should pair these with the
	// record's CNAME target suffix (github.io, herokuapp.com, myshopify.com) to confirm.
	{ID: "github_pages_unclaimed", Service: "github_pages", BodyHasAny: []string{"There isn't a GitHub Pages site here"}, Claimable: true},
	{ID: "heroku_no_such_app", Service: "heroku", BodyHasAny: []string{"No such app", "no-such-app.html"}, Claimable: true},
	{ID: "shopify_unavailable", Service: "shopify", BodyHasAny: []string{"Sorry, this shop is currently unavailable"}, Claimable: true},
}

func matchTakeoverSignature(status int, server, body string) (TakeoverSignature, bool) {
	serverLower := strings.ToLower(server)
	for _, s := range takeoverSignatures {
		if s.Status != 0 && status != s.Status {
			continue
		}
		if s.ServerHas != "" && !strings.Contains(serverLower, s.ServerHas) {
			continue
		}
		if len(s.BodyHasAny) > 0 {
			matched := false
			for _, sub := range s.BodyHasAny {
				if strings.Contains(body, sub) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		return s, true
	}
	return TakeoverSignature{}, false
}

// fingerprint returns the matched signature ID (or "") — retained for the evidence map
// and for callers that only need the legacy string form.
func fingerprint(status int, server, body string) string {
	if s, ok := matchTakeoverSignature(status, server, body); ok {
		return s.ID
	}
	return ""
}

func dnsTimeoutErr() error {
	return &net.DNSError{IsTimeout: true}
}

func dnsNotFoundErr() error {
	return &net.DNSError{IsNotFound: true}
}

