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
