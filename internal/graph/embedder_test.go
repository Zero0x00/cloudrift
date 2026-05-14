package graph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Zero0x00/cloudrift/internal/config"
	"github.com/Zero0x00/cloudrift/internal/models"
)

func vec384(seed float64) []float32 {
	v := make([]float32, ExpectedVectorDimensions)
	for i := range v {
		v[i] = float32(math.Sin(seed+float64(i)*0.01)) * 0.01
	}
	return v
}

func TestFindingEmbeddingText_StableFieldOrder(t *testing.T) {
	f := models.Finding{
		ID: "f1", Title: "t", Severity: models.SeverityHigh, Module: models.ModuleOrphanedEdge,
		Claimability: models.ClaimDangling, AffectedARN: "arn:a", AccountID: "111",
		AccountName: "acct", OUPath: "/ou", Team: "sec", Hostname: "h.example",
		Impact: "imp", Recommendation: "rec", RemediationCmd: "cmd", ScanID: "s1",
	}
	a := FindingEmbeddingText(f)
	b := FindingEmbeddingText(f)
	if a != b {
		t.Fatalf("deterministic text differed")
	}
	if !strings.Contains(a, "f1") || !strings.Contains(a, "h.example") {
		t.Fatalf("unexpected payload: %q", a)
	}
	// Evidence must not appear (omitted by design).
	if strings.Contains(strings.ToLower(a), "evidence") {
		t.Fatal("evidence must not be embedded in text")
	}
}

func TestFindingEmbeddingText_ReordersFieldsDoesNotChangeWhenFieldValuesIdentical(t *testing.T) {
	// Same logical finding built twice — text must match.
	f1 := models.Finding{ID: "x", Title: "y", ScanID: "s", AccountID: "1"}
	f2 := models.Finding{ID: "x", Title: "y", ScanID: "s", AccountID: "1"}
	if FindingEmbeddingText(f1) != FindingEmbeddingText(f2) {
		t.Fatal("expected identical embedding text")
	}
}

func TestFindingJSONOmitsEmbedding(t *testing.T) {
	f := models.Finding{
		ID: "id1", Title: "T", ScanID: "s", AccountID: "1",
		Embedding: vec384(1),
	}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["embedding"]; ok {
		t.Fatalf("embedding must not appear in JSON, got keys %v", m)
	}
}

