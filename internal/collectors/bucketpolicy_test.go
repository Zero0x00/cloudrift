package collectors

import "testing"

func TestExternalGrantsFromBucketPolicy(t *testing.T) {
	const owner = "111111111111"

	t.Run("external account read", func(t *testing.T) {
		pol := `{"Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::222222222222:root"},"Action":["s3:GetObject"],"Resource":"*"}]}`
		g := externalGrantsFromBucketPolicy(owner, pol)
		if len(g) != 1 {
			t.Fatalf("expected 1 grant, got %d", len(g))
		}
		if g[0].principalType != "aws_account" || g[0].externalAccountID != "222222222222" || g[0].write || g[0].isPublic {
			t.Fatalf("unexpected grant: %+v", g[0])
		}
	})

	t.Run("external account write", func(t *testing.T) {
		pol := `{"Statement":[{"Effect":"Allow","Principal":{"AWS":"222222222222"},"Action":["s3:PutObject","s3:DeleteObject"],"Resource":"*"}]}`
		g := externalGrantsFromBucketPolicy(owner, pol)
		if len(g) != 1 || !g[0].write {
			t.Fatalf("expected 1 write grant, got %+v", g)
		}
	})

	t.Run("public string principal", func(t *testing.T) {
		pol := `{"Statement":[{"Effect":"Allow","Principal":"*","Action":"s3:GetObject","Resource":"*"}]}`
		g := externalGrantsFromBucketPolicy(owner, pol)
		if len(g) != 1 || !g[0].isPublic || g[0].principalType != "public" {
			t.Fatalf("expected public grant, got %+v", g)
		}
	})

	t.Run("public AWS-wildcard write", func(t *testing.T) {
		pol := `{"Statement":[{"Effect":"Allow","Principal":{"AWS":"*"},"Action":"s3:*","Resource":"*"}]}`
		g := externalGrantsFromBucketPolicy(owner, pol)
		if len(g) != 1 || !g[0].isPublic || !g[0].write {
			t.Fatalf("expected public write grant, got %+v", g)
		}
	})

	t.Run("same-account grant is ignored", func(t *testing.T) {
		pol := `{"Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::111111111111:root"},"Action":"s3:GetObject","Resource":"*"}]}`
		if g := externalGrantsFromBucketPolicy(owner, pol); len(g) != 0 {
			t.Fatalf("expected same-account grant ignored, got %+v", g)
		}
	})

	t.Run("deny statement is ignored", func(t *testing.T) {
		pol := `{"Statement":[{"Effect":"Deny","Principal":"*","Action":"s3:*","Resource":"*"}]}`
		if g := externalGrantsFromBucketPolicy(owner, pol); len(g) != 0 {
			t.Fatalf("expected deny ignored, got %+v", g)
		}
	})
}
