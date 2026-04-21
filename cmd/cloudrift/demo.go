package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"cloudrift/internal/config"
	"cloudrift/internal/models"
	"cloudrift/internal/scans"
)

const demoDirTimestampFormat = "20060102T150405Z"

type demoArtifacts struct {
	metadata      models.ScanSnapshot
	findings      []models.Finding
	relationships []models.Relationship
	assetFiles    map[string][]models.AssetNode
}

func newDemoCommand(cfgPath *string) *cobra.Command {
	var outputDir string
	var neo4jEnabled bool
	var fixedTimestamp string
	var scanIDFlag string

	demoCmd := &cobra.Command{
		Use:   "demo",
		Short: "Generate demo scan datasets",
	}

	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a deterministic demo scan dataset",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return err
			}
			if strings.TrimSpace(outputDir) == "" {
				outputDir = cfg.Output.OutputDir
			}

			now := time.Now().UTC()
			if ts := strings.TrimSpace(fixedTimestamp); ts != "" {
				parsed, err := time.Parse(time.RFC3339, ts)
				if err != nil {
					return fmt.Errorf("parse --timestamp: %w", err)
				}
				now = parsed.UTC()
			}

			scanPath, err := generateDemoScan(outputDir, now, strings.TrimSpace(scanIDFlag))
			if err != nil {
				return err
			}

			if neo4jEnabled {
				if err := exportScanToNeo4j(context.Background(), cfg, scanPath, defaultNeo4jConnector{}); err != nil {
					return err
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "demo scan generated at %s\n", scanPath)
			return nil
		},
	}
	generateCmd.Flags().StringVar(&outputDir, "output-dir", "", "Output directory")
	generateCmd.Flags().BoolVar(&neo4jEnabled, "neo4j", false, "Write generated demo projection to Neo4j")
	generateCmd.Flags().StringVar(&fixedTimestamp, "timestamp", "", "Fixed RFC3339 timestamp (deterministic testing)")
	generateCmd.Flags().StringVar(&scanIDFlag, "scan-id", "", "Fixed scan directory name (e.g. demo). Default: demo-<UTC timestamp>. Must satisfy safe scan id rules.")
	_ = generateCmd.Flags().MarkHidden("timestamp")
	demoCmd.AddCommand(generateCmd)
	return demoCmd
}

func generateDemoScan(outputDir string, t time.Time, fixedScanID string) (string, error) {
	scanID := fixedScanID
	if scanID == "" {
		scanID = "demo-" + t.UTC().Format(demoDirTimestampFormat)
	}
	if !scans.IsSafeScanID(scanID) {
		return "", fmt.Errorf("invalid --scan-id %q: must be a non-empty safe scan id (letters, digits, ._- only)", scanID)
	}
	scanPath := filepath.Join(outputDir, scanID)
	if err := os.MkdirAll(filepath.Join(scanPath, "assets"), 0o755); err != nil {
		return "", fmt.Errorf("create demo assets directory: %w", err)
	}

	artifacts := buildDemoArtifacts(scanID, t.UTC())
	if err := writeJSONFile(filepath.Join(scanPath, "scan-metadata.json"), artifacts.metadata); err != nil {
		return "", err
	}
	if err := writeJSONFile(filepath.Join(scanPath, "findings.json"), artifacts.findings); err != nil {
		return "", err
	}
	if err := writeJSONFile(filepath.Join(scanPath, "relationships.json"), artifacts.relationships); err != nil {
		return "", err
	}

	assetNames := make([]string, 0, len(artifacts.assetFiles))
	for name := range artifacts.assetFiles {
		assetNames = append(assetNames, name)
	}
	sort.Strings(assetNames)
	for _, name := range assetNames {
		path := filepath.Join(scanPath, "assets", name)
		if err := writeJSONFile(path, artifacts.assetFiles[name]); err != nil {
			return "", err
		}
	}

	return scanPath, nil
}

