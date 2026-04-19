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

	"cloudrift/internal/config"
	"cloudrift/internal/models"
)

type ValidationResult struct {
	DNSStatus        string `json:"dns_status"`
	HTTPStatus       int    `json:"http_status"`
	TLSValid         bool   `json:"tls_valid"`
	CDNDetected      bool   `json:"cdn_detected"`
	CDNVendor        string `json:"cdn_vendor"`
	ErrorFingerprint string `json:"error_fingerprint"`
	BodySnippet      string `json:"body_snippet"`
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
	res.ErrorFingerprint = fingerprint(resp.StatusCode, resp.Header.Get("Server"), res.BodySnippet)
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

func fingerprint(status int, server, body string) string {
	switch {
	case strings.Contains(body, "<Code>NoSuchBucket</Code>"):
		return "s3_bucket_deleted"
	case status == 403 && strings.Contains(strings.ToLower(server), "s3"):
		return "s3_bucket_exists_private"
	case strings.Contains(body, "The request could not be satisfied"):
		return "cloudfront_origin_error"
	case strings.Contains(body, "<Code>InvalidClientTokenId</Code>"):
		return "aws_endpoint_controlled"
	default:
		return ""
	}
}

func dnsTimeoutErr() error {
	return &net.DNSError{IsTimeout: true}
}

func dnsNotFoundErr() error {
	return &net.DNSError{IsNotFound: true}
}

