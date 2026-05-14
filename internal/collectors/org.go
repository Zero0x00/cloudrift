package collectors

import (
	"context"
	"fmt"
	"sync"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/organizations/types"

	"github.com/Zero0x00/cloudrift/internal/aws"
	"github.com/Zero0x00/cloudrift/internal/config"
)

type Account struct {
	ID      string
	Name    string
	OUPath  string
	Team    string
	Owner   string
	Contact string
	Session *awsv2.Config
}

type OrganizationsAPI interface {
	ListAccounts(ctx context.Context, params *organizations.ListAccountsInput, optFns ...func(*organizations.Options)) (*organizations.ListAccountsOutput, error)
	ListTagsForResource(ctx context.Context, params *organizations.ListTagsForResourceInput, optFns ...func(*organizations.Options)) (*organizations.ListTagsForResourceOutput, error)
	ListParents(ctx context.Context, params *organizations.ListParentsInput, optFns ...func(*organizations.Options)) (*organizations.ListParentsOutput, error)
	DescribeOrganizationalUnit(ctx context.Context, params *organizations.DescribeOrganizationalUnitInput, optFns ...func(*organizations.Options)) (*organizations.DescribeOrganizationalUnitOutput, error)
}

func CollectAccounts(ctx context.Context, cfg *config.Config, orgAPI OrganizationsAPI, sm *aws.SessionManager) ([]Account, error) {
	var accounts []types.Account
	p := organizations.NewListAccountsPaginator(orgAPI, &organizations.ListAccountsInput{})
	for p.HasMorePages() {
		out, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, out.Accounts...)
	}

	out := make([]Account, len(accounts))
	sem := make(chan struct{}, max(1, cfg.Scan.RoleAssumptionConcurrency))
	var wg sync.WaitGroup
	var firstErr error
	var mu sync.Mutex
	for i, a := range accounts {
		i, a := i, a
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			session, err := sm.AssumeAccount(ctx, awsv2.ToString(a.Id))
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}

			out[i] = Account{
				ID:      awsv2.ToString(a.Id),
				Name:    awsv2.ToString(a.Name),
				OUPath:  buildOUPath(ctx, orgAPI, awsv2.ToString(a.Id)),
				Team:    tagValue(ctx, orgAPI, awsv2.ToString(a.Id), "Team"),
				Owner:   tagValue(ctx, orgAPI, awsv2.ToString(a.Id), "Owner"),
				Contact: tagValue(ctx, orgAPI, awsv2.ToString(a.Id), "Contact"),
				Session: &session,
			}
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

func tagValue(ctx context.Context, orgAPI OrganizationsAPI, accountID, key string) string {
	out, err := orgAPI.ListTagsForResource(ctx, &organizations.ListTagsForResourceInput{ResourceId: &accountID})
	if err != nil {
		return ""
	}
	for _, t := range out.Tags {
		if awsv2.ToString(t.Key) == key {
			return awsv2.ToString(t.Value)
		}
	}
	return ""
}

func buildOUPath(ctx context.Context, orgAPI OrganizationsAPI, accountID string) string {
	var parts []string
	current := accountID
	for {
		resp, err := orgAPI.ListParents(ctx, &organizations.ListParentsInput{
			ChildId: awsv2.String(current),
		})
		if err != nil || len(resp.Parents) == 0 {
			break
		}
		parent := resp.Parents[0]
		pid := awsv2.ToString(parent.Id)
		if parent.Type == types.ParentTypeRoot {
			parts = append([]string{"root"}, parts...)
			break
		}
		ouName := pid
		if d, err := orgAPI.DescribeOrganizationalUnit(ctx, &organizations.DescribeOrganizationalUnitInput{
			OrganizationalUnitId: awsv2.String(pid),
		}); err == nil && d.OrganizationalUnit != nil {
			ouName = awsv2.ToString(d.OrganizationalUnit.Name)
		}
		parts = append([]string{ouName}, parts...)
		current = pid
	}
	if len(parts) == 0 {
		return "unknown"
	}
	return fmt.Sprintf("/%s", joinPath(parts))
}

func joinPath(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += "/" + parts[i]
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
