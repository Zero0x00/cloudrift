package synth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Zero0x00/cloudrift/internal/config"
)

func testProvider(t *testing.T, handler http.HandlerFunc) *anthropicSynth {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &anthropicSynth{
		client:    srv.Client(),
		baseURL:   srv.URL,
		apiKey:    "test-key",
		model:     "claude-opus-4-8",
		maxTokens: 1024,
	}
}

func sampleItems() []ContextItem {
	return []ContextItem{
		{ID: "abc123", Title: "docs.example.com -> reclaimable", Severity: "critical", Module: "orphaned_edge", Recommendation: "Remove DNS record"},
	}
}

func TestAnthropicSynthesize_RequestShapeAndParse(t *testing.T) {
	var gotPath, gotKey, gotVersion string
	var gotBody anthropicRequest

	p := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"model":"claude-opus-4-8","stop_reason":"end_turn","content":[{"type":"thinking","text":""},{"type":"text","text":"The takeover risk is [abc123]."}]}`))
	})

	res, err := p.Synthesize(context.Background(), "What is the biggest takeover risk?", sampleItems())
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}
	if !res.Used || res.Answer != "The takeover risk is [abc123]." {
		t.Fatalf("unexpected result: %+v", res)
	}
	if gotPath != "/v1/messages" {
		t.Errorf("path: want /v1/messages, got %s", gotPath)
	}
	if gotKey != "test-key" || gotVersion != anthropicVersion {
		t.Errorf("headers: key=%q version=%q", gotKey, gotVersion)
	}
	if gotBody.Model != "claude-opus-4-8" || gotBody.MaxTokens != 1024 || len(gotBody.Messages) != 1 {
		t.Errorf("request body wrong: %+v", gotBody)
	}
}

func TestAnthropicSynthesize_RefusalDegrades(t *testing.T) {
	p := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"model":"claude-opus-4-8","stop_reason":"refusal","content":[]}`))
	})
	res, err := p.Synthesize(context.Background(), "q", sampleItems())
	if err != nil {
		t.Fatalf("refusal should not error: %v", err)
	}
	if res.Used {
		t.Fatalf("refusal should degrade to Used=false, got %+v", res)
	}
}

func TestAnthropicSynthesize_HTTPErrorReturnsError(t *testing.T) {
	p := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	})
	if _, err := p.Synthesize(context.Background(), "q", sampleItems()); err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestAnthropicSynthesize_NoItemsOrQuestionDegrades(t *testing.T) {
	p := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not call the API with no grounding")
	})
	if res, _ := p.Synthesize(context.Background(), "q", nil); res.Used {
		t.Error("no items should degrade")
	}
	if res, _ := p.Synthesize(context.Background(), "", sampleItems()); res.Used {
		t.Error("no question should degrade")
	}
}

func TestNew_NoKeyReturnsNoop(t *testing.T) {
	cfg := config.Default()
	t.Setenv(cfg.Synthesis.APIKeyEnv, "")
	s := New(cfg)
	if s.Available() {
		t.Fatal("expected no-op (unavailable) without API key")
	}
}

func TestNew_WithKeyReturnsAnthropic(t *testing.T) {
	cfg := config.Default()
	t.Setenv(cfg.Synthesis.APIKeyEnv, "sk-test")
	s := New(cfg)
	if !s.Available() {
		t.Fatal("expected available synthesizer with API key set")
	}
}

func TestNew_UnknownProviderReturnsNoop(t *testing.T) {
	cfg := config.Default()
	cfg.Synthesis.Provider = "gemini" // not yet implemented
	t.Setenv(cfg.Synthesis.APIKeyEnv, "sk-test")
	if New(cfg).Available() {
		t.Fatal("unknown provider should be no-op")
	}
}
