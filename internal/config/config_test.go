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

func TestNeo4jPasswordResolutionOrder(t *testing.T) {
	// 1. env var wins when set
	cfg := Default()
	cfg.Neo4j.PasswordEnv = "CLOUDRIFT_TEST_NEO4J_PW"
	cfg.Neo4j.Password = "inline-pw"
	os.Setenv("CLOUDRIFT_TEST_NEO4J_PW", "env-pw")
	t.Cleanup(func() { os.Unsetenv("CLOUDRIFT_TEST_NEO4J_PW") })
	if got := cfg.Neo4jPassword(); got != "env-pw" {
		t.Fatalf("expected env password to win, got %q", got)
	}

	// 2. inline password when env is unset
	os.Unsetenv("CLOUDRIFT_TEST_NEO4J_PW")
	if got := cfg.Neo4jPassword(); got != "inline-pw" {
		t.Fatalf("expected inline password, got %q", got)
	}

	// 3. password file when env and inline are empty
	dir := t.TempDir()
	pwFile := filepath.Join(dir, "pw")
	if err := os.WriteFile(pwFile, []byte("file-pw\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.Neo4j.Password = ""
	cfg.Neo4j.PasswordFile = pwFile
	if got := cfg.Neo4jPassword(); got != "file-pw" {
		t.Fatalf("expected file password (trimmed), got %q", got)
	}

	// 4. empty when nothing is set
	cfg.Neo4j.PasswordFile = ""
	if got := cfg.Neo4jPassword(); got != "" {
		t.Fatalf("expected empty password, got %q", got)
	}
}