func TestNewEmbeddingProvider_EmptyProviderStringUsesOpenAI(t *testing.T) {
	cfg := config.Default()
	cfg.Embeddings.Provider = ""
	p, pm, err := NewEmbeddingProvider(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.(openAIEmbeddingProvider); !ok {
		t.Fatalf("empty provider must normalize to openai, got %T", p)
	}
	if pm.Provider != "openai" || pm.Model != "text-embedding-3-small" || pm.Dimensions != ExpectedVectorDimensions {
		t.Fatalf("unexpected ProviderMeta: %+v", pm)
	}
}

func TestNewEmbeddingProvider_DefaultOpenAIAndExplicitLocal(t *testing.T) {
	cfg := config.Default()
	p, pm, err := NewEmbeddingProvider(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.(openAIEmbeddingProvider); !ok {
		t.Fatalf("expected default openai provider, got %T", p)
	}
	if pm.Provider != "openai" || pm.Dimensions != ExpectedVectorDimensions {
		t.Fatalf("unexpected default meta: %+v", pm)
	}

	cfg.Embeddings.Provider = "local"
	pLocal, pmLocal, err := NewEmbeddingProvider(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := pLocal.(localEmbeddingProvider); !ok {
		t.Fatalf("expected local provider, got %T", pLocal)
	}
	if pmLocal.Provider != "local" || pmLocal.Model != "all-MiniLM-L6-v2" || pmLocal.Dimensions != ExpectedVectorDimensions {
		t.Fatalf("unexpected local meta: %+v", pmLocal)
	}

	cfg.Embeddings.Provider = "openai"
	cfg.Embeddings.OpenaiAPIKeyEnv = "K_ENV"
	p2, _, err := NewEmbeddingProvider(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p2.(openAIEmbeddingProvider); !ok {
		t.Fatalf("expected openai provider, got %T", p2)
	}

	cfg.Embeddings.Provider = "bogus"
	if _, _, err := NewEmbeddingProvider(cfg); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestNewEmbeddingProvider_OpenAIRequiresKeyEnvName(t *testing.T) {
	cfg := config.Default()
	cfg.Embeddings.Provider = "openai"
	cfg.Embeddings.OpenaiAPIKeyEnv = ""
	if _, _, err := NewEmbeddingProvider(cfg); err == nil {
		t.Fatal("expected error for empty openai_api_key_env")
	}
}

func TestLocalProviderEmbedFailsCleanly(t *testing.T) {
	cfg := config.Default()
	cfg.Embeddings.Provider = "local"
	p, _, err := NewEmbeddingProvider(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Embed(context.Background(), []string{"a"})
	if err == nil || !errors.Is(err, ErrLocalEmbeddingsUnavailable) {
		t.Fatalf("expected local unavailable error, got %v", err)
	}
}

func TestAttachFindingsEmbeddings_WithOpenAIProviderMockBatch(t *testing.T) {
	emb := make([]float64, ExpectedVectorDimensions)
	for i := range emb {
		emb[i] = 0.01 * float64(i%5)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Input) != 2 {
			t.Fatalf("expected 2 inputs, got %d", len(body.Input))
		}
		data := []any{
			map[string]any{"object": "embedding", "index": 0, "embedding": emb},
			map[string]any{"object": "embedding", "index": 1, "embedding": emb},
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": data})
	}))
	defer srv.Close()
	t.Setenv("OPENAI_BATCH_KEY", "sk")
	p := openAIEmbeddingProvider{
		apiKeyEnv:  "OPENAI_BATCH_KEY",
		httpClient: srv.Client(),
		model:      "text-embedding-3-small",
		baseURL:    srv.URL + "/v1/embeddings",
		dimensions: ExpectedVectorDimensions,
	}
	findings := []models.Finding{
		{ID: "a", Title: "t1", ScanID: "s", AccountID: "1"},
		{ID: "b", Title: "t2", ScanID: "s", AccountID: "1"},
	}
	if err := AttachFindingsEmbeddings(context.Background(), p, findings); err != nil {
		t.Fatal(err)
	}
	for i := range findings {
		if len(findings[i].Embedding) != ExpectedVectorDimensions {
			t.Fatalf("finding %d: bad len %d", i, len(findings[i].Embedding))
		}
	}
}

func TestAttachFindingsEmbeddings_InPlace(t *testing.T) {
	findings := []models.Finding{
		{ID: "a", Title: "t1", ScanID: "s", AccountID: "1"},
		{ID: "b", Title: "t2", ScanID: "s", AccountID: "1"},
	}
	mock := &mockEmbedProvider{out: [][]float32{vec384(0), vec384(1)}}
	if err := AttachFindingsEmbeddings(context.Background(), mock, findings); err != nil {
		t.Fatal(err)
	}
	if len(findings[0].Embedding) != ExpectedVectorDimensions || len(findings[1].Embedding) != ExpectedVectorDimensions {
		t.Fatal("embeddings not attached")
	}
}

func TestAttachFindingsEmbeddings_ProviderError(t *testing.T) {
	findings := []models.Finding{{ID: "a", Title: "t", ScanID: "s", AccountID: "1"}}
	mock := &mockEmbedProvider{err: fmt.Errorf("provider boom")}
	if err := AttachFindingsEmbeddings(context.Background(), mock, findings); err == nil {
		t.Fatal("expected error")
	}
	if len(findings[0].Embedding) != 0 {
		t.Fatal("finding should not be mutated on error")
	}
}

func TestAttachFindingsEmbeddings_WrongDimFromProvider(t *testing.T) {
	findings := []models.Finding{{ID: "a", Title: "t", ScanID: "s", AccountID: "1"}}
	short := []float32{1, 2, 3}
	mock := &mockEmbedProvider{out: [][]float32{short}}
	if err := AttachFindingsEmbeddings(context.Background(), mock, findings); err == nil {
		t.Fatal("expected dimension error")
	}
}

func TestOpenAIProvider_MissingAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	}))
	defer srv.Close()
	t.Setenv("EMPTY_OPENAI", "")
	p := openAIEmbeddingProvider{
		apiKeyEnv:  "EMPTY_OPENAI",
		httpClient: srv.Client(),
		model:      "m",
		baseURL:    srv.URL + "/v1/embeddings",
		dimensions: ExpectedVectorDimensions,
	}
	_, err := p.Embed(context.Background(), []string{"x"})
	if err == nil || !strings.Contains(err.Error(), "EMPTY_OPENAI") {
		t.Fatalf("expected missing key error, got %v", err)
	}
}

