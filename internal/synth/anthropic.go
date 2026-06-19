package synth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Zero0x00/cloudrift/internal/config"
)

// Raw net/http is used deliberately (not the Anthropic Go SDK) to match how this project
// already integrates an LLM provider — internal/graph/embedder.go calls OpenAI over
// net/http — and to avoid pulling a large SDK dependency for one endpoint. The Messages
// API shape (x-api-key, anthropic-version, /v1/messages, content[].text) is stable.

const (
	anthropicDefaultBaseURL = "https://api.anthropic.com"
	anthropicVersion        = "2023-06-01"
	anthropicRequestTimeout = 60 * time.Second
)

// httpDoer is the seam for tests (httptest server / fake transport).
type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type anthropicSynth struct {
	client    httpDoer
	baseURL   string
	apiKey    string
	model     string
	maxTokens int
}

func apiKeyFromEnv(envName string) string {
	envName = strings.TrimSpace(envName)
	if envName == "" {
		envName = "ANTHROPIC_API_KEY"
	}
	return strings.TrimSpace(os.Getenv(envName))
}

func newAnthropic(cfg *config.Config, key string) *anthropicSynth {
	model := strings.TrimSpace(cfg.Synthesis.Model)
	if model == "" {
		model = "claude-opus-4-8"
	}
	maxTokens := cfg.Synthesis.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2048
	}
	return &anthropicSynth{
		client:    &http.Client{Timeout: anthropicRequestTimeout},
		baseURL:   anthropicDefaultBaseURL,
		apiKey:    key,
		model:     model,
		maxTokens: maxTokens,
	}
}

func (a *anthropicSynth) Available() bool { return a != nil && a.apiKey != "" }

const synthSystemPrompt = `You are a cloud-security analyst assistant for Cloudrift, a tool that finds orphaned edge assets (subdomain-takeover risk) and risky external IAM trust. Answer the operator's question using ONLY the retrieved findings provided. Ground every claim in a specific finding and cite it by its ID in brackets, e.g. [a1b2c3]. Be concise and action-oriented: lead with the answer, then the supporting findings. If the findings are insufficient to answer, say so plainly rather than speculating. Do not invent findings, ARNs, or accounts that are not in the provided context.`

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system"`
	Thinking  map[string]any     `json:"thinking,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Model      string                  `json:"model"`
	StopReason string                  `json:"stop_reason"`
	Content    []anthropicContentBlock `json:"content"`
}

func (a *anthropicSynth) Synthesize(ctx context.Context, question string, items []ContextItem) (Result, error) {
	if !a.Available() {
		return Result{Used: false}, nil
	}
	if strings.TrimSpace(question) == "" || len(items) == 0 {
		// Nothing to ground on — let the caller fall back to retrieval-only.
		return Result{Used: false}, nil
	}

	body := anthropicRequest{
		Model:     a.model,
		MaxTokens: a.maxTokens,
		System:    synthSystemPrompt,
		Thinking:  map[string]any{"type": "adaptive"},
		Messages: []anthropicMessage{
			{Role: "user", Content: buildPrompt(question, items)},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return Result{}, fmt.Errorf("synth: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(a.baseURL, "/")+"/v1/messages", bytes.NewReader(raw))
	if err != nil {
		return Result{}, fmt.Errorf("synth: build request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := a.client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("synth: request failed: %w", err)
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("synth: anthropic returned %d: %s", resp.StatusCode, truncate(string(payload), 300))
	}

	var parsed anthropicResponse
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return Result{}, fmt.Errorf("synth: parse response: %w", err)
	}
	// A safety refusal returns 200 with stop_reason "refusal" and (often) no text —
	// degrade to retrieval-only rather than erroring the whole query.
	if parsed.StopReason == "refusal" {
		return Result{Used: false, Model: parsed.Model}, nil
	}
	answer := extractText(parsed.Content)
	if strings.TrimSpace(answer) == "" {
		return Result{Used: false, Model: parsed.Model}, nil
	}
	return Result{Answer: strings.TrimSpace(answer), Model: parsed.Model, Used: true}, nil
}

func extractText(blocks []anthropicContentBlock) string {
	var b strings.Builder
	for _, blk := range blocks {
		if blk.Type == "text" {
			b.WriteString(blk.Text)
		}
	}
	return b.String()
}

func buildPrompt(question string, items []ContextItem) string {
	var b strings.Builder
	b.WriteString("Question: ")
	b.WriteString(strings.TrimSpace(question))
	b.WriteString("\n\nRetrieved findings:\n")
	for _, it := range items {
		fmt.Fprintf(&b, "- [%s] (%s/%s) %s\n", it.ID, strings.TrimSpace(it.Severity), strings.TrimSpace(it.Module), strings.TrimSpace(it.Title))
		if d := strings.TrimSpace(it.Detail); d != "" {
			fmt.Fprintf(&b, "    detail: %s\n", d)
		}
		if r := strings.TrimSpace(it.Recommendation); r != "" {
			fmt.Fprintf(&b, "    recommendation: %s\n", r)
		}
	}
	b.WriteString("\nAnswer the question grounded only in these findings, citing IDs in brackets.")
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
