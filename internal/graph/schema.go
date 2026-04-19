// Package graph provides optional Neo4j export for scan artifacts (Phase 3).
// It does not replace JSON flat files; callers opt in explicitly.
package graph

// VectorIndexFindingEmbeddings is the Neo4j vector index name used by schema DDL and RAG retrieval.
const VectorIndexFindingEmbeddings = "finding_embeddings"

// SchemaStatements returns idempotent Cypher DDL for the Phase 3 graph model
// (constraints + vector index). Execute in order; each statement uses IF NOT EXISTS.
//
// The vector index uses 384 dimensions (Neo4j cosine). Finding.embedding is populated in-memory
// by graph.AttachFindingsEmbeddings + graph.EmbeddingProvider (OpenAI text-embedding-3-small with
// dimensions=384 by default); it is never written to flat JSON (models.Finding json:"-").
func SchemaStatements() []string {
	return []string{
		`CREATE CONSTRAINT account_id IF NOT EXISTS FOR (a:AwsAccount) REQUIRE a.account_id IS UNIQUE`,
		`CREATE CONSTRAINT finding_id IF NOT EXISTS FOR (f:Finding) REQUIRE f.id IS UNIQUE`,
		"CREATE VECTOR INDEX " + VectorIndexFindingEmbeddings + " IF NOT EXISTS FOR (f:Finding) ON (f.embedding) OPTIONS {indexConfig: {`vector.dimensions`: 384, `vector.similarity_function`: 'cosine'}}",
	}
}
