package collectors

import (
	"testing"

	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func TestNormalizeBucketRegion(t *testing.T) {
	if got := normalizeBucketRegion(""); got != "us-east-1" {
		t.Fatalf("expected us-east-1 for empty region, got %s", got)
	}
	if got := normalizeBucketRegion(s3types.BucketLocationConstraintEu); got != "eu-west-1" {
		t.Fatalf("expected eu-west-1 for EU alias, got %s", got)
	}
	if got := normalizeBucketRegion(s3types.BucketLocationConstraintUsWest2); got != "us-west-2" {
		t.Fatalf("expected pass-through region, got %s", got)
	}
}
