package graph

import (
	"fmt"
	"strings"

	"github.com/Zero0x00/cloudrift/internal/models"
)

// ProviderMeta identifies the embedding backend used to produce vectors (provider + model + width).
// It is returned alongside EmbeddingProvider from NewEmbeddingProvider and must stay aligned
// with vectors written to :Finding.embedding and with Neo4j ScanSnapshot embedding_* properties.
type ProviderMeta struct {
	Provider   string // canonical: openai, local
	Model      string // e.g. text-embedding-3-small, all-MiniLM-L6-v2
	Dimensions int    // vector width; must match ExpectedVectorDimensions for this graph schema
}

// IsEmpty reports whether meta should be treated as absent (no embedding identity to persist).
func (m ProviderMeta) IsEmpty() bool {
	return strings.TrimSpace(m.Provider) == "" || m.Dimensions <= 0
}

// GraphEmbeddingMeta is embedding identity read from a stored scan (Neo4j node or JSON metadata).
// Used with ValidateEmbeddingCompatibility before vector retrieval.
type GraphEmbeddingMeta struct {
	Provider   string
	Model      string
	Dimensions int
}

// GraphEmbeddingMetaFromScanSnapshot extracts embedding identity from scan metadata.
func GraphEmbeddingMetaFromScanSnapshot(s models.ScanSnapshot) GraphEmbeddingMeta {
	return GraphEmbeddingMeta{
		Provider:   strings.TrimSpace(s.EmbeddingProvider),
		Model:      strings.TrimSpace(s.EmbeddingModel),
		Dimensions: s.EmbeddingDimensions,
	}
}

// HasIdentity reports whether the graph/scan was exported with embedding metadata.
func (g GraphEmbeddingMeta) HasIdentity() bool {
	return strings.TrimSpace(g.Provider) != "" && g.Dimensions > 0
}

// ValidateEmbeddingCompatibility ensures the current embedding backend matches what the graph
// scan was indexed with. Use before vector retrieval so query vectors land in the same space.
//
// Backward compatibility: if graphMeta has no stored identity (legacy scans), returns nil.
// If graphMeta has identity but current is empty, returns an error (cannot safely retrieve).
// Mismatched provider, model, or dimensions return a clear, non-secret error.
func ValidateEmbeddingCompatibility(graphMeta GraphEmbeddingMeta, current ProviderMeta) error {
	if !graphMeta.HasIdentity() {
		return nil
	}
	if current.IsEmpty() {
		return fmt.Errorf("graph: scan has embedding identity (provider=%q model=%q dims=%d) but current provider meta is empty",
			graphMeta.Provider, graphMeta.Model, graphMeta.Dimensions)
	}
	if !strings.EqualFold(strings.TrimSpace(graphMeta.Provider), strings.TrimSpace(current.Provider)) {
		return fmt.Errorf("graph: embedding provider mismatch (graph=%q current=%q)",
			graphMeta.Provider, current.Provider)
	}
	if strings.TrimSpace(graphMeta.Model) != strings.TrimSpace(current.Model) {
		return fmt.Errorf("graph: embedding model mismatch (graph=%q current=%q)",
			graphMeta.Model, current.Model)
	}
	if graphMeta.Dimensions != current.Dimensions {
		return fmt.Errorf("graph: embedding dimensions mismatch (graph=%d current=%d)",
			graphMeta.Dimensions, current.Dimensions)
	}
	return nil
}

// SyncScanSnapshotEmbeddingMeta copies provider identity onto the scan snapshot for Neo4j/JSON.
// Call after a successful AttachFindingsEmbeddings (or when re-exporting with the same backend)
// so mergeScanSnapshotStatement can persist embedding_provider / embedding_model / embedding_dimensions.
func SyncScanSnapshotEmbeddingMeta(meta *models.ScanSnapshot, pm ProviderMeta) {
	if meta == nil || pm.IsEmpty() {
		return
	}
	meta.EmbeddingProvider = strings.TrimSpace(pm.Provider)
	meta.EmbeddingModel = strings.TrimSpace(pm.Model)
	meta.EmbeddingDimensions = pm.Dimensions
}
