package graph

import (
	"strings"
	"testing"
)

func TestSchemaStatements_ContainsConstraintsAndVectorIndex(t *testing.T) {
	stmts := SchemaStatements()
	if len(stmts) < 3 {
		t.Fatalf("expected at least 3 schema statements, got %d", len(stmts))
	}
	joined := strings.Join(stmts, "\n")
	for _, needle := range []string{
		"AwsAccount",
		"account_id",
		"UNIQUE",
		"Finding",
		"VECTOR INDEX",
		"finding_embeddings",
		"embedding",
		"384",
		"cosine",
	} {
		if !strings.Contains(joined, needle) {
			t.Errorf("expected schema to contain %q\n%s", needle, joined)
		}
	}
}

func TestSchemaStatements_IdempotentKeywords(t *testing.T) {
	for _, s := range SchemaStatements() {
		if !strings.Contains(s, "IF NOT EXISTS") {
			t.Errorf("each schema statement must be idempotent (IF NOT EXISTS): %q", s)
		}
	}
}

func TestSchemaStatements_OrderStable(t *testing.T) {
	a := strings.Join(SchemaStatements(), "\n")
	b := strings.Join(SchemaStatements(), "\n")
	if a != b {
		t.Fatal("SchemaStatements must return stable ordering")
	}
}
