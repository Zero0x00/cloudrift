package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/spf13/cobra"

	"cloudrift/internal/config"
	"cloudrift/internal/graph"
	"cloudrift/internal/models"
)

// newQueryCommand registers Phase 3 natural-language retrieval over Neo4j (no LLM synthesis).
func newQueryCommand(cfgPath *string) *cobra.Command {
	var (
		outputDir       string
		scanIDInput     string
		queryFlag       string
		format          string
		topK            int
		requireIdentity bool
	)

	cmd := &cobra.Command{
		Use:   "query [flags] [QUERY_TEXT...]",
		Short: "Vector retrieval over exported graph (Phase 3; no answer synthesis)",
		Long: strings.TrimSpace(`
Runs embedding-backed hybrid retrieval against Neo4j for a single scan. Results are grounded
in graph rows only (no JSON findings substitution). Requires scan output on disk (metadata),
working Neo4j config, embeddings provider (OpenAI by default), and an exported graph with vectors.

Answer synthesis is not implemented; output is retrieval-only with explicit warnings for legacy
exports, empty hints, probe limits, and operator notes.`),
		RunE: func(cmd *cobra.Command, args []string) error {
			fromArgs := strings.TrimSpace(strings.Join(args, " "))
			qf := strings.TrimSpace(queryFlag)
			if fromArgs != "" && qf != "" {
				return fmt.Errorf("cloudrift query: provide either positional QUERY_TEXT or --query, not both")
			}
			queryText := fromArgs
			if queryText == "" {
				queryText = qf
			}
			if queryText == "" {
				return fmt.Errorf("cloudrift query: query text is required (positional args or --query)")
			}

			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return err
			}
			if outputDir == "" {
				outputDir = "./cloudrift-output"
			}

			out := cmd.OutOrStdout()
			errOut := cmd.ErrOrStderr()
			return runQueryCLI(cmd.Context(), cfg, queryCLIOptions{
				OutputDir:                      outputDir,
				ScanIDInput:                    scanIDInput,
				QueryText:                      queryText,
				Format:                         format,
				TopK:                           topK,
				RequireStoredEmbeddingIdentity: requireIdentity,
				Stdout:                         out,
				Stderr:                         errOut,
			})
		},
	}

	cmd.Flags().StringVar(&outputDir, "output-dir", "./cloudrift-output", "Directory containing scan subfolders")
	cmd.Flags().StringVar(&scanIDInput, "scan-id", "latest", "Scan ID or \"latest\" (newest by scan-metadata.json timestamp; tie-break directory name ascending, same as API)")
	cmd.Flags().StringVar(&queryFlag, "query", "", "Query text (optional if positional QUERY_TEXT is given)")
	cmd.Flags().StringVar(&format, "format", "table", "table (human retrieval summary) | json")
	cmd.Flags().IntVar(&topK, "top-k", 0, "Max findings after scan scoping (default 10, max 100; also scales vector probe)")
	cmd.Flags().BoolVar(&requireIdentity, "require-stored-embedding-identity", false, "Reject scans without embedding_provider/dimensions in scan-metadata.json")
	return cmd
}

type queryCLIOptions struct {
	OutputDir                      string
	ScanIDInput                    string
	QueryText                      string
	Format                         string
	TopK                           int
	RequireStoredEmbeddingIdentity bool
	Stdout                         io.Writer
	Stderr                         io.Writer
}

func runQueryCLI(ctx context.Context, cfg *config.Config, o queryCLIOptions) error {
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}
	f := strings.TrimSpace(strings.ToLower(o.Format))
	if f != "table" && f != "json" {
		return fmt.Errorf("cloudrift query: unsupported format %q (use table or json)", o.Format)
	}

	resolvedDir, err := resolveScanID(o.OutputDir, o.ScanIDInput)
	if err != nil {
		return fmt.Errorf("cloudrift query: resolve scan: %w", err)
	}
	if resolvedDir == "" {
		return fmt.Errorf("cloudrift query: no scan directories under %q (export a scan first)", o.OutputDir)
	}
	// resolvedDir is path-safe: resolveScanID delegates to scans.ResolveScanDirectoryName.
	scanPath := filepath.Join(o.OutputDir, resolvedDir)
	meta, err := loadScanMetadata(scanPath)
	if err != nil {
		return fmt.Errorf("cloudrift query: %w", err)
	}

	if err := validateNeo4jConfigForQuery(cfg); err != nil {
		return fmt.Errorf("cloudrift query: %w", err)
	}

	embed, pm, err := graph.NewEmbeddingProvider(cfg)
	if err != nil {
		return fmt.Errorf("cloudrift query: embeddings: %w", err)
	}

	driver, closeFn, err := openNeo4jDriverForQuery(ctx, cfg)
	if err != nil {
		return fmt.Errorf("cloudrift query: %w", err)
	}
	defer func() { _ = closeFn(ctx) }()

	resp, err := runQueryRetrieval(ctx, cfg, meta, o, driver, nil, embed, pm)
	if err != nil {
		return queryRetrievalError(err)
	}

	switch f {
	case "json":
		return writeQueryJSON(o.Stdout, o.QueryText, meta.ScanID, o.TopK, resp)
	default:
		return writeQueryHuman(o.Stdout, o.Stderr, o.QueryText, meta.ScanID, o.TopK, resp)
	}
}