func writeJSONFile(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filepath.Base(path), err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func buildDemoArtifacts(scanID string, ts time.Time) demoArtifacts {
	accounts := []string{"111111111111", "222222222222", "333333333333"}
	findings := findingsForScan(scanID)

	criticalCount := 0
	highCount := 0
	totalCost := 0.0
	for _, f := range findings {
		if f.Severity == models.SeverityCritical {
			criticalCount++
		}
		if f.Severity == models.SeverityHigh {
			highCount++
		}
		totalCost += f.MonthlyRiskCost
	}

	metadata := models.ScanSnapshot{
		ScanID:           scanID,
		Timestamp:        ts,
		AccountIDs:       accounts,
		ToolVersion:      version,
		FindingCount:     len(findings),
		CriticalCount:    criticalCount,
		HighCount:        highCount,
		TotalMonthlyCost: totalCost,
	}

	assets := demoAssets(scanID)
	relationships := demoRelationships(scanID)
	return demoArtifacts{
		metadata:      metadata,
		findings:      findings,
		relationships: relationships,
		assetFiles: map[string][]models.AssetNode{
			"edge.json":               assets.edge,
			"iam_and_external.json":   assets.iamAndExternal,
			"infrastructure_core.json": assets.infrastructureCore,
		},
	}
}

type demoAssetSets struct {
	edge               []models.AssetNode
	iamAndExternal     []models.AssetNode
	infrastructureCore []models.AssetNode
}

func demoAssets(scanID string) demoAssetSets {
	infra := []models.AssetNode{
		{ARN: "arn:aws:cloudfront::111111111111:distribution/E123EXAMPLE", AssetType: models.AssetCloudFrontDist, Name: "legacy-api-dist", AccountID: "111111111111", Region: "us-east-1", ScanID: scanID},
		{ARN: "arn:aws:acm:us-east-1:111111111111:certificate/cert-123", AssetType: models.AssetACMCert, Name: "legacy-api-cert", AccountID: "111111111111", Region: "us-east-1", ScanID: scanID},
		{ARN: "arn:aws:s3:::old-campaign-assets", AssetType: models.AssetS3Bucket, Name: "old-campaign-assets", AccountID: "111111111111", Region: "us-east-1", ScanID: scanID},
		{ARN: "arn:aws:s3:::telemetry-archive", AssetType: models.AssetS3Bucket, Name: "telemetry-archive", AccountID: "222222222222", Region: "us-west-2", ScanID: scanID},
	}

	edge := []models.AssetNode{
		{ARN: "arn:aws:route53:::hostedzone/Z1D633PJN8HTWQ/static.example.com", AssetType: models.AssetDNSRecord, Name: "static.example.com", AccountID: "111111111111", Region: "global", ScanID: scanID},
		{ARN: "arn:aws:route53:::hostedzone/Z2EXAMPLE/old-marketing.example.com", AssetType: models.AssetDNSRecord, Name: "old-marketing.example.com", AccountID: "111111111111", Region: "global", ScanID: scanID},
		{ARN: "arn:aws:route53:::hostedzone/Z3EXAMPLE/api.legacy.corp", AssetType: models.AssetDNSRecord, Name: "api.legacy.corp", AccountID: "111111111111", Region: "global", ScanID: scanID},
		{ARN: "arn:aws:route53:::hostedzone/Z4EXAMPLE/cdn.partner.io", AssetType: models.AssetDNSRecord, Name: "cdn.partner.io", AccountID: "222222222222", Region: "global", ScanID: scanID},
	}

	iamRoles := []models.AssetNode{
		demoRole(scanID, "111111111111", "VendorAccessRole"),
		demoRole(scanID, "222222222222", "BreakGlassRole"),
		demoRole(scanID, "111111111111", "IntegrationRole"),
		demoRole(scanID, "333333333333", "ReadOnlyAudit"),
		demoRole(scanID, "111111111111", "OidcDevRole"),
		demoRole(scanID, "333333333333", "SamlBillingRole"),
		demoRole(scanID, "222222222222", "OpsSupportRole"),
	}
	externalPrincipals := []models.AssetNode{
		demoExternalPrincipal(scanID, "111111111111", "aws_account", "arn:aws:iam::999999999999:root"),
		demoExternalPrincipal(scanID, "222222222222", "aws_account", "arn:aws:iam::888888888888:root"),
		demoExternalPrincipal(scanID, "111111111111", "aws_account", "arn:aws:iam::222222222222:root"),
		demoExternalPrincipal(scanID, "333333333333", "aws_account", "arn:aws:iam::444444444444:root"),
		demoExternalPrincipal(scanID, "111111111111", "oidc", "https://dev.okta.com/oauth2/default"),
		demoExternalPrincipal(scanID, "333333333333", "saml", "arn:aws:iam::000000000000:saml-provider/CorpSAML"),
		demoExternalPrincipal(scanID, "222222222222", "unknown", "arn:aws:iam::555555555555:root"),
	}

	iamAndExternal := append(iamRoles, externalPrincipals...)
	return demoAssetSets{
		edge:               edge,
		iamAndExternal:     iamAndExternal,
		infrastructureCore: infra,
	}
}

func demoRole(scanID, accountID, roleName string) models.AssetNode {
	return models.AssetNode{
		ARN:       fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, roleName),
		AssetType: models.AssetIAMRole,
		Name:      roleName,
		AccountID: accountID,
		Region:    "global",
		ScanID:    scanID,
		Properties: map[string]any{
			"path": "/",
		},
	}
}

