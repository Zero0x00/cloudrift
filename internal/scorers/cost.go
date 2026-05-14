package scorers

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"

	"github.com/Zero0x00/cloudrift/internal/config"
	"github.com/Zero0x00/cloudrift/internal/models"
)

type CostExplorerAPI interface {
	GetCostAndUsage(ctx context.Context, params *costexplorer.GetCostAndUsageInput, optFns ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error)
}

var newCostExplorerClient = func(cfg awsv2.Config) CostExplorerAPI {
	return costexplorer.NewFromConfig(cfg)
}

var loadAWSCfgForCE = func(ctx context.Context, cfg config.Config) (awsv2.Config, error) {
	var opts []func(*awscfg.LoadOptions) error
	if cfg.AWS.ManagementProfile != "" {
		opts = append(opts, awscfg.WithSharedConfigProfile(cfg.AWS.ManagementProfile))
	}
	return awscfg.LoadDefaultConfig(ctx, opts...)
}

var ceNowUTC = func() time.Time {
	return time.Now().UTC()
}

func ScoreCost(node models.AssetNode, finding *models.Finding) (directCost, riskCost float64) {
	// Phase 1 baseline estimates; refined with CUR in later phases.
	switch node.AssetType {
	case models.AssetDNSRecord:
		directCost = 0.50
	case models.AssetS3Bucket:
		directCost = 0.23
	case models.AssetCloudFrontDist:
		directCost = 35.0
	case models.AssetACMCert:
		directCost = 7.0
	default:
		directCost = 0
	}
	multiplier := 1.0
	switch finding.Claimability {
	case models.ClaimReclaimable:
		multiplier = 5.0
	case models.ClaimDangling:
		multiplier = 3.0
	case models.ClaimBroken, models.ClaimUnknown, models.ClaimEdgeObscured:
		multiplier = 1.0
	}
	riskCost = directCost * multiplier
	return directCost, riskCost
}

func EnrichCostFromCE(ctx context.Context, findings []models.Finding, cfg config.Config) ([]models.Finding, error) {
	if !cfg.Cost.UseCUR {
		return findings, nil
	}
	if len(findings) == 0 {
		return findings, nil
	}

	awsCfg, err := loadAWSCfgForCE(ctx, cfg)
	if err != nil {
		warnCE("failed to load AWS config for cost enrichment", err)
		return findings, nil
	}
	client := newCostExplorerClient(awsCfg)

	end := ceNowUTC().Format("2006-01-02")
	start := ceNowUTC().AddDate(0, 0, -30).Format("2006-01-02")
	out, err := client.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{
		TimePeriod: &cetypes.DateInterval{
			Start: &start,
			End:   &end,
		},
		Granularity: cetypes.GranularityMonthly,
		Metrics:     []string{"UnblendedCost"},
		GroupBy: []cetypes.GroupDefinition{
			{Type: cetypes.GroupDefinitionTypeDimension, Key: awsv2.String("LINKED_ACCOUNT")},
			{Type: cetypes.GroupDefinitionTypeDimension, Key: awsv2.String("SERVICE")},
		},
	})
	if err != nil {
		warnCE("Cost Explorer request failed; keeping static pricing", err)
		return findings, nil
	}

	costMap := buildCECostMap(out)
	if len(costMap) == 0 {
		return findings, nil
	}

	enriched := make([]models.Finding, len(findings))
	copy(enriched, findings)
	indicesByKey := make(map[string][]int)
	for i := range enriched {
		service := serviceForFinding(enriched[i])
		if service == "" {
			continue
		}
		key := ceKey(enriched[i].AccountID, service)
		if _, ok := costMap[key]; !ok {
			continue
		}
		indicesByKey[key] = append(indicesByKey[key], i)
	}

	// CE is grouped at account+service level. We distribute that monthly amount across
	// matching findings to avoid inflated additive totals when multiple findings map
	// to the same CE group.
	for key, idxs := range indicesByKey {
		if len(idxs) == 0 {
			continue
		}
		monthlyCost := costMap[key]
		share := monthlyCost / float64(len(idxs))
		for _, idx := range idxs {
			enriched[idx].MonthlyDirectCost = share
			enriched[idx].MonthlyRiskCost = share * multiplierForFinding(enriched[idx].Module, enriched[idx].Claimability)
		}
	}
	return enriched, nil
}

func buildCECostMap(out *costexplorer.GetCostAndUsageOutput) map[string]float64 {
	result := map[string]float64{}
	if out == nil {
		return result
	}
	for _, period := range out.ResultsByTime {
		for _, group := range period.Groups {
			if len(group.Keys) < 2 {
				continue
			}
			accountID := strings.TrimSpace(group.Keys[0])
			service := strings.TrimSpace(group.Keys[1])
			if accountID == "" || service == "" {
				continue
			}
			metric, ok := group.Metrics["UnblendedCost"]
			if !ok || metric.Amount == nil {
				continue
			}
			amount, err := parseAmount(*metric.Amount)
			if err != nil {
				continue
			}
			result[ceKey(accountID, service)] += amount
		}
	}
	return result
}

func parseAmount(raw string) (float64, error) {
	var value float64
	_, err := fmt.Sscanf(strings.TrimSpace(raw), "%f", &value)
	return value, err
}

func ceKey(accountID, service string) string {
	return accountID + "|" + service
}

func serviceForFinding(f models.Finding) string {
	// Restrict CE-derived service mapping to orphaned-edge findings.
	// External-access/trust findings should not inherit inferred service costs by default.
	if f.Module == models.ModuleExternalAccess {
		return ""
	}
	arn := strings.ToLower(f.AffectedARN)
	switch {
	case strings.Contains(arn, ":s3:::"):
		return "Amazon Simple Storage Service"
	case strings.Contains(arn, ":cloudfront:"):
		return "Amazon CloudFront"
	case strings.Contains(arn, ":route53:::"):
		return "Amazon Route 53"
	case strings.Contains(arn, ":apigateway:"):
		return "Amazon API Gateway"
	case strings.Contains(arn, ":acm:"):
		return "AWS Certificate Manager"
	case strings.Contains(arn, ":iam:"):
		return "AWS Identity and Access Management"
	default:
		return ""
	}
}

func multiplierForClaimability(c models.Claimability) float64 {
	return multiplierForFinding(models.ModuleOrphanedEdge, c)
}

func multiplierForFinding(module models.Module, c models.Claimability) float64 {
	if module != models.ModuleOrphanedEdge {
		return 1.0
	}
	switch c {
	case models.ClaimReclaimable:
		return 5.0
	case models.ClaimDangling:
		return 3.0
	default:
		return 1.0
	}
}

func warnCE(message string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: %s: %v\n", message, err)
	}
}
