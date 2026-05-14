// Embedding helpers for Phase 3 (optional graph vectors; not wired into default scan).
//
// DEFAULT AND OPERATIONAL PROVIDER: OpenAI (text-embedding-3-small, dimensions=384) — see
// config.Default().Embeddings and NewEmbeddingProvider. This matches the Neo4j vector index.
//
// LOCAL: Stub only today — embeddings.provider "local" always fails Embed. Add a real
// on-device path (e.g. bundled MiniLM/ONNX, same 384-dim contract) when local-first / air-gapped
// deployments matter; keep vectors aligned with ExpectedVectorDimensions and Neo4j index width.
//
// Retrieval paths validate provider/model/dimensions against stored scan identity
// (ValidateEmbeddingCompatibility) before vector search; errors do not include API keys.
// HTTP error bodies returned to callers are truncated to avoid leaking large or sensitive payloads.
package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/Zero0x00/cloudrift/internal/config"
	"github.com/Zero0x00/cloudrift/internal/models"
)

// ExpectedVectorDimensions matches the Neo4j vector index (all-MiniLM-L6-v2) in schema.go.
// Providers must return exactly this many dimensions for graph export compatibility.
const ExpectedVectorDimensions = 384

// openAIHTTPErrorBodyMaxRunes caps error text from non-2xx OpenAI responses (operator-facing).
const openAIHTTPErrorBodyMaxRunes = 300

func truncateForOperatorMessage(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if maxRunes <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…(truncated)"
}

// ErrLocalEmbeddingsUnavailable is returned by the local (planned-only) provider on every
// Embed call. Local is not an operational provider today — use OpenAI.
var ErrLocalEmbeddingsUnavailable = errors.New("graph: local embeddings are not available (planned MiniLM/ONNX path only; use OpenAI provider)")

// ErrOpenAIDimensionMismatch is returned when the OpenAI API returns a vector length
// other than ExpectedVectorDimensions (current Neo4j index is fixed at 384).
var ErrOpenAIDimensionMismatch = errors.New("graph: OpenAI embedding dimension mismatch vs Neo4j index (384)")