func demoExternalPrincipal(scanID, accountID, principalType, value string) models.AssetNode {
	return models.AssetNode{
		ARN:       externalPrincipalARNForDemo(principalType, value),
		AssetType: models.AssetExternalPrincipal,
		Name:      value,
		AccountID: accountID,
		Region:    "global",
		ScanID:    scanID,
		Properties: map[string]any{
			"principal_type":  principalType,
			"principal_value": value,
		},
	}
}

func demoRelationships(scanID string) []models.Relationship {
	rels := []models.Relationship{
		{SourceARN: "arn:aws:route53:::hostedzone/Z1D633PJN8HTWQ/static.example.com", TargetARN: "arn:aws:s3:::old-campaign-assets", RelType: models.RelPointsTo, ScanID: scanID},
		{SourceARN: "arn:aws:route53:::hostedzone/Z3EXAMPLE/api.legacy.corp", TargetARN: "arn:aws:cloudfront::111111111111:distribution/E123EXAMPLE", RelType: models.RelPointsTo, ScanID: scanID},
		{SourceARN: "arn:aws:route53:::hostedzone/Z4EXAMPLE/cdn.partner.io", TargetARN: "arn:aws:cloudfront::111111111111:distribution/E123EXAMPLE", RelType: models.RelPointsTo, ScanID: scanID},
		{SourceARN: "arn:aws:cloudfront::111111111111:distribution/E123EXAMPLE", TargetARN: "arn:aws:acm:us-east-1:111111111111:certificate/cert-123", RelType: models.RelUsesCert, ScanID: scanID},
		{SourceARN: "arn:aws:cloudfront::111111111111:distribution/E123EXAMPLE", TargetARN: "arn:aws:s3:::old-campaign-assets", RelType: models.RelFronts, ScanID: scanID},
	}

	trustPairs := [][2]string{
		{"arn:aws:iam::111111111111:role/VendorAccessRole", externalPrincipalARNForDemo("aws_account", "arn:aws:iam::999999999999:root")},
		{"arn:aws:iam::222222222222:role/BreakGlassRole", externalPrincipalARNForDemo("aws_account", "arn:aws:iam::888888888888:root")},
		{"arn:aws:iam::111111111111:role/IntegrationRole", externalPrincipalARNForDemo("aws_account", "arn:aws:iam::222222222222:root")},
		{"arn:aws:iam::333333333333:role/ReadOnlyAudit", externalPrincipalARNForDemo("aws_account", "arn:aws:iam::444444444444:root")},
		{"arn:aws:iam::111111111111:role/OidcDevRole", externalPrincipalARNForDemo("oidc", "https://dev.okta.com/oauth2/default")},
		{"arn:aws:iam::333333333333:role/SamlBillingRole", externalPrincipalARNForDemo("saml", "arn:aws:iam::000000000000:saml-provider/CorpSAML")},
		{"arn:aws:iam::222222222222:role/OpsSupportRole", externalPrincipalARNForDemo("unknown", "arn:aws:iam::555555555555:root")},
	}
	for _, p := range trustPairs {
		rels = append(rels, models.Relationship{
			SourceARN: p[0],
			TargetARN: p[1],
			RelType:   models.RelTrusts,
			ScanID:    scanID,
		})
	}
	return rels
}

func externalPrincipalARNForDemo(principalType, value string) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(value))
	return "arn:cloudrift:external-principal:::" + principalType + "/" + encoded
}

