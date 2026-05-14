package graph

import (
	"strings"
	"testing"

	"github.com/Zero0x00/cloudrift/internal/models"
)

func TestGraphEmbeddingMetaFromScanSnapshot(t *testing.T) {
	s := models.ScanSnapshot{
		EmbeddingProvider:   "  openai  ",
		EmbeddingModel:      " text-embedding-3-small ",
		EmbeddingDimensions: 384,
	}
	g := GraphEmbeddingMetaFromScanSnapshot(s)
	if g.Provider != "openai" || g.Model != "text-embedding-3-small" || g.Dimensions != 384 {
		t.Fatalf("unexpected graph meta: %+v", g)
	}
	if !g.HasIdentity() {
		t.Fatal("expected HasIdentity")
	}
}

func TestGraphEmbeddingMeta_HasIdentityFalseWhenIncomplete(t *testing.T) {
	cases := []GraphEmbeddingMeta{
		{Provider: "", Model: "m", Dimensions: 384},
		{Provider: "openai", Model: "m", Dimensions: 0},
		{Provider: "   ", Model: "m", Dimensions: 384},
	}
	for i, g := range cases {
		if g.HasIdentity() {
			t.Fatalf("case %d: expected no identity, got %+v", i, g)
		}
	}
}

func TestProviderMeta_IsEmpty(t *testing.T) {
	if !(ProviderMeta{}).IsEmpty() {
		t.Fatal("zero value should be empty")
	}
	if (ProviderMeta{Provider: "openai", Dimensions: 384}).IsEmpty() {
		t.Fatal("openai + positive dims should not be empty")
	}
	if !(ProviderMeta{Provider: "openai", Dimensions: 0}).IsEmpty() {
		t.Fatal("missing dims should be empty")
	}
	if !(ProviderMeta{Dimensions: 384}).IsEmpty() {
		t.Fatal("missing provider should be empty")
	}
}

func TestValidateEmbeddingCompatibility_LegacyNoGraphIdentity(t *testing.T) {
	legacy := GraphEmbeddingMetaFromScanSnapshot(models.ScanSnapshot{})
	if err := ValidateEmbeddingCompatibility(legacy, ProviderMeta{}); err != nil {
		t.Fatalf("legacy scan should pass with nil current: %v", err)
	}
	pm := ProviderMeta{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 384}
	if err := ValidateEmbeddingCompatibility(legacy, pm); err != nil {
		t.Fatalf("legacy scan should pass with any current: %v", err)
	}
}

func TestValidateEmbeddingCompatibility_GraphIdentityCurrentEmpty(t *testing.T) {
	g := GraphEmbeddingMeta{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 384}
	err := ValidateEmbeddingCompatibility(g, ProviderMeta{})
	if err == nil || !strings.Contains(err.Error(), "current provider meta is empty") {
		t.Fatalf("expected empty-current error, got %v", err)
	}
}

func TestValidateEmbeddingCompatibility_ProviderMismatch(t *testing.T) {
	g := GraphEmbeddingMeta{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 384}
	cur := ProviderMeta{Provider: "local", Model: "all-MiniLM-L6-v2", Dimensions: 384}
	err := ValidateEmbeddingCompatibility(g, cur)
	if err == nil || !strings.Contains(err.Error(), "provider mismatch") {
		t.Fatalf("expected provider mismatch, got %v", err)
	}
}

func TestValidateEmbeddingCompatibility_ModelMismatch(t *testing.T) {
	g := GraphEmbeddingMeta{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 384}
	cur := ProviderMeta{Provider: "openai", Model: "text-embedding-3-large", Dimensions: 384}
	err := ValidateEmbeddingCompatibility(g, cur)
	if err == nil || !strings.Contains(err.Error(), "model mismatch") {
		t.Fatalf("expected model mismatch, got %v", err)
	}
}

func TestValidateEmbeddingCompatibility_DimensionsMismatch(t *testing.T) {
	g := GraphEmbeddingMeta{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 384}
	cur := ProviderMeta{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536}
	err := ValidateEmbeddingCompatibility(g, cur)
	if err == nil || !strings.Contains(err.Error(), "dimensions mismatch") {
		t.Fatalf("expected dimensions mismatch, got %v", err)
	}
}

func TestValidateEmbeddingCompatibility_ProviderCaseInsensitive(t *testing.T) {
	g := GraphEmbeddingMeta{Provider: "OpenAI", Model: "text-embedding-3-small", Dimensions: 384}
	cur := ProviderMeta{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 384}
	if err := ValidateEmbeddingCompatibility(g, cur); err != nil {
		t.Fatal(err)
	}
}

func TestValidateEmbeddingCompatibility_OK(t *testing.T) {
	g := GraphEmbeddingMeta{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 384}
	cur := ProviderMeta{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 384}
	if err := ValidateEmbeddingCompatibility(g, cur); err != nil {
		t.Fatal(err)
	}
}

func TestSyncScanSnapshotEmbeddingMeta_NilSafe(t *testing.T) {
	SyncScanSnapshotEmbeddingMeta(nil, ProviderMeta{Provider: "openai", Model: "m", Dimensions: 384})
}

func TestSyncScanSnapshotEmbeddingMeta_SkipsEmptyProviderMeta(t *testing.T) {
	meta := models.ScanSnapshot{ScanID: "s1"}
	SyncScanSnapshotEmbeddingMeta(&meta, ProviderMeta{})
	if meta.EmbeddingProvider != "" || meta.EmbeddingDimensions != 0 {
		t.Fatalf("expected no change, got %+v", meta)
	}
}

func TestSyncScanSnapshotEmbeddingMeta_Copies(t *testing.T) {
	meta := models.ScanSnapshot{ScanID: "s1"}
	pm := ProviderMeta{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 384}
	SyncScanSnapshotEmbeddingMeta(&meta, pm)
	if meta.EmbeddingProvider != "openai" || meta.EmbeddingModel != "text-embedding-3-small" || meta.EmbeddingDimensions != 384 {
		t.Fatalf("unexpected snapshot after sync: %+v", meta)
	}
}
