package graph

import (
	"context"
	"log/slog"

	"github.com/Zero0x00/cloudrift/internal/config"
	"github.com/Zero0x00/cloudrift/internal/models"
)

// AttachEmbeddingsBestEffort generates finding embeddings using the configured provider and
// stamps the embedding identity onto meta, in place, just before a Neo4j export. Without this
// step no :Finding node ever gets an embedding, so the vector index stays empty and all RAG /
// `cloudrift query` vector searches return nothing.
//
// Best-effort by design: if the provider is unavailable or embedding fails (e.g. missing API
// key, provider=local stub), it logs a warning and leaves findings/meta unembedded so the graph
// export still succeeds — just without vector search. Returns true if embeddings were attached.
func AttachEmbeddingsBestEffort(ctx context.Context, cfg *config.Config, meta *models.ScanSnapshot, findings []models.Finding) bool {
	if len(findings) == 0 {
		return false
	}
	provider, pm, err := NewEmbeddingProvider(cfg)
	if err != nil {
		slog.Warn("embeddings disabled; exporting graph without vector search", "error", err)
		return false
	}
	if err := AttachFindingsEmbeddings(ctx, provider, findings); err != nil {
		slog.Warn("embedding generation failed; exporting graph without vector search", "error", err)
		return false
	}
	if meta != nil {
		SyncScanSnapshotEmbeddingMeta(meta, pm)
	}
	return true
}