func demoFindings(scanID string) []models.Finding {
	return []models.Finding{
		{
			ID:                "demo-orphan-reclaimable",
			Title:             "static.example.com -> reclaimable",
			Severity:          models.SeverityCritical,
			Module:            models.ModuleOrphanedEdge,
			Claimability:      models.ClaimReclaimable,
			AffectedARN:       "arn:aws:route53:::hostedzone/Z1D633PJN8HTWQ/static.example.com",
			AccountID:         "111111111111",
			AccountName:       "Workload-Prod",
			OUPath:            "Root/Workloads/Prod",
			Team:              "Platform",
			Hostname:          "static.example.com",
			MonthlyDirectCost: 0.5,
			MonthlyRiskCost:   82.0,
			Impact:            "DNS points at deleted S3 website; bucket not present in scanned accounts.",
			Recommendation:    "Remove hosted zone record or reclaim hostname after validation.",
			RemediationCmd:    "aws route53 change-resource-record-sets --hosted-zone-id Z1D633PJN8HTWQ --change-batch file://batch.json",
			Evidence: map[string]any{
				"dns_status":   "resolved",
				"http_status":  404,
				"fingerprint":  "s3_bucket_deleted",
				"bucket_name":  "old-campaign-assets",
				"expected_edge": "s3_static_site",
			},
			ScanID: scanID,
		},
		{
			ID:                "demo-orphan-dangling",
			Title:             "api.legacy.corp -> dangling",
			Severity:          models.SeverityHigh,
			Module:            models.ModuleOrphanedEdge,
			Claimability:      models.ClaimDangling,
			AffectedARN:       "arn:aws:route53:::hostedzone/Z3EXAMPLE/api.legacy.corp",
			AccountID:         "111111111111",
			AccountName:       "Workload-Prod",
			OUPath:            "Root/Workloads/Prod",
			Team:              "Platform",
			Hostname:          "api.legacy.corp",
			MonthlyDirectCost: 35,
			MonthlyRiskCost:   140,
			Impact:            "Alias resolves to CloudFront; origin errors suggest misconfiguration.",
			Recommendation:    "Validate origin and remove if unused.",
			Evidence: map[string]any{
				"dns_status":  "resolved",
				"http_status": 502,
				"fingerprint": "origin_error",
			},
			ScanID: scanID,
		},
		{
			ID:                "demo-orphan-broken",
			Title:             "old-marketing.example.com -> broken",
			Severity:          models.SeverityLow,
			Module:            models.ModuleOrphanedEdge,
			Claimability:      models.ClaimBroken,
			AffectedARN:       "arn:aws:route53:::hostedzone/Z2EXAMPLE/old-marketing.example.com",
			AccountID:         "111111111111",
			AccountName:       "Workload-Prod",
			OUPath:            "Root/Workloads/Prod",
			Team:              "Marketing",
			Hostname:          "old-marketing.example.com",
			MonthlyDirectCost: 0.4,
			MonthlyRiskCost:   0.4,
			Impact:            "NXDOMAIN indicates stale record inventory.",
			Recommendation:    "Delete stale DNS entry after owner confirmation.",
			Evidence: map[string]any{
				"dns_status": "nxdomain",
			},
			ScanID: scanID,
		},
		externalAccessFinding(scanID, "demo-trust-1", "VendorAccessRole", "111111111111", "Security", models.SeverityHigh, "unknown_vendor", 400, "arn:aws:iam::999999999999:root", "aws_account", "999999999999", permissionVisibility(models.PermissionTierScoped, false, true, false, false, false), "external account not found in approved list"),
		externalAccessFinding(scanID, "demo-trust-2", "BreakGlassRole", "222222222222", "Security", models.SeverityCritical, "ghost_admin_access", 5, "arn:aws:iam::888888888888:root", "aws_account", "888888888888", permissionVisibility(models.PermissionTierAdmin, true, true, true, true, true), "is_admin=true with external trust"),
		externalAccessFinding(scanID, "demo-trust-3", "IntegrationRole", "111111111111", "Integrations", models.SeverityMedium, "aging", 120, "arn:aws:iam::222222222222:root", "aws_account", "222222222222", permissionVisibility(models.PermissionTierPrivileged, true, true, false, false, true), "days_since_used between stale and ghost thresholds"),
		externalAccessFinding(scanID, "demo-trust-4", "ReadOnlyAudit", "333333333333", "Security", models.SeverityLow, "active", 12, "arn:aws:iam::444444444444:root", "aws_account", "444444444444", permissionVisibility(models.PermissionTierLimited, false, false, false, false, false), "days_since_used below stale threshold"),
		externalAccessFinding(scanID, "demo-trust-5", "OidcDevRole", "111111111111", "Platform", models.SeverityHigh, "stale_review_now", -1, "https://dev.okta.com/oauth2/default", "oidc", "", permissionVisibility(models.PermissionTierUnknown, false, false, false, false, false), "never used or days_since_used > ghost threshold"),
		externalAccessFinding(scanID, "demo-trust-6", "SamlBillingRole", "333333333333", "Finance-Platform", models.SeverityMedium, "aging", 95, "arn:aws:iam::000000000000:saml-provider/CorpSAML", "saml", "", permissionVisibility(models.PermissionTierScoped, false, false, true, false, false), "days_since_used between stale and ghost thresholds"),
		externalAccessFinding(scanID, "demo-trust-7", "OpsSupportRole", "222222222222", "Operations", models.SeverityHigh, "stale_review_now", 500, "arn:aws:iam::555555555555:root", "unknown", "", permissionVisibility(models.PermissionTierUnknown, false, false, false, false, false), "external principal type not classified"),
	}
}