func TestOpenAIProvider_DimensionMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"object": "list",
			"data": []any{
				map[string]any{"object": "embedding", "index": 0, "embedding": []float64{0.1, 0.2, 0.3}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	t.Setenv("OPENAI_TEST_KEY", "sk-test")
	p := openAIEmbeddingProvider{
		apiKeyEnv:  "OPENAI_TEST_KEY",
		httpClient: srv.Client(),
		model:      "text-embedding-3-small",
		baseURL:    srv.URL + "/v1/embeddings",
		dimensions: ExpectedVectorDimensions,
	}
	_, err := p.Embed(context.Background(), []string{"hello"})
	if err == nil || !strings.Contains(err.Error(), "dimension mismatch") {
		t.Fatalf("expected dimension mismatch, got %v", err)
	}
}

func TestOpenAIProvider_RequestIncludesDimensions384(t *testing.T) {
	emb := make([]float64, ExpectedVectorDimensions)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		dim, ok := body["dimensions"].(float64)
		if !ok || int(dim) != ExpectedVectorDimensions {
			t.Fatalf("expected dimensions=%d in request body, got %#v", ExpectedVectorDimensions, body["dimensions"])
		}
		if body["encoding_format"] != "float" {
			t.Fatalf("expected encoding_format float, got %v", body["encoding_format"])
		}
		resp := map[string]any{
			"object": "list",
			"data": []any{
				map[string]any{"object": "embedding", "index": 0, "embedding": emb},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	t.Setenv("OPENAI_TEST_KEY", "sk-test")
	p := openAIEmbeddingProvider{
		apiKeyEnv:  "OPENAI_TEST_KEY",
		httpClient: srv.Client(),
		model:      "text-embedding-3-small",
		baseURL:    srv.URL + "/v1/embeddings",
		dimensions: ExpectedVectorDimensions,
	}
	if _, err := p.Embed(context.Background(), []string{"hello"}); err != nil {
		t.Fatal(err)
	}
}

func TestOpenAIProvider_Success384ViaMockServer(t *testing.T) {
	emb := make([]float64, ExpectedVectorDimensions)
	for i := range emb {
		emb[i] = 0.001 * float64(i%7)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method %s", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if int(body["dimensions"].(float64)) != ExpectedVectorDimensions {
			t.Fatalf("missing or wrong dimensions: %#v", body["dimensions"])
		}
		resp := map[string]any{
			"object": "list",
			"data": []any{
				map[string]any{"object": "embedding", "index": 0, "embedding": emb},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	t.Setenv("OPENAI_TEST_KEY", "sk-test")
	p := openAIEmbeddingProvider{
		apiKeyEnv:  "OPENAI_TEST_KEY",
		httpClient: srv.Client(),
		model:      "text-embedding-3-small",
		baseURL:    srv.URL + "/v1/embeddings",
		dimensions: ExpectedVectorDimensions,
	}
	out, err := p.Embed(context.Background(), []string{"hello"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || len(out[0]) != ExpectedVectorDimensions {
		t.Fatalf("bad vector shape: %d", len(out[0]))
	}
}

func TestAttachFindingsEmbeddings_NilProvider(t *testing.T) {
	err := AttachFindingsEmbeddings(context.Background(), nil, []models.Finding{{ID: "x"}})
	if err == nil || !strings.Contains(err.Error(), "nil") {
		t.Fatalf("expected nil provider error, got %v", err)
	}
}

func TestAttachFindingsEmbeddings_EmptyFindingsNoOp(t *testing.T) {
	mock := &mockEmbedProvider{err: fmt.Errorf("should not call")}
	if err := AttachFindingsEmbeddings(context.Background(), mock, nil); err != nil {
		t.Fatal(err)
	}
	if err := AttachFindingsEmbeddings(context.Background(), mock, []models.Finding{}); err != nil {
		t.Fatal(err)
	}
}

func TestOpenAIEmbeddingProvider_HTTPErrorTruncatesBody(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key-not-used-for-network")
	longBody := strings.Repeat("x", 5000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(longBody))
	}))
	defer srv.Close()
	p := openAIEmbeddingProvider{
		apiKeyEnv:  "OPENAI_API_KEY",
		httpClient: srv.Client(),
		model:      "text-embedding-3-small",
		baseURL:    srv.URL,
		dimensions: ExpectedVectorDimensions,
	}
	_, err := p.Embed(context.Background(), []string{"a"})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if strings.Contains(msg, longBody) {
		t.Fatal("full response body should not appear in error")
	}
	if !strings.Contains(msg, "truncated") {
		t.Fatalf("expected truncation marker: %s", msg)
	}
	if len(msg) > 2000 {
		t.Fatalf("error message unexpectedly long: %d", len(msg))
	}
}

type mockEmbedProvider struct {
	out [][]float32
	err error
	n   int
}

func (m *mockEmbedProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	m.n++
	if m.err != nil {
		return nil, m.err
	}
	if len(m.out) != len(texts) {
		return nil, fmt.Errorf("mock: len mismatch")
	}
	return m.out, nil
}