// EmbeddingProvider turns finding-derived text into dense vectors. Implementations must
// not log secrets (API keys, raw prompts beyond finding text).
type EmbeddingProvider interface {
	// Embed returns one vector per input string, same order and length as texts.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// FindingEmbeddingText builds a stable, deterministic string for embedding from public
// finding fields only (no secrets). Rules:
//   - Fields are joined with ASCII unit separator (0x1f) in a fixed column order so
//     adding new columns in the future is a deliberate schema change, not silent drift.
//   - Order: id, title, severity, module, claimability, affected_arn, account_id,
//     account_name, ou_path, team, hostname, impact, recommendation, remediation_command,
//     scan_id. Evidence is omitted (arbitrary JSON shape; avoid unstable serialization).
//   - Each field is strings.TrimSpace; empty fields remain as empty segments.
func FindingEmbeddingText(f models.Finding) string {
	parts := []string{
		strings.TrimSpace(f.ID),
		strings.TrimSpace(f.Title),
		string(f.Severity),
		string(f.Module),
		string(f.Claimability),
		strings.TrimSpace(f.AffectedARN),
		strings.TrimSpace(f.AccountID),
		strings.TrimSpace(f.AccountName),
		strings.TrimSpace(f.OUPath),
		strings.TrimSpace(f.Team),
		strings.TrimSpace(f.Hostname),
		strings.TrimSpace(f.Impact),
		strings.TrimSpace(f.Recommendation),
		strings.TrimSpace(f.RemediationCmd),
		strings.TrimSpace(f.ScanID),
	}
	return strings.Join(parts, "\x1f")
}

// NewEmbeddingProvider returns an EmbeddingProvider and ProviderMeta for cfg.Embeddings.provider.
//
// Operational today: only "openai" (see config defaults). OpenAI requests dimensions=384
// to align with the Neo4j vector index.
//
// Not operational: "local" is a planned stub (all-MiniLM-L6-v2 / ONNX not bundled); Embed
// always returns ErrLocalEmbeddingsUnavailable.
//
// Empty provider string is treated as "openai" to match config.Default() — do not default to local.
//
// ProviderMeta must be passed to SyncScanSnapshotEmbeddingMeta before graph export when embeddings
// are attached, and used with ValidateEmbeddingCompatibility before retrieval.
func NewEmbeddingProvider(cfg *config.Config) (EmbeddingProvider, ProviderMeta, error) {
	if cfg == nil {
		return nil, ProviderMeta{}, fmt.Errorf("graph: config is nil")
	}
	prov := strings.TrimSpace(strings.ToLower(cfg.Embeddings.Provider))
	if prov == "" {
		prov = "openai"
	}
	switch prov {
	case "local":
		model := strings.TrimSpace(cfg.Embeddings.LocalModel)
		if model == "" {
			model = "all-MiniLM-L6-v2"
		}
		pm := ProviderMeta{Provider: "local", Model: model, Dimensions: ExpectedVectorDimensions}
		return localEmbeddingProvider{model: model}, pm, nil
	case "openai":
		envName := strings.TrimSpace(cfg.Embeddings.OpenaiAPIKeyEnv)
		if envName == "" {
			return nil, ProviderMeta{}, fmt.Errorf("graph: embeddings.openai_api_key_env is empty")
		}
		modelName := "text-embedding-3-small"
		pm := ProviderMeta{Provider: "openai", Model: modelName, Dimensions: ExpectedVectorDimensions}
		return openAIEmbeddingProvider{
			apiKeyEnv:  envName,
			httpClient: http.DefaultClient,
			model:      modelName,
			baseURL:    "",
			dimensions: ExpectedVectorDimensions,
		}, pm, nil
	default:
		return nil, ProviderMeta{}, fmt.Errorf("graph: unknown embeddings.provider %q (operational: openai; planned stub: local)", cfg.Embeddings.Provider)
	}
}

// localEmbeddingProvider is a placeholder until a real local runtime exists (see package
// header). OpenAI remains the only operational provider today.
type localEmbeddingProvider struct {
	model string
}

func (p localEmbeddingProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(texts) == 0 {
		return nil, nil
	}
	_ = p.model
	return nil, fmt.Errorf("%w (model name %q reserved for future local runtime; operational provider is openai with API key in openai_api_key_env)", ErrLocalEmbeddingsUnavailable, p.model)
}

// openAIEmbeddingProvider calls the OpenAI embeddings API (text-embedding-3*).
// It always requests dimensions=ExpectedVectorDimensions (384) so vectors match the
// Neo4j vector index in schema.go without changing index width.
type openAIEmbeddingProvider struct {
	apiKeyEnv  string
	httpClient *http.Client
	model      string
	// baseURL overrides the OpenAI embeddings endpoint (tests); empty uses production URL.
	baseURL string
	// dimensions is sent as the JSON "dimensions" field (OpenAI text-embedding-3+ only).
	dimensions int
}

func (p openAIEmbeddingProvider) embeddingsEndpoint() string {
	if strings.TrimSpace(p.baseURL) != "" {
		return p.baseURL
	}
	return "https://api.openai.com/v1/embeddings"
}

func (p openAIEmbeddingProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(texts) == 0 {
		return nil, nil
	}
	key := strings.TrimSpace(os.Getenv(p.apiKeyEnv))
	if key == "" {
		return nil, fmt.Errorf("graph: env %s is unset or empty (OpenAI embeddings)", p.apiKeyEnv)
	}

	reqBody := map[string]any{
		"model":           p.model,
		"input":           texts,
		"encoding_format": "float",
	}
	if p.dimensions > 0 {
		reqBody["dimensions"] = p.dimensions
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.embeddingsEndpoint(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("graph: openai embeddings request: %w", err)
	}
	defer resp.Body.Close()
	rb, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail := truncateForOperatorMessage(string(rb), openAIHTTPErrorBodyMaxRunes)
		return nil, fmt.Errorf("graph: openai embeddings HTTP %d: %s", resp.StatusCode, detail)
	}

	var parsed struct {
		Data []struct {
			Object    string    `json:"object"`
			Index     int       `json:"index"`
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rb, &parsed); err != nil {
		return nil, fmt.Errorf("graph: openai embeddings decode: %w", err)
	}
	if len(parsed.Data) != len(texts) {
		return nil, fmt.Errorf("graph: openai returned %d embeddings for %d inputs", len(parsed.Data), len(texts))
	}

	out := make([][]float32, len(texts))
	byIndex := make([][]float64, len(texts))
	for _, d := range parsed.Data {
		if d.Index < 0 || d.Index >= len(texts) {
			return nil, fmt.Errorf("graph: openai embedding index out of range: %d", d.Index)
		}
		byIndex[d.Index] = d.Embedding
	}
	for i := range texts {
		vec := byIndex[i]
		if len(vec) != ExpectedVectorDimensions {
			return nil, fmt.Errorf("%w: got %d for input %d (model %q; index expects %d for MiniLM parity)",
				ErrOpenAIDimensionMismatch, len(vec), i, p.model, ExpectedVectorDimensions)
		}
		f32 := make([]float32, len(vec))
		for j, v := range vec {
			f32[j] = float32(v)
		}
		out[i] = f32
	}
	return out, nil
}

// AttachFindingsEmbeddings fills f.Embedding in-place for each finding using the provider.
// It does not write JSON or touch disk. Errors from the provider propagate (embedding-enabled
// paths only). Wrong vector length always errors.
func AttachFindingsEmbeddings(ctx context.Context, p EmbeddingProvider, findings []models.Finding) error {
	if p == nil {
		return fmt.Errorf("graph: embedding provider is nil")
	}
	if len(findings) == 0 {
		return nil
	}
	texts := make([]string, len(findings))
	for i := range findings {
		texts[i] = FindingEmbeddingText(findings[i])
	}
	vecs, err := p.Embed(ctx, texts)
	if err != nil {
		return err
	}
	if len(vecs) != len(findings) {
		return fmt.Errorf("graph: embedding count mismatch: %d vectors for %d findings", len(vecs), len(findings))
	}
	for i := range findings {
		if len(vecs[i]) != ExpectedVectorDimensions {
			return fmt.Errorf("graph: finding %q: expected %d embedding dims, got %d",
				findings[i].ID, ExpectedVectorDimensions, len(vecs[i]))
		}
		findings[i].Embedding = vecs[i]
	}
	return nil
}
