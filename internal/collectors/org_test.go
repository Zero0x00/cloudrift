package collectors

import (
	"context"
	"testing"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	orgtypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"

	internalaws "cloudrift/internal/aws"
	"cloudrift/internal/config"
)

type fakeOrgAPI struct{}

func (f *fakeOrgAPI) ListAccounts(_ context.Context, _ *organizations.ListAccountsInput, _ ...func(*organizations.Options)) (*organizations.ListAccountsOutput, error) {
	return &organizations.ListAccountsOutput{
		Accounts: []orgtypes.Account{{Id: awsv2.String("111122223333"), Name: awsv2.String("prod")}},
	}, nil
}

func (f *fakeOrgAPI) ListTagsForResource(_ context.Context, in *organizations.ListTagsForResourceInput, _ ...func(*organizations.Options)) (*organizations.ListTagsForResourceOutput, error) {
	_ = in
	return &organizations.ListTagsForResourceOutput{
		Tags: []orgtypes.Tag{
			{Key: awsv2.String("Team"), Value: awsv2.String("platform")},
			{Key: awsv2.String("Owner"), Value: awsv2.String("secops")},
		},
	}, nil
}

func (f *fakeOrgAPI) ListParents(_ context.Context, in *organizations.ListParentsInput, _ ...func(*organizations.Options)) (*organizations.ListParentsOutput, error) {
	if awsv2.ToString(in.ChildId) == "111122223333" {
		return &organizations.ListParentsOutput{
			Parents: []orgtypes.Parent{{Id: awsv2.String("ou-abcd-12345678"), Type: orgtypes.ParentTypeOrganizationalUnit}},
		}, nil
	}
	return &organizations.ListParentsOutput{
		Parents: []orgtypes.Parent{{Id: awsv2.String("r-root"), Type: orgtypes.ParentTypeRoot}},
	}, nil
}

func (f *fakeOrgAPI) DescribeOrganizationalUnit(_ context.Context, _ *organizations.DescribeOrganizationalUnitInput, _ ...func(*organizations.Options)) (*organizations.DescribeOrganizationalUnitOutput, error) {
	return &organizations.DescribeOrganizationalUnitOutput{
		OrganizationalUnit: &orgtypes.OrganizationalUnit{Name: awsv2.String("engineering")},
	}, nil
}

func TestCollectAccounts_MapsTagsAndOUPath(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	base := awsv2.Config{
		Region:      "us-east-1",
		Credentials: awsv2.NewCredentialsCache(credentials.NewStaticCredentialsProvider("id", "secret", "token")),
	}
	sm := internalaws.NewSessionManagerFromConfig(base, cfg.AWS.OrgRoleName)
	accounts, err := CollectAccounts(ctx, cfg, &fakeOrgAPI{}, sm)
	if err != nil {
		t.Fatalf("collect accounts failed: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected one account, got %d", len(accounts))
	}
	got := accounts[0]
	if got.Team != "platform" || got.Owner != "secops" {
		t.Fatalf("unexpected tags: team=%s owner=%s", got.Team, got.Owner)
	}
	if got.OUPath == "unknown" || got.OUPath == "" {
		t.Fatalf("expected ou path, got %q", got.OUPath)
	}
}
