package scorers

import (
	"context"
	"errors"
	"testing"
	"time"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"

	"github.com/Zero0x00/cloudrift/internal/config"
	"github.com/Zero0x00/cloudrift/internal/models"
)

func TestScoreCostMultipliers(t *testing.T) {
	node := models.AssetNode{AssetType: models.AssetCloudFrontDist}
	f := &models.Finding{Claimability: models.ClaimDangling}
	d, r := ScoreCost(node, f)
	if d != 35.0 || r != 105.0 {
		t.Fatalf("unexpected costs direct=%v risk=%v", d, r)
	}
}

func TestScoreCost_ReclaimableMultiplier(t *testing.T) {
	node := models.AssetNode{AssetType: models.AssetDNSRecord}
	f := &models.Finding{Claimability: models.ClaimReclaimable}
	d, r := ScoreCost(node, f)
	if d != 0.50 || r != 2.50 {
		t.Fatalf("expected 0.50 and 2.50, got %v and %v", d, r)
	}
}

func TestScoreCost_BrokenMultiplierOne(t *testing.T) {
	node := models.AssetNode{AssetType: models.AssetS3Bucket}
	f := &models.Finding{Claimability: models.ClaimBroken}
	d, r := ScoreCost(node, f)
	if d != 0.23 || r != 0.23 {
		t.Fatalf("expected same direct and risk cost, got %v and %v", d, r)
	}
}

