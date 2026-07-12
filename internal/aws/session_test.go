package aws

import (
	"context"
	"testing"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
)

func TestAssumeAccountCachesByAccountID(t *testing.T) {
	manager := NewSessionManagerFromConfig(awsv2.Config{Region: "us-east-1"}, "CloudriftAuditRole")

	first, err := manager.AssumeAccount(context.Background(), "123456789012")
	if err != nil {
		t.Fatalf("first assume failed: %v", err)
	}
	second, err := manager.AssumeAccount(context.Background(), "123456789012")
	if err != nil {
		t.Fatalf("second assume failed: %v", err)
	}

	if first.Region != second.Region {
		t.Fatalf("expected cached config consistency")
	}
	if len(manager.cache) != 1 {
		t.Fatalf("expected one cached entry, got %d", len(manager.cache))
	}
}

func TestAssumeAccountNoAssumeModeReturnsBaseConfig(t *testing.T) {
	// Empty role name = no-assume mode: return base credentials directly, never assume
	// a role and never populate the cache.
	base := awsv2.Config{Region: "eu-west-1"}
	manager := NewSessionManagerFromConfig(base, "")

	got, err := manager.AssumeAccount(context.Background(), "123456789012")
	if err != nil {
		t.Fatalf("no-assume AssumeAccount failed: %v", err)
	}
	if got.Region != base.Region {
		t.Fatalf("expected base config region %q, got %q", base.Region, got.Region)
	}
	if len(manager.cache) != 0 {
		t.Fatalf("no-assume mode must not cache assume-role sessions, got %d entries", len(manager.cache))
	}
}
