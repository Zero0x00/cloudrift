package scorers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"cloudrift/internal/models"
	"cloudrift/internal/validators"
)

func ScoreRisk(node models.AssetNode, validation validators.ValidationResult, bucketNames map[string]bool) models.Finding {
	claimability, severity := classifyRisk(node, validation, bucketNames)

	title := fmt.Sprintf("%s -> %s", node.Name, claimability)
	h := sha256.Sum256([]byte(node.ARN + "|" + title))
	id := hex.EncodeToString(h[:])[:12]
	return models.Finding{
		ID:           id,
		Title:        title,
		Severity:     severity,
		Module:       models.ModuleOrphanedEdge,
		Claimability: claimability,
		AffectedARN:  node.ARN,
		AccountID:    node.AccountID,
		Hostname:     node.Name,
		Impact:       impactFor(claimability),
		Recommendation: recommendationFor(claimability),
		Evidence: map[string]any{
			"dns_status":  validation.DNSStatus,
			"http_status": validation.HTTPStatus,
			"fingerprint": validation.ErrorFingerprint,
			"bucket_name": bucketNameFromTarget(node),
		},
	}
}

func classifyRisk(node models.AssetNode, validation validators.ValidationResult, bucketNames map[string]bool) (models.Claimability, models.Severity) {
	// RECLAIMABLE: resolved + deleted S3 website target + bucket missing in all scanned accounts.
	if validation.DNSStatus == "resolved" &&
		validation.ErrorFingerprint == "s3_bucket_deleted" &&
		strings.EqualFold(targetService(node), "s3_website") {
		bucket := bucketNameFromTarget(node)
		if bucket != "" && !bucketNames[bucket] {
			return models.ClaimReclaimable, models.SeverityCritical
		}
		// Deleted fingerprint but bucket exists in scanned accounts means not reclaimable.
		return models.ClaimDangling, models.SeverityHigh
	}

	// DANGLING: resolved + AWS controlled error body, not reclaimable.
	if validation.DNSStatus == "resolved" && validation.ErrorFingerprint != "" {
		return models.ClaimDangling, models.SeverityHigh
	}

	// EDGE_OBSCURED: cloudfront-resolved target but alias not in distribution alt domains.
	if validation.DNSStatus == "resolved" && isCloudFrontSignal(node, validation) && !inAlternateDomains(node) {
		return models.ClaimEdgeObscured, models.SeverityMedium
	}

	// BROKEN: name does not resolve reliably.
	if validation.DNSStatus == "nxdomain" || validation.DNSStatus == "timeout" || validation.DNSStatus == "servfail" {
		return models.ClaimBroken, models.SeverityLow
	}

	return models.ClaimUnknown, models.SeverityInfo
}

func bucketNameFromTarget(node models.AssetNode) string {
	value, ok := node.Properties["value"].(string)
	if !ok {
		return ""
	}
	parts := strings.Split(value, ".")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func targetService(node models.AssetNode) string {
	if raw, ok := node.Properties["target_service"].(string); ok && raw != "" {
		return raw
	}
	value, _ := node.Properties["value"].(string)
	v := strings.ToLower(value)
	switch {
	case strings.Contains(v, ".s3-website-") || strings.Contains(v, ".s3-website."):
		return "s3_website"
	case strings.HasSuffix(v, ".cloudfront.net"):
		return "cloudfront"
	default:
		return ""
	}
}

func inAlternateDomains(node models.AssetNode) bool {
	v, ok := node.Properties["in_alternate_domains"].(bool)
	return ok && v
}

func isCloudFrontSignal(node models.AssetNode, validation validators.ValidationResult) bool {
	if strings.EqualFold(targetService(node), "cloudfront") {
		return true
	}
	if validation.CDNDetected && strings.EqualFold(validation.CDNVendor, "cloudfront") {
		return true
	}
	value, _ := node.Properties["value"].(string)
	return strings.HasSuffix(strings.ToLower(value), ".cloudfront.net")
}

func impactFor(c models.Claimability) string {
	switch c {
	case models.ClaimReclaimable:
		return "Potential subdomain takeover via reclaimable endpoint."
	case models.ClaimDangling:
		return "Dangling AWS endpoint may be misconfigured and exploitable."
	case models.ClaimEdgeObscured:
		return "Hostname is obscured behind edge and may not be actively bound."
	case models.ClaimBroken:
		return "Broken DNS target can cause service unavailability."
	default:
		return "Insufficient evidence for deterministic risk classification."
	}
}

func recommendationFor(c models.Claimability) string {
	switch c {
	case models.ClaimReclaimable:
		return "Remove DNS record immediately or recreate/secure claimed bucket in owned account."
	case models.ClaimDangling:
		return "Validate endpoint ownership and remove stale DNS or restore intended target."
	case models.ClaimEdgeObscured:
		return "Confirm CloudFront alternate domain bindings and origin ownership."
	case models.ClaimBroken:
		return "Delete or repair broken DNS record and verify target health."
	default:
		return "Collect additional validation evidence before remediation."
	}
}