// runQueryRetrieval is the test seam: supply a non-nil rowReader to avoid Neo4j (driver ignored).
func runQueryRetrieval(ctx context.Context, cfg *config.Config, meta models.ScanSnapshot, opts queryCLIOptions, driver neo4j.DriverWithContext, rowReader graph.RowReader, embed graph.EmbeddingProvider, pm graph.ProviderMeta) (*graph.RAGRetrievalResponse, error) {
	var rr graph.RowReader
	if rowReader != nil {
		rr = rowReader
	} else {
		if driver == nil {
			return nil, fmt.Errorf("cloudrift query: neo4j driver is nil")
		}
		rr = graph.NewDriverRowReader(driver, "")
	}
	return graph.RetrieveFindingContext(ctx, graph.RAGRetrievalInput{
		QueryText:                      opts.QueryText,
		ScanID:                         meta.ScanID,
		TopK:                           opts.TopK,
		GraphMeta:                      graph.GraphEmbeddingMetaFromScanSnapshot(meta),
		RequireStoredEmbeddingIdentity: opts.RequireStoredEmbeddingIdentity,
	}, rr, embed, pm)
}

func loadScanMetadata(scanPath string) (models.ScanSnapshot, error) {
	var meta models.ScanSnapshot
	b, err := os.ReadFile(filepath.Join(scanPath, "scan-metadata.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return meta, fmt.Errorf("scan output not found at %q (expected scan-metadata.json). Run scan and export to Neo4j before query", scanPath)
		}
		return meta, fmt.Errorf("read scan-metadata.json: %w", err)
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		return meta, fmt.Errorf("parse scan-metadata.json: %w", err)
	}
	if strings.TrimSpace(meta.ScanID) == "" {
		return meta, fmt.Errorf("scan-metadata.json has empty scan_id")
	}
	return meta, nil
}

func validateNeo4jConfigForQuery(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	uri := strings.TrimSpace(cfg.Neo4j.URI)
	user := strings.TrimSpace(cfg.Neo4j.Username)
	passEnv := strings.TrimSpace(cfg.Neo4j.PasswordEnv)
	if uri == "" {
		return errors.New("neo4j.uri is empty (configure Neo4j in config TOML for query)")
	}
	if user == "" {
		return errors.New("neo4j.username is empty")
	}
	if passEnv == "" {
		return errors.New("neo4j.password_env is empty")
	}
	if strings.TrimSpace(os.Getenv(passEnv)) == "" {
		return fmt.Errorf("env %s is unset or empty (Neo4j password)", passEnv)
	}
	return nil
}

func openNeo4jDriverForQuery(ctx context.Context, cfg *config.Config) (neo4j.DriverWithContext, func(context.Context) error, error) {
	uri := strings.TrimSpace(cfg.Neo4j.URI)
	user := strings.TrimSpace(cfg.Neo4j.Username)
	pass := strings.TrimSpace(os.Getenv(strings.TrimSpace(cfg.Neo4j.PasswordEnv)))
	d, err := newNeo4jDriver(ctx, uri, user, pass)
	if err != nil {
		return nil, nil, fmt.Errorf("neo4j connect: %w", err)
	}
	closeFn := func(ctx context.Context) error { return d.Close(ctx) }
	return d, closeFn, nil
}

type queryJSONResponse struct {
	QueryText                 string                  `json:"query"`
	ScanID                    string                  `json:"scan_id"`
	TopKRequested             int                     `json:"top_k_requested"`
	TopKEffective             int                     `json:"top_k_effective"`
	LegacyEmbeddingUnverified bool                    `json:"legacy_embedding_unverified"`
	EmbeddingIdentityVerified bool                    `json:"embedding_identity_verified"`
	VectorProbe               int                     `json:"vector_probe"`
	VectorGlobalMatchCount    int                     `json:"vector_global_match_count"`
	EmptyHint                 string                  `json:"empty_hint"`
	ProbeSaturated            bool                    `json:"probe_saturated"`
	OperatorNotes             []string                `json:"operator_notes,omitempty"`
	Hits                      []graph.RAGRetrievalHit `json:"hits"`
	AnswerSynthesis           string                  `json:"answer_synthesis"`
}

