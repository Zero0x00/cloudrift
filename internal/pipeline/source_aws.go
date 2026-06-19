package pipeline

import (
	"context"
	"fmt"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/organizations"

	cloudaws "github.com/Zero0x00/cloudrift/internal/aws"
	"github.com/Zero0x00/cloudrift/internal/collectors"
	"github.com/Zero0x00/cloudrift/internal/config"
	"github.com/Zero0x00/cloudrift/internal/models"
)

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

	orgClient := organizations.NewFromConfig(baseCfg)
	sm := cloudaws.NewSessionManagerFromConfig(baseCfg, cfg.AWS.OrgRoleName)

	accounts, err := collectors.CollectAccounts(ctx, cfg, orgClient, sm)
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

	storage, err := collectors.CollectStorageWithConfig(ctx, cfg, accounts)
	if err != nil {
		return Result{}, fmt.Errorf("collect storage: %w", err)
	}
	assets = append(assets, storage...)

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

	return Result{Accounts: accounts, Assets: assets, Rels: rels, Activity: activity}, nil
}
