package remediator

import (
	"strings"
	"testing"

	"cloudrift/internal/models"
)

func TestBuildRemediation_Route53(t *testing.T) {
	f := models.Finding{
		ID:             "abc123",
		Title:          "dangling",
		Severity:       models.SeverityHigh,
		AccountID:      "1111",
		Hostname:       "app.example.com",
		Claimability:   models.ClaimDangling,
		AffectedARN:    "arn:aws:route53:::hostedzone/Z1ABC/app.example.com",
		Recommendation: "remove record",
		Impact:         "possible takeover",
		Evidence:       map[string]any{"fingerprint": "s3_bucket_deleted"},
	}
	cmd, md, payload := BuildRemediation(f)
	if !strings.Contains(cmd, "route53") {
		t.Fatalf("expected route53 command, got %s", cmd)
	}
	if !strings.Contains(md, "possible takeover") {
		t.Fatalf("expected impact in markdown")
	}
	if !strings.Contains(string(payload), "\"claimability\"") {
		t.Fatalf("expected claimability in payload")
	}
}

func TestBuildRemediation_CloudFront(t *testing.T) {
	f := models.Finding{AffectedARN: "arn:aws:cloudfront::1111:distribution/ABC"}
	cmd, _, _ := BuildRemediation(f)
	if !strings.Contains(strings.ToLower(cmd), "cloudfront") {
		t.Fatalf("expected cloudfront command, got %s", cmd)
	}
}
