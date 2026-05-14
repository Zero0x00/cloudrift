package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Zero0x00/cloudrift/internal/config"
	"github.com/Zero0x00/cloudrift/internal/graph"
	"github.com/Zero0x00/cloudrift/internal/models"
)

type testEmbedder struct{}

func (testEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	vec := make([]float32, graph.ExpectedVectorDimensions)
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = vec
	}
	return out, nil
}

type queryFakeRowReader struct {
	rows      []map[string]any
	countRows []map[string]any // optional second-phase count when hybrid returns no rows
	err       error
}

func (q *queryFakeRowReader) Read(ctx context.Context, cypher string, params map[string]any) ([]map[string]any, error) {
	if q.err != nil {
		return nil, q.err
	}
	if strings.Contains(cypher, "RETURN count(*) AS vector_global_match_count") {
		if q.countRows != nil {
			return q.countRows, nil
		}
		return []map[string]any{{"vector_global_match_count": int64(0)}}, nil
	}
	return q.rows, nil
}

func TestQueryCommandRegistered(t *testing.T) {
	root := newRootCommand()
	q, _, err := root.Find([]string{"query"})
	if err != nil || q == nil {
		t.Fatalf("query command: %v", err)
	}
	for _, name := range []string{"output-dir", "scan-id", "query", "format", "top-k", "require-stored-embedding-identity"} {
		if q.Flags().Lookup(name) == nil {
			t.Fatalf("expected query flag --%s", name)
		}
	}
}

