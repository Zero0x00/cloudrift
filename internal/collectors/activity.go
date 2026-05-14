package collectors

import (
	"context"
	"sort"
	"sync"
	"time"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/Zero0x00/cloudrift/internal/config"
)

type RoleActivity struct {
	RoleARN       string     `json:"role_arn"`
	RoleName      string     `json:"role_name"`
	AccountID     string     `json:"account_id"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
	DaysSinceUsed int        `json:"days_since_used"`
}

type IAMActivityAPI interface {
	ListRoles(ctx context.Context, params *iam.ListRolesInput, optFns ...func(*iam.Options)) (*iam.ListRolesOutput, error)
	GetRole(ctx context.Context, params *iam.GetRoleInput, optFns ...func(*iam.Options)) (*iam.GetRoleOutput, error)
}

var newIAMActivityClient = func(cfg awsv2.Config) IAMActivityAPI {
	return iam.NewFromConfig(cfg)
}

var activityNowUTC = func() time.Time {
	return time.Now().UTC()
}

func CollectActivity(ctx context.Context, accounts []Account) ([]RoleActivity, error) {
	return CollectActivityWithConfig(ctx, config.Default(), accounts)
}

func CollectActivityWithConfig(ctx context.Context, cfg *config.Config, accounts []Account) ([]RoleActivity, error) {
	sem := make(chan struct{}, max(1, cfg.Scan.RoleAssumptionConcurrency))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	var all []RoleActivity
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

			local, err := collectActivityFromClient(ctx, account.ID, newIAMActivityClient(*account.Session))
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}

			mu.Lock()
			all = append(all, local...)
			mu.Unlock()
		}()
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].RoleARN < all[j].RoleARN
	})
	return all, nil
}

func collectActivityFromClient(ctx context.Context, accountID string, client IAMActivityAPI) ([]RoleActivity, error) {
	now := activityNowUTC().Truncate(24 * time.Hour)
	var out []RoleActivity
	var marker *string

	for {
		listOut, err := client.ListRoles(ctx, &iam.ListRolesInput{Marker: marker})
		if err != nil {
			return nil, err
		}

		for _, role := range listOut.Roles {
			roleName := awsv2.ToString(role.RoleName)
			getOut, err := client.GetRole(ctx, &iam.GetRoleInput{RoleName: awsv2.String(roleName)})
			if err != nil {
				return nil, err
			}

			roleARN := awsv2.ToString(role.Arn)
			if getOut.Role != nil && getOut.Role.Arn != nil {
				roleARN = awsv2.ToString(getOut.Role.Arn)
			}

			activity := RoleActivity{
				RoleARN:       roleARN,
				RoleName:      roleName,
				AccountID:     accountID,
				DaysSinceUsed: -1,
			}

			if getOut.Role != nil && getOut.Role.RoleLastUsed != nil && getOut.Role.RoleLastUsed.LastUsedDate != nil {
				lastUsed := getOut.Role.RoleLastUsed.LastUsedDate.UTC()
				lastUsedDay := lastUsed.Truncate(24 * time.Hour)
				days := int(now.Sub(lastUsedDay).Hours() / 24)
				if days < 0 {
					days = 0
				}
				activity.LastUsedAt = &lastUsed
				activity.DaysSinceUsed = days
			}

			out = append(out, activity)
		}

		if !listOut.IsTruncated || listOut.Marker == nil {
			break
		}
		marker = listOut.Marker
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].RoleARN < out[j].RoleARN
	})
	return out, nil
}

func IndexActivityByRoleARN(rows []RoleActivity) map[string]RoleActivity {
	index := make(map[string]RoleActivity, len(rows))
	for _, row := range rows {
		index[row.RoleARN] = row
	}
	return index
}

func setActivityNow(now time.Time) func() {
	prev := activityNowUTC
	activityNowUTC = func() time.Time { return now.UTC() }
	return func() {
		activityNowUTC = prev
	}
}

