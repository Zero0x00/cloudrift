package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	cloudaws "github.com/Zero0x00/cloudrift/internal/aws"
	"github.com/Zero0x00/cloudrift/internal/collectors"
	"github.com/Zero0x00/cloudrift/internal/config"
	"github.com/Zero0x00/cloudrift/internal/models"
)

// currentAccountOnly builds a single-account scan target from the base credentials, used
// in no-assume mode (empty org_role_name). It resolves the caller's account via STS and
// scans it directly, with no role assumption and no Organizations API call.
func currentAccountOnly(ctx context.Context, baseCfg awsv2.Config) ([]collectors.Account, collectors.Coverage, error) {
	ident, err := sts.NewFromConfig(baseCfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, collectors.Coverage{}, fmt.Errorf("get caller identity: %w", err)
	}
	acctID := awsv2.ToString(ident.Account)
	slog.Info("no-assume mode: scanning caller's account only with base credentials", "account", acctID)
	sess := baseCfg
	accounts := []collectors.Account{{ID: acctID, Session: &sess}}
	cov := collectors.Coverage{Discovered: 1, Scanned: []string{acctID}}
	return accounts, cov, nil
}

// awsSource is the production Source: it bootstraps AWS clients and runs every collector.
type awsSource struct{}

// NewAWSSource returns the production collection source.
func NewAWSSource() Source { return awsSource{} }

func (awsSource) Collect(ctx context.Context, cfg *config.Config) (Result, error) {
	// Credential selection: explicit management_profile wins; otherwise fall back to the
	// AWS default chain (env/AWS_PROFILE/SSO/instance role).
	var loadOpts []func(*awsconfig.LoadOptions) error
	if p := strings.TrimSpace(cfg.AWS.ManagementProfile); p != "" {
		loadOpts = append(loadOpts, awsconfig.WithSharedConfigProfile(p))
	}
	baseCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return Result{}, fmt.Errorf("load aws config: %w", err)
	}

	var accounts []collectors.Account
	var coverage collectors.Coverage
	if strings.TrimSpace(cfg.AWS.OrgRoleName) == "" {
		// No-assume mode: scan only the caller's own account using the base credentials.
		// No cross-account role assumption, and no Organizations:ListAccounts call (so it
		// works for identities that only have read access to a single account).
		accounts, coverage, err = currentAccountOnly(ctx, baseCfg)
	} else {
		orgClient := organizations.NewFromConfig(baseCfg)
		sm := cloudaws.NewSessionManagerFromConfig(baseCfg, cfg.AWS.OrgRoleName)
		accounts, coverage, err = collectors.CollectAccounts(ctx, cfg, orgClient, sm)
	}
	if err != nil {
		return Result{}, fmt.Errorf("collect accounts: %w", err)
	}

	var assets []models.AssetNode
	var rels []models.Relationship

	dns, err := collectors.CollectDNSWithConfig(ctx, cfg, accounts)
	if err != nil {
		return Result{}, fmt.Errorf("collect dns: %w", err)
	}
	assets = append(assets, dns...)

	storage, storageFailed, err := collectors.CollectStorageWithConfig(ctx, cfg, accounts)
	if err != nil {
		return Result{}, fmt.Errorf("collect storage: %w", err)
	}
	assets = append(assets, storage...)
	// Bucket-enumeration gaps directly weaken the reclaimable verdict, so fold them into
	// coverage (Complete() becomes false, triggering the pipeline's reclaimable guard).
	if len(storageFailed) > 0 {
		if coverage.Failed == nil {
			coverage.Failed = map[string]string{}
		}
		for _, id := range storageFailed {
			coverage.Failed[id] = "s3 bucket enumeration failed"
		}
	}

	// Resource-based cross-account exposure (S3 bucket policies), reusing the buckets we
	// just listed so there is no second ListBuckets pass.
	bpPrincipals, bpRels, bpFailed, err := collectors.CollectBucketPoliciesWithConfig(ctx, cfg, accounts, storage)
	if err != nil {
		return Result{}, fmt.Errorf("collect bucket policies: %w", err)
	}
	assets = append(assets, bpPrincipals...)
	rels = append(rels, bpRels...)
	if len(bpFailed) > 0 {
		if coverage.Failed == nil {
			coverage.Failed = map[string]string{}
		}
		for _, id := range bpFailed {
			coverage.Failed[id] = "s3 bucket policy read denied"
		}
	}

	edgeAssets, edgeRels, err := collectors.CollectEdgeWithConfig(ctx, cfg, accounts)
	if err != nil {
		return Result{}, fmt.Errorf("collect edge: %w", err)
	}
	assets = append(assets, edgeAssets...)
	rels = append(rels, edgeRels...)

	trustAssets, trustRels, err := collectors.CollectTrustWithConfig(ctx, cfg, accounts)
	if err != nil {
		return Result{}, fmt.Errorf("collect trust: %w", err)
	}
	assets = append(assets, trustAssets...)
	rels = append(rels, trustRels...)

	activity, err := collectors.CollectActivityWithConfig(ctx, cfg, accounts)
	if err != nil {
		return Result{}, fmt.Errorf("collect activity: %w", err)
	}

	return Result{Accounts: accounts, Assets: assets, Rels: rels, Activity: activity, Coverage: coverage}, nil
}
