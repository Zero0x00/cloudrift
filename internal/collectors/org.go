package collectors

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/organizations/types"

	"github.com/Zero0x00/cloudrift/internal/aws"
	"github.com/Zero0x00/cloudrift/internal/config"
)

// Coverage records which org accounts a scan actually reached. It is the trust signal
// for findings: a reclaimable verdict is only safe when coverage is complete, because a
// bucket "missing" from scanned accounts could still be owned by an account we skipped.
type Coverage struct {
	Discovered int               `json:"discovered"`        // accounts returned by Organizations
	Scanned    []string          `json:"scanned"`           // account IDs successfully assumed
	Failed     map[string]string `json:"failed,omitempty"`  // account ID -> assume error
}

// Complete reports whether every discovered account was successfully assumed.
func (c Coverage) Complete() bool { return len(c.Failed) == 0 }

// warnPartial logs (without aborting) when a collector produced partial results because
// some per-account API calls failed. Collectors stay resilient: one account's missing
// permission or throttle must not discard every other account's findings.
func warnPartial(collector string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: %s collector: partial results, some accounts/calls failed: %v\n", collector, err)
	}
}

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

func CollectAccounts(ctx context.Context, cfg *config.Config, orgAPI OrganizationsAPI, sm *aws.SessionManager) ([]Account, Coverage, error) {
	var accounts []types.Account
	p := organizations.NewListAccountsPaginator(orgAPI, &organizations.ListAccountsInput{})
	for p.HasMorePages() {
		out, err := p.NextPage(ctx)
		if err != nil {
			// Listing the org itself is fatal — without it there is nothing to scan.
			return nil, Coverage{}, err
		}
		accounts = append(accounts, out.Accounts...)
	}

	out := make([]Account, len(accounts))
	failed := make(map[string]string)
	sem := make(chan struct{}, max(1, cfg.Scan.RoleAssumptionConcurrency))
	var wg sync.WaitGroup
	var mu sync.Mutex
	for i, a := range accounts {
		i, a := i, a
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			accID := awsv2.ToString(a.Id)
			session, err := sm.AssumeAccount(ctx, accID)
			if err != nil {
				// Resilient: an account we cannot assume (closed, missing audit role, SCP
				// denial) is recorded and skipped, never aborting the whole org scan.
				mu.Lock()
				failed[accID] = err.Error()
				mu.Unlock()
				return
			}

			out[i] = Account{
				ID:      accID,
				Name:    awsv2.ToString(a.Name),
				OUPath:  buildOUPath(ctx, orgAPI, accID),
				Team:    tagValue(ctx, orgAPI, accID, "Team"),
				Owner:   tagValue(ctx, orgAPI, accID, "Owner"),
				Contact: tagValue(ctx, orgAPI, accID, "Contact"),
				Session: &session,
			}
		}()
	}
	wg.Wait()

	// Compact to successfully-assumed accounts; failed entries are left zero-value.
	scanned := make([]Account, 0, len(out))
	scannedIDs := make([]string, 0, len(out))
	for _, acc := range out {
		if acc.Session == nil {
			continue
		}
		scanned = append(scanned, acc)
		scannedIDs = append(scannedIDs, acc.ID)
	}
	sort.Strings(scannedIDs)
	cov := Coverage{Discovered: len(accounts), Scanned: scannedIDs, Failed: failed}
	if len(failed) > 0 {
		fmt.Fprintf(os.Stderr, "WARN: account collection: %d/%d accounts could not be assumed and were skipped\n", len(failed), len(accounts))
	}
	return scanned, cov, nil
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
