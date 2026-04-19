package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsWhenFileMissing(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.AWS.OrgRoleName != "CloudriftAuditRole" {
		t.Fatalf("unexpected default org role: %s", cfg.AWS.OrgRoleName)
	}
	if cfg.Embeddings.Provider != "openai" || cfg.Embeddings.LocalModel != "all-MiniLM-L6-v2" {
		t.Fatalf("unexpected default embeddings: provider=%q model=%q", cfg.Embeddings.Provider, cfg.Embeddings.LocalModel)
	}
	if cfg.Embeddings.OpenaiAPIKeyEnv != "OPENAI_API_KEY" {
		t.Fatalf("unexpected default openai key env: %q", cfg.Embeddings.OpenaiAPIKeyEnv)
	}
}

func TestLoadFromExplicitPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cloudrift.toml")
	data := `
[aws]
org_role_name = "CustomRole"

[scan]
http_probe_concurrency = 25
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.AWS.OrgRoleName != "CustomRole" {
		t.Fatalf("expected CustomRole, got %s", cfg.AWS.OrgRoleName)
	}
	if cfg.Scan.HTTPProbeConcurrency != 25 {
		t.Fatalf("expected 25, got %d", cfg.Scan.HTTPProbeConcurrency)
	}
}