func writeQueryJSON(w io.Writer, queryText, scanID string, topKRequested int, resp *graph.RAGRetrievalResponse) error {
	if resp == nil {
		return fmt.Errorf("cloudrift query: nil response")
	}
	topKEff := graph.EffectiveTopK(topKRequested)
	out := queryJSONResponse{
		QueryText:                 queryText,
		ScanID:                    scanID,
		TopKRequested:             topKRequested,
		TopKEffective:             topKEff,
		LegacyEmbeddingUnverified: resp.LegacyEmbeddingUnverified,
		EmbeddingIdentityVerified: !resp.LegacyEmbeddingUnverified,
		VectorProbe:               resp.VectorProbe,
		VectorGlobalMatchCount:    resp.VectorGlobalMatchCount,
		EmptyHint:                 resp.EmptyHint.String(),
		ProbeSaturated:            resp.ProbeSaturated,
		OperatorNotes:             resp.OperatorNotes,
		Hits:                      resp.Hits,
		AnswerSynthesis:           "",
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func writeQueryHuman(stdout, stderr io.Writer, queryText, scanID string, topKRequested int, resp *graph.RAGRetrievalResponse) error {
	if resp == nil {
		return fmt.Errorf("cloudrift query: nil response")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Query: %s\n", queryText)
	fmt.Fprintf(&b, "Scan ID: %s\n", scanID)
	eff := graph.EffectiveTopK(topKRequested)
	fmt.Fprintf(&b, "Top-K (requested / effective): %d / %d\n", topKRequested, eff)
	if resp.LegacyEmbeddingUnverified {
		fmt.Fprintf(&b, "Embedding identity: LEGACY — stored embedding_provider/model/dimensions missing on scan; compatibility check was skipped. Treat scores cautiously.\n")
	} else {
		fmt.Fprintf(&b, "Embedding identity: verified against scan-metadata.json\n")
	}
	fmt.Fprintf(&b, "Vector probe (global neighbor budget): %d\n", resp.VectorProbe)
	fmt.Fprintf(&b, "Vector global match count (pre-scan filter): %d\n", resp.VectorGlobalMatchCount)
	if resp.ProbeSaturated {
		fmt.Fprintf(&b, "Probe saturation: true — global neighbor count reached the probe cap; retrieval may omit relevant corpus nodes beyond this budget.\n")
	}
	if resp.EmptyHint != graph.RAGEmptyHintNone {
		fmt.Fprintf(&b, "Empty result hint: %s\n", resp.EmptyHint.String())
	}
	if len(resp.OperatorNotes) > 0 {
		fmt.Fprintf(&b, "Operator notes:\n")
		for _, n := range resp.OperatorNotes {
			fmt.Fprintf(&b, "  - %s\n", n)
		}
	}
	fmt.Fprintf(&b, "Hits (%d):\n", len(resp.Hits))
	for i, h := range resp.Hits {
		fmt.Fprintf(&b, "  %d. [%s] score=%.4f %s\n", i+1, h.FindingID, h.Score, h.Title)
		fmt.Fprintf(&b, "     severity=%s claimability=%s account=%q ou=%q team=%q linked_arn=%q\n",
			h.Severity, h.Claimability, h.AccountName, h.AccountOUPath, h.AccountTeam, h.LinkedARN)
		fmt.Fprintf(&b, "     monthly_direct_cost_usd=%.2f recommendation=%s\n", h.MonthlyDirectCostUSD, h.Recommendation)
	}
	fmt.Fprintf(&b, "Answer synthesis: not implemented (retrieval-only).\n")
	if _, err := io.WriteString(stdout, b.String()); err != nil {
		return err
	}
	if resp.LegacyEmbeddingUnverified {
		_, _ = io.WriteString(stderr, "cloudrift query: warning: legacy embedding lineage (see stdout).\n")
	}
	return nil
}

// queryRetrievalError surfaces operator-facing text for known failures while preserving error chains.
func queryRetrievalError(err error) error {
	if err == nil {
		return nil
	}
	if graph.IsRAGVectorIndexMissing(err) {
		return fmt.Errorf("%s\n%w", graph.RAGVectorIndexOperatorMessage, err)
	}
	if errors.Is(err, graph.ErrRAGMissingEmbeddingProvider) {
		return fmt.Errorf("cloudrift query: configure embeddings (e.g. embeddings.provider and openai_api_key_env): %w", err)
	}
	if errors.Is(err, graph.ErrRAGInvalidProviderMeta) {
		return fmt.Errorf("cloudrift query: embedding provider metadata invalid: %w", err)
	}
	if errors.Is(err, graph.ErrRAGMissingGraphEmbeddingIdentity) {
		return fmt.Errorf("cloudrift query: scan lacks stored embedding identity; re-export with embedding metadata or omit --require-stored-embedding-identity: %w", err)
	}
	if errors.Is(err, graph.ErrLocalEmbeddingsUnavailable) {
		return fmt.Errorf("cloudrift query: %w", err)
	}
	if errors.Is(err, graph.ErrOpenAIDimensionMismatch) {
		return fmt.Errorf("cloudrift query: %w", err)
	}
	if errors.Is(err, graph.ErrRAGNeo4jQuery) {
		return fmt.Errorf("cloudrift query: neo4j query failed (see cause; raw Neo4j text is diagnostic only): %w", err)
	}
	return fmt.Errorf("cloudrift query: %w", err)
}