func TestLoadScanMetadata(t *testing.T) {
	dir := t.TempDir()
	_, err := loadScanMetadata(dir)
	if err == nil || !strings.Contains(err.Error(), "scan-metadata.json") {
		t.Fatalf("expected missing metadata error, got %v", err)
	}

	meta := models.ScanSnapshot{ScanID: "s-1", Timestamp: models.ScanSnapshot{}.Timestamp}
	b, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(dir, "scan-metadata.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := loadScanMetadata(dir)
	if err != nil || got.ScanID != "s-1" {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestRunQueryRetrieval_Success(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	meta := models.ScanSnapshot{
		ScanID:              "scan-1",
		EmbeddingProvider:   "openai",
		EmbeddingModel:      "text-embedding-3-small",
		EmbeddingDimensions: graph.ExpectedVectorDimensions,
	}
	rows := &queryFakeRowReader{rows: []map[string]any{{
		"finding_id":                "f1",
		"title":                     "Idle LB",
		"severity":                  "high",
		"claimability":              "claimable",
		"monthly_direct_cost_usd":   float64(12),
		"recommendation":            "delete",
		"account_name":              "acct",
		"account_ou_path":           "/root",
		"account_team":              "net",
		"linked_arn":                "arn:aws:elasticloadbalancing:...",
		"score":                     float64(0.91),
		"vector_global_match_count": int64(3),
	}}}
	pm := graph.ProviderMeta{Provider: "openai", Model: "text-embedding-3-small", Dimensions: graph.ExpectedVectorDimensions}
	resp, err := runQueryRetrieval(ctx, cfg, meta, queryCLIOptions{
		QueryText: "unused cost",
		TopK:      5,
	}, nil, rows, testEmbedder{}, pm)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Hits) != 1 || resp.Hits[0].FindingID != "f1" {
		t.Fatalf("hits: %+v", resp.Hits)
	}
	if resp.LegacyEmbeddingUnverified {
		t.Fatal("expected verified lineage")
	}
	if resp.EmptyHint != graph.RAGEmptyHintNone {
		t.Fatalf("empty hint: %v", resp.EmptyHint)
	}
}

func TestRunQueryRetrieval_LegacyEmptyHintAndNotes(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	meta := models.ScanSnapshot{ScanID: "scan-1"}
	rows := &queryFakeRowReader{rows: nil, countRows: []map[string]any{{"vector_global_match_count": int64(0)}}}
	pm := graph.ProviderMeta{Provider: "openai", Model: "text-embedding-3-small", Dimensions: graph.ExpectedVectorDimensions}
	resp, err := runQueryRetrieval(ctx, cfg, meta, queryCLIOptions{QueryText: "q", TopK: 5}, nil, rows, testEmbedder{}, pm)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.LegacyEmbeddingUnverified {
		t.Fatal("expected legacy")
	}
	if resp.EmptyHint != graph.RAGEmptyHintNoVectorCandidates {
		t.Fatalf("hint %v", resp.EmptyHint)
	}
	if len(resp.OperatorNotes) == 0 {
		t.Fatal("expected operator notes")
	}
}

func TestRunQueryRetrieval_ProbeSaturatedAndScopeHint(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	meta := models.ScanSnapshot{
		ScanID:              "scan-1",
		EmbeddingProvider:   "openai",
		EmbeddingModel:      "text-embedding-3-small",
		EmbeddingDimensions: graph.ExpectedVectorDimensions,
	}
	// TopK 100 => vector_probe = min(100*20, 2000) = 2000; saturation when global count >= probe.
	const probeBudget = 2000
	rows := &queryFakeRowReader{rows: []map[string]any{{
		"finding_id":                "f1",
		"title":                     "t",
		"severity":                  "low",
		"claimability":              "unknown",
		"monthly_direct_cost_usd":   float64(0),
		"recommendation":            "",
		"account_name":              "",
		"account_ou_path":           "",
		"account_team":              "",
		"linked_arn":                "",
		"score":                     float64(0.5),
		"vector_global_match_count": int64(probeBudget),
	}}}
	pm := graph.ProviderMeta{Provider: "openai", Model: "text-embedding-3-small", Dimensions: graph.ExpectedVectorDimensions}
	resp, err := runQueryRetrieval(ctx, cfg, meta, queryCLIOptions{QueryText: "q", TopK: 100}, nil, rows, testEmbedder{}, pm)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.ProbeSaturated {
		t.Fatal("expected probe saturated")
	}
	found := false
	for _, n := range resp.OperatorNotes {
		if strings.Contains(strings.ToLower(n), "saturation") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected saturation note, got %#v", resp.OperatorNotes)
	}
}

func TestRunQueryRetrieval_RequireIdentityRejectsLegacy(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	meta := models.ScanSnapshot{ScanID: "scan-1"}
	rows := &queryFakeRowReader{}
	pm := graph.ProviderMeta{Provider: "openai", Model: "text-embedding-3-small", Dimensions: graph.ExpectedVectorDimensions}
	_, err := runQueryRetrieval(ctx, cfg, meta, queryCLIOptions{
		QueryText:                      "q",
		RequireStoredEmbeddingIdentity: true,
	}, nil, rows, testEmbedder{}, pm)
	if err == nil || !errors.Is(err, graph.ErrRAGMissingGraphEmbeddingIdentity) {
		t.Fatalf("expected ErrRAGMissingGraphEmbeddingIdentity, got %v", err)
	}
}

func TestRunQueryRetrieval_VectorIndexMissing(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	meta := models.ScanSnapshot{
		ScanID:              "scan-1",
		EmbeddingProvider:   "openai",
		EmbeddingModel:      "text-embedding-3-small",
		EmbeddingDimensions: graph.ExpectedVectorDimensions,
	}
	rows := &queryFakeRowReader{err: errors.New("There is no such vector index `finding_embeddings`")}
	pm := graph.ProviderMeta{Provider: "openai", Model: "text-embedding-3-small", Dimensions: graph.ExpectedVectorDimensions}
	_, err := runQueryRetrieval(ctx, cfg, meta, queryCLIOptions{QueryText: "q"}, nil, rows, testEmbedder{}, pm)
	if err == nil || !graph.IsRAGVectorIndexMissing(err) {
		t.Fatalf("expected missing index, got %v", err)
	}
}

func TestQueryRetrievalErrorSurfacesOperatorMessage(t *testing.T) {
	base := errors.Join(graph.ErrRAGVectorIndexMissing, errors.New("no such index"))
	wrapped := queryRetrievalError(base)
	if !strings.Contains(wrapped.Error(), graph.RAGVectorIndexOperatorMessage) {
		t.Fatalf("expected operator message in error: %v", wrapped)
	}
	if !graph.IsRAGVectorIndexMissing(wrapped) {
		t.Fatal("expected errors.Is to still detect sentinel")
	}
}

func TestWriteQueryJSON(t *testing.T) {
	resp := &graph.RAGRetrievalResponse{
		Hits:                   []graph.RAGRetrievalHit{{FindingID: "a", Title: "T", Score: 0.5}},
		VectorProbe:            128,
		VectorGlobalMatchCount: 1,
		EmptyHint:              graph.RAGEmptyHintNone,
	}
	var buf bytes.Buffer
	if err := writeQueryJSON(&buf, "qtext", "sid", 0, resp); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["empty_hint"] != "none" {
		t.Fatalf("empty_hint: %v", got["empty_hint"])
	}
	if got["answer_synthesis"] != "" {
		t.Fatalf("answer_synthesis should be empty stub")
	}
}

func TestWriteQueryHuman_LegacyStderr(t *testing.T) {
	resp := &graph.RAGRetrievalResponse{
		LegacyEmbeddingUnverified: true,
		Hits:                      nil,
		EmptyHint:                 graph.RAGEmptyHintNoVectorCandidates,
		VectorProbe:               64,
		OperatorNotes:             []string{"note one"},
	}
	var out, errOut bytes.Buffer
	if err := writeQueryHuman(&out, &errOut, "q", "s", 0, resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "LEGACY") {
		t.Fatalf("stdout: %s", out.String())
	}
	if !strings.Contains(errOut.String(), "warning") {
		t.Fatalf("stderr: %s", errOut.String())
	}
}

func TestValidateNeo4jConfigForQuery(t *testing.T) {
	cfg := config.Default()
	cfg.Neo4j.URI = ""
	if err := validateNeo4jConfigForQuery(cfg); err == nil {
		t.Fatal("expected error")
	}
}

func TestQueryCommandMutuallyExclusiveQuerySources(t *testing.T) {
	root := newRootCommand()
	root.SetArgs([]string{"query", "--query", "from-flag", "positional"})
	var stderr bytes.Buffer
	root.SetOut(io.Discard)
	root.SetErr(&stderr)
	root.SetContext(context.Background())
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "not both") {
		t.Fatalf("expected mutual exclusion error, got %v stderr=%q", err, stderr.String())
	}
}

func TestQueryCommandRequiresQueryText(t *testing.T) {
	root := newRootCommand()
	root.SetArgs([]string{"query", "--output-dir", t.TempDir()})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	err := root.Execute()
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "query text") {
		t.Fatalf("got %v", err)
	}
}
