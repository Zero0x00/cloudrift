package validators

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cloudrift/internal/models"
)

type fakeResolver struct {
	err error
}

func (f fakeResolver) LookupHost(_ context.Context, _ string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []string{"127.0.0.1"}, nil
}

type failHeadTransport struct {
	next http.RoundTripper
}

func (t failHeadTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodHead {
		return nil, errors.New("head blocked")
	}
	return t.next.RoundTrip(req)
}

func TestFingerprint(t *testing.T) {
	if got := fingerprint(404, "", "<Code>NoSuchBucket</Code>"); got != "s3_bucket_deleted" {
		t.Fatalf("unexpected fingerprint: %s", got)
	}
	if got := fingerprint(403, "AmazonS3", ""); got != "s3_bucket_exists_private" {
		t.Fatalf("unexpected fingerprint: %s", got)
	}
}

func TestValidateAssets_HTTPFallbackAndFingerprint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<Code>NoSuchBucket</Code>"))
	}))
	defer srv.Close()

	node := models.AssetNode{
		ARN:  "arn:test",
		Name: "example.com",
		Properties: map[string]any{
			"probe_url": srv.URL,
		},
	}
	client := &http.Client{
		Timeout:   2 * time.Second,
		Transport: failHeadTransport{next: http.DefaultTransport},
	}
	got := validateAssetsWithClients(context.Background(), []models.AssetNode{node}, 5, false, 2*time.Second, "cloudrift-test", fakeResolver{}, client, &net.Dialer{Timeout: time.Second})
	res := got["arn:test"]
	if res.DNSStatus != "resolved" {
		t.Fatalf("expected resolved dns, got %s", res.DNSStatus)
	}
	if res.ErrorFingerprint != "s3_bucket_deleted" {
		t.Fatalf("expected s3_bucket_deleted, got %s", res.ErrorFingerprint)
	}
}

func TestValidateAssets_NoHTTPSkipsProbing(t *testing.T) {
	node := models.AssetNode{ARN: "arn:nohttp", Name: "example.com"}
	got := validateAssetsWithClients(context.Background(), []models.AssetNode{node}, 2, true, time.Second, "ua", fakeResolver{}, &http.Client{}, &net.Dialer{})
	res := got["arn:nohttp"]
	if res.DNSStatus != "resolved" {
		t.Fatalf("expected resolved, got %s", res.DNSStatus)
	}
	if res.HTTPStatus != 0 || res.BodySnippet != "" {
		t.Fatalf("expected no http probing result")
	}
}

func TestValidateAssets_DNSFailureModes(t *testing.T) {
	node := models.AssetNode{ARN: "arn:dns", Name: "bad.example"}
	nx := validateAssetsWithClients(context.Background(), []models.AssetNode{node}, 1, false, time.Second, "ua", fakeResolver{err: dnsNotFoundErr()}, &http.Client{}, &net.Dialer{})["arn:dns"]
	if nx.DNSStatus != "nxdomain" {
		t.Fatalf("expected nxdomain, got %s", nx.DNSStatus)
	}
	timeout := validateAssetsWithClients(context.Background(), []models.AssetNode{node}, 1, false, time.Second, "ua", fakeResolver{err: dnsTimeoutErr()}, &http.Client{}, &net.Dialer{})["arn:dns"]
	if timeout.DNSStatus != "timeout" {
		t.Fatalf("expected timeout, got %s", timeout.DNSStatus)
	}
	servfail := validateAssetsWithClients(context.Background(), []models.AssetNode{node}, 1, false, time.Second, "ua", fakeResolver{err: errors.New("boom")}, &http.Client{}, &net.Dialer{})["arn:dns"]
	if servfail.DNSStatus != "servfail" {
		t.Fatalf("expected servfail, got %s", servfail.DNSStatus)
	}
}

func TestTargetForProbeOverride(t *testing.T) {
	node := models.AssetNode{
		Name: "ignored.example",
		Properties: map[string]any{
			"probe_url": "http://localhost:9000/p",
		},
	}
	host, target, scheme := targetForProbe(node)
	if host != "localhost" || scheme != "http" || !strings.Contains(target, "localhost:9000") {
		t.Fatalf("unexpected target parse host=%s scheme=%s target=%s", host, scheme, target)
	}
}