func TestEnrichCost_Disabled(t *testing.T) {
	restore := stubCELoaders(t)
	defer restore()

	findings := []models.Finding{
		{
			ID:                "f1",
			AccountID:         "111111111111",
			AffectedARN:       "arn:aws:s3:::demo-bucket",
			Claimability:      models.ClaimDangling,
			MonthlyDirectCost: 0.23,
			MonthlyRiskCost:   0.69,
		},
	}
	cfg := *config.Default()
	cfg.Cost.UseCUR = false

	out, err := EnrichCostFromCE(context.Background(), findings, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[0].MonthlyDirectCost != findings[0].MonthlyDirectCost || out[0].MonthlyRiskCost != findings[0].MonthlyRiskCost {
		t.Fatalf("expected unchanged findings when disabled")
	}
}

func TestEnrichCost_Success(t *testing.T) {
	restore := stubCELoaders(t)
	defer restore()

	findings := []models.Finding{
		{
			ID:           "f1",
			Module:       models.ModuleOrphanedEdge,
			AccountID:    "111111111111",
			AffectedARN:  "arn:aws:cloudfront::111111111111:distribution/E123",
			Claimability: models.ClaimDangling,
		},
	}
	cfg := *config.Default()
	cfg.Cost.UseCUR = true

	mock := &fakeCE{
		out: &costexplorer.GetCostAndUsageOutput{
			ResultsByTime: []cetypes.ResultByTime{
				{
					Groups: []cetypes.Group{
						{
							Keys: []string{"111111111111", "Amazon CloudFront"},
							Metrics: map[string]cetypes.MetricValue{
								"UnblendedCost": {Amount: awsv2.String("20.50"), Unit: awsv2.String("USD")},
							},
						},
					},
				},
			},
		},
	}
	newCostExplorerClient = func(cfg awsv2.Config) CostExplorerAPI { return mock }

	out, err := EnrichCostFromCE(context.Background(), findings, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[0].MonthlyDirectCost != 20.50 {
		t.Fatalf("expected direct cost 20.50, got %v", out[0].MonthlyDirectCost)
	}
	if out[0].MonthlyRiskCost != 61.5 {
		t.Fatalf("expected risk cost 61.5, got %v", out[0].MonthlyRiskCost)
	}
}

func TestEnrichCost_FailureFallback(t *testing.T) {
	restore := stubCELoaders(t)
	defer restore()

	findings := []models.Finding{
		{
			ID:                "f1",
			AccountID:         "111111111111",
			AffectedARN:       "arn:aws:s3:::demo-bucket",
			Claimability:      models.ClaimReclaimable,
			MonthlyDirectCost: 10,
			MonthlyRiskCost:   50,
		},
	}
	cfg := *config.Default()
	cfg.Cost.UseCUR = true

	newCostExplorerClient = func(cfg awsv2.Config) CostExplorerAPI {
		return &fakeCE{err: errors.New("ce unavailable")}
	}

	out, err := EnrichCostFromCE(context.Background(), findings, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[0].MonthlyDirectCost != 10 || out[0].MonthlyRiskCost != 50 {
		t.Fatalf("expected fallback to original values, got direct=%v risk=%v", out[0].MonthlyDirectCost, out[0].MonthlyRiskCost)
	}
}

func TestEnrichCost_PartialMatch(t *testing.T) {
	restore := stubCELoaders(t)
	defer restore()

	findings := []models.Finding{
		{
			ID:                "f1",
			Module:            models.ModuleOrphanedEdge,
			AccountID:         "111111111111",
			AffectedARN:       "arn:aws:s3:::demo-bucket",
			Claimability:      models.ClaimDangling,
			MonthlyDirectCost: 0.23,
			MonthlyRiskCost:   0.69,
		},
		{
			ID:                "f2",
			Module:            models.ModuleOrphanedEdge,
			AccountID:         "111111111111",
			AffectedARN:       "arn:aws:route53:::hostedzone/Z1",
			Claimability:      models.ClaimBroken,
			MonthlyDirectCost: 0.50,
			MonthlyRiskCost:   0.50,
		},
	}
	cfg := *config.Default()
	cfg.Cost.UseCUR = true
	newCostExplorerClient = func(cfg awsv2.Config) CostExplorerAPI {
		return &fakeCE{
			out: &costexplorer.GetCostAndUsageOutput{
				ResultsByTime: []cetypes.ResultByTime{
					{
						Groups: []cetypes.Group{
							{
								Keys: []string{"111111111111", "Amazon Simple Storage Service"},
								Metrics: map[string]cetypes.MetricValue{
									"UnblendedCost": {Amount: awsv2.String("3.00"), Unit: awsv2.String("USD")},
								},
							},
						},
					},
				},
			},
		}
	}

	out, err := EnrichCostFromCE(context.Background(), findings, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[0].MonthlyDirectCost != 3 || out[0].MonthlyRiskCost != 9 {
		t.Fatalf("expected updated S3 finding costs, got direct=%v risk=%v", out[0].MonthlyDirectCost, out[0].MonthlyRiskCost)
	}
	if out[1].MonthlyDirectCost != 0.50 || out[1].MonthlyRiskCost != 0.50 {
		t.Fatalf("expected unmatched service to remain unchanged")
	}
}

func TestEnrichCost_DistributesAcrossMatchingFindings(t *testing.T) {
	restore := stubCELoaders(t)
	defer restore()

	findings := []models.Finding{
		{
			ID:           "f1",
			Module:       models.ModuleOrphanedEdge,
			AccountID:    "111111111111",
			AffectedARN:  "arn:aws:s3:::bucket-a",
			Claimability: models.ClaimDangling,
		},
		{
			ID:           "f2",
			Module:       models.ModuleOrphanedEdge,
			AccountID:    "111111111111",
			AffectedARN:  "arn:aws:s3:::bucket-b",
			Claimability: models.ClaimReclaimable,
		},
	}
	cfg := *config.Default()
	cfg.Cost.UseCUR = true
	newCostExplorerClient = func(cfg awsv2.Config) CostExplorerAPI {
		return &fakeCE{
			out: &costexplorer.GetCostAndUsageOutput{
				ResultsByTime: []cetypes.ResultByTime{
					{
						Groups: []cetypes.Group{
							{
								Keys: []string{"111111111111", "Amazon Simple Storage Service"},
								Metrics: map[string]cetypes.MetricValue{
									"UnblendedCost": {Amount: awsv2.String("10.00"), Unit: awsv2.String("USD")},
								},
							},
						},
					},
				},
			},
		}
	}

	out, err := EnrichCostFromCE(context.Background(), findings, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[0].MonthlyDirectCost != 5 || out[1].MonthlyDirectCost != 5 {
		t.Fatalf("expected equal 5.00 direct split, got %v and %v", out[0].MonthlyDirectCost, out[1].MonthlyDirectCost)
	}
	if out[0].MonthlyRiskCost != 15 {
		t.Fatalf("expected dangling risk cost 15.00, got %v", out[0].MonthlyRiskCost)
	}
	if out[1].MonthlyRiskCost != 25 {
		t.Fatalf("expected reclaimable risk cost 25.00, got %v", out[1].MonthlyRiskCost)
	}
}

func TestEnrichCost_ExternalAccessTrustFindingNotOverridden(t *testing.T) {
	restore := stubCELoaders(t)
	defer restore()

	findings := []models.Finding{
		{
			ID:                "trust-1",
			Module:            models.ModuleExternalAccess,
			AccountID:         "111111111111",
			AffectedARN:       "arn:aws:iam::111111111111:role/TrustedRole",
			Claimability:      models.ClaimReclaimable,
			MonthlyDirectCost: 1.25,
			MonthlyRiskCost:   1.25,
		},
	}
	cfg := *config.Default()
	cfg.Cost.UseCUR = true
	newCostExplorerClient = func(cfg awsv2.Config) CostExplorerAPI {
		return &fakeCE{
			out: &costexplorer.GetCostAndUsageOutput{
				ResultsByTime: []cetypes.ResultByTime{
					{
						Groups: []cetypes.Group{
							{
								Keys: []string{"111111111111", "AWS Identity and Access Management"},
								Metrics: map[string]cetypes.MetricValue{
									"UnblendedCost": {Amount: awsv2.String("99.00"), Unit: awsv2.String("USD")},
								},
							},
						},
					},
				},
			},
		}
	}

	out, err := EnrichCostFromCE(context.Background(), findings, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[0].MonthlyDirectCost != 1.25 || out[0].MonthlyRiskCost != 1.25 {
		t.Fatalf("expected trust finding costs to remain unchanged, got direct=%v risk=%v", out[0].MonthlyDirectCost, out[0].MonthlyRiskCost)
	}
}

type fakeCE struct {
	out *costexplorer.GetCostAndUsageOutput
	err error
}

func (f *fakeCE) GetCostAndUsage(_ context.Context, _ *costexplorer.GetCostAndUsageInput, _ ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.out, nil
}

func stubCELoaders(t *testing.T) func() {
	t.Helper()
	origLoad := loadAWSCfgForCE
	origClient := newCostExplorerClient
	origNow := ceNowUTC
	loadAWSCfgForCE = func(ctx context.Context, cfg config.Config) (awsv2.Config, error) {
		return awsv2.Config{Region: "us-east-1"}, nil
	}
	newCostExplorerClient = func(cfg awsv2.Config) CostExplorerAPI {
		return &fakeCE{out: &costexplorer.GetCostAndUsageOutput{}}
	}
	ceNowUTC = func() time.Time {
		return time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC)
	}
	return func() {
		loadAWSCfgForCE = origLoad
		newCostExplorerClient = origClient
		ceNowUTC = origNow
	}
}
