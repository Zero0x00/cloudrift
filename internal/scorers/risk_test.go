package scorers

import (
	"testing"

	"github.com/Zero0x00/cloudrift/internal/models"
	"github.com/Zero0x00/cloudrift/internal/validators"
)

func TestScoreRisk_DeletedS3IsReclaimable(t *testing.T) {
	node := models.AssetNode{
		ARN:       "arn:1",
		AssetType: models.AssetDNSRecord,
		Name:      "app.example.com",
		AccountID: "1111",
		Properties: map[string]any{
			"value": "missing-bucket.s3-website-us-east-1.amazonaws.com",
		},
	}
	result := validators.ValidationResult{DNSStatus: "resolved", ErrorFingerprint: "s3_bucket_deleted"}
	f := ScoreRisk(node, result, map[string]bool{})
	if f.Claimability != models.ClaimReclaimable || f.Severity != models.SeverityCritical {
		t.Fatalf("expected reclaimable/critical, got %s/%s", f.Claimability, f.Severity)
	}
}

func TestScoreRisk_BucketExistsFallsBackToDangling(t *testing.T) {
	node := models.AssetNode{
		ARN:       "arn:2",
		AssetType: models.AssetDNSRecord,
		Name:      "app.example.com",
		AccountID: "1111",
		Properties: map[string]any{
			"value": "existing-bucket.s3-website-us-east-1.amazonaws.com",
		},
	}
	result := validators.ValidationResult{DNSStatus: "resolved", ErrorFingerprint: "s3_bucket_deleted"}
	f := ScoreRisk(node, result, map[string]bool{"existing-bucket": true})
	if f.Claimability != models.ClaimDangling || f.Severity != models.SeverityHigh {
		t.Fatalf("expected dangling/high, got %s/%s", f.Claimability, f.Severity)
	}
}

func TestScoreRisk_DanglingFromAWSControlledEndpoint(t *testing.T) {
	node := models.AssetNode{
		ARN:       "arn:3",
		AssetType: models.AssetDNSRecord,
		Name:      "cdn.example.com",
		AccountID: "2222",
		Properties: map[string]any{
			"value": "d111.cloudfront.net",
		},
	}
	result := validators.ValidationResult{DNSStatus: "resolved", ErrorFingerprint: "cloudfront_origin_error"}
	f := ScoreRisk(node, result, map[string]bool{})
	if f.Claimability != models.ClaimDangling || f.Severity != models.SeverityHigh {
		t.Fatalf("expected dangling/high, got %s/%s", f.Claimability, f.Severity)
	}
}

func TestScoreRisk_EdgeObscuredWhenCloudFrontAliasNotConfigured(t *testing.T) {
	node := models.AssetNode{
		ARN:       "arn:4",
		AssetType: models.AssetDNSRecord,
		Name:      "static.example.com",
		AccountID: "3333",
		Properties: map[string]any{
			"value":                "d222.cloudfront.net",
			"target_service":       "cloudfront",
			"in_alternate_domains": false,
		},
	}
	result := validators.ValidationResult{DNSStatus: "resolved", CDNDetected: true, CDNVendor: "cloudfront"}
	f := ScoreRisk(node, result, map[string]bool{})
	if f.Claimability != models.ClaimEdgeObscured || f.Severity != models.SeverityMedium {
		t.Fatalf("expected edge_obscured/medium, got %s/%s", f.Claimability, f.Severity)
	}
}

func TestScoreRisk_BrokenForNXDomain(t *testing.T) {
	node := models.AssetNode{
		ARN:       "arn:5",
		AssetType: models.AssetDNSRecord,
		Name:      "broken.example.com",
		AccountID: "4444",
	}
	result := validators.ValidationResult{DNSStatus: "nxdomain", ErrorFingerprint: "dns_nxdomain"}
	f := ScoreRisk(node, result, map[string]bool{})
	if f.Claimability != models.ClaimBroken || f.Severity != models.SeverityLow {
		t.Fatalf("expected broken/low, got %s/%s", f.Claimability, f.Severity)
	}
}

func TestScoreRisk_UnknownWhenNoSignals(t *testing.T) {
	node := models.AssetNode{
		ARN:       "arn:6",
		AssetType: models.AssetDNSRecord,
		Name:      "unknown.example.com",
		AccountID: "5555",
	}
	result := validators.ValidationResult{DNSStatus: "resolved"}
	f := ScoreRisk(node, result, map[string]bool{})
	if f.Claimability != models.ClaimUnknown || f.Severity != models.SeverityInfo {
		t.Fatalf("expected unknown/info, got %s/%s", f.Claimability, f.Severity)
	}
}
