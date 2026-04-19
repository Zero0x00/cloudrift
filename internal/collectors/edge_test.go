package collectors

import "testing"

func TestBucketFromOrigin(t *testing.T) {
	if got := bucketFromOrigin("mybucket.s3.amazonaws.com"); got != "mybucket" {
		t.Fatalf("expected mybucket, got %s", got)
	}
}