func permissionVisibility(tier models.PermissionTier, adminLike, canAssume, iamWrite, s3Write, cloudfront bool) map[string]any {
	confidence := models.PermissionConfidenceMedium
	if tier == models.PermissionTierAdmin {
		confidence = models.PermissionConfidenceHigh
	}
	if tier == models.PermissionTierUnknown {
		confidence = models.PermissionConfidenceLow
	}
	return map[string]any{
		"classification": tier,
		"capabilities": map[string]any{
			"admin_like":         adminLike,
			"can_assume_role":    canAssume,
			"iam_write_access":   iamWrite,
			"s3_write_access":    s3Write,
			"cloudfront_control": cloudfront,
		},
		"confidence": confidence,
	}
}

func externalAccessFinding(
	scanID, idSuffix, roleName, accountID, team string,
	severity models.Severity,
	verdict string,
	daysSinceUsed int,
	externalPrincipal, principalType, externalAccountID string,
	permissionVisibility map[string]any,
	reason string,
) models.Finding {
	return models.Finding{
		ID:                idSuffix,
		Title:             fmt.Sprintf("External trust on %s -> %s", roleName, verdict),
		Severity:          severity,
		Module:            models.ModuleExternalAccess,
		Claimability:      models.ClaimUnknown,
		AffectedARN:       fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, roleName),
		AccountID:         accountID,
		AccountName:       map[string]string{"111111111111": "Workload-Prod", "222222222222": "Shared-Services", "333333333333": "Sandbox"}[accountID],
		OUPath:            map[string]string{"111111111111": "Root/Workloads/Prod", "222222222222": "Root/Shared", "333333333333": "Root/Dev"}[accountID],
		Team:              team,
		MonthlyDirectCost: 0,
		MonthlyRiskCost:   map[models.Severity]float64{models.SeverityCritical: 300, models.SeverityHigh: 120, models.SeverityMedium: 45, models.SeverityLow: 12}[severity],
		Impact:            "Role trusts an external principal and requires access recertification.",
		Recommendation:    "Review trust boundary, validate owner approval, and scope permissions to least privilege.",
		Evidence: map[string]any{
			"role_arn":              fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, roleName),
			"external_principal":    externalPrincipal,
			"principal_type":        principalType,
			"external_account_id":   externalAccountID,
			"days_since_used":       daysSinceUsed,
			"verdict":               verdict,
			"reason":                reason,
			"activity_source":       "iam:getrole:role_last_used",
			"activity_status":       activityStatusForDays(daysSinceUsed),
			"permission_visibility": permissionVisibility,
			"admin_eval_state":      adminEvalState(permissionVisibility),
			"is_admin":              boolFromNested(permissionVisibility, "capabilities", "admin_like"),
			"unknown_vendor":        verdict == "unknown_vendor",
		},
		ScanID: scanID,
	}
}

func activityStatusForDays(days int) string {
	if days < 0 {
		return "iam_never_used"
	}
	return "observed"
}

func adminEvalState(visibility map[string]any) string {
	if boolFromNested(visibility, "capabilities", "admin_like") {
		return "true"
	}
	if visibility["classification"] == models.PermissionTierUnknown {
		return "unknown"
	}
	return "false"
}

func boolFromNested(root map[string]any, mapKey, boolKey string) bool {
	raw, ok := root[mapKey]
	if !ok {
		return false
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return false
	}
	v, _ := m[boolKey].(bool)
	return v
}
