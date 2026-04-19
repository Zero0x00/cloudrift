package collectors

import (
	"context"
	"fmt"
	"strings"
	"sync"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"cloudrift/internal/config"
	"cloudrift/internal/models"
)

func CollectStorage(ctx context.Context, accounts []Account) ([]models.AssetNode, error) {
	return CollectStorageWithConfig(ctx, config.Default(), accounts)
}

func CollectStorageWithConfig(ctx context.Context, cfg *config.Config, accounts []Account) ([]models.AssetNode, error) {
	var out []models.AssetNode
	sem := make(chan struct{}, max(1, cfg.Scan.RoleAssumptionConcurrency))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	for _, account := range accounts {
		account := account
		if account.Session == nil {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			c := s3.NewFromConfig(*account.Session)
			list, err := c.ListBuckets(ctx, &s3.ListBucketsInput{})
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			var local []models.AssetNode
			for _, b := range list.Buckets {
				name := awsv2.ToString(b.Name)
				regionOut, err := c.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: b.Name})
				if err != nil {
					continue
				}
				region := normalizeBucketRegion(regionOut.LocationConstraint)
				websiteEnabled := false
				websiteEndpoint := ""
				if _, err = c.GetBucketWebsite(ctx, &s3.GetBucketWebsiteInput{Bucket: b.Name}); err == nil {
					websiteEnabled = true
					websiteEndpoint = websiteEndpointFor(name, region)
				}
				local = append(local, models.AssetNode{
					ARN:       fmt.Sprintf("arn:aws:s3:::%s", name),
					AssetType: models.AssetS3Bucket,
					Name:      name,
					AccountID: account.ID,
					Region:    region,
					Properties: map[string]any{
						"website_enabled":  websiteEnabled,
						"website_endpoint": websiteEndpoint,
						"bucket_region":    region,
					},
				})
			}
			mu.Lock()
			out = append(out, local...)
			mu.Unlock()
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

func websiteEndpointFor(bucket, region string) string {
	// Phase 1 supports both legacy and modern S3 website endpoint patterns.
	if strings.HasPrefix(region, "cn-") {
		return fmt.Sprintf("%s.s3-website.%s.amazonaws.com.cn", bucket, region)
	}
	return fmt.Sprintf("%s.s3-website-%s.amazonaws.com", bucket, region)
}

func normalizeBucketRegion(region s3types.BucketLocationConstraint) string {
	switch region {
	case "":
		return "us-east-1"
	case s3types.BucketLocationConstraintEu:
		return "eu-west-1"
	default:
		return string(region)
	}
}
