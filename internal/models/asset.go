package models

type AssetType string

const (
	AssetDNSRecord         AssetType = "dns_record"
	AssetS3Bucket          AssetType = "s3_bucket"
	AssetCloudFrontDist    AssetType = "cloudfront_dist"
	AssetAPIGatewayDomain  AssetType = "apigateway_domain"
	AssetACMCert           AssetType = "acm_cert"
	AssetIAMRole           AssetType = "iam_role"
	AssetExternalPrincipal AssetType = "external_principal"
)

type AssetNode struct {
	ARN        string         `json:"arn"`
	AssetType  AssetType      `json:"asset_type"`
	Name       string         `json:"name"`
	AccountID  string         `json:"account_id"`
	Region     string         `json:"region"`
	Properties map[string]any `json:"properties"`
	ScanID     string         `json:"scan_id"`
}
