package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	AWS struct {
		OrgRoleName       string   `toml:"org_role_name"`
		ManagementProfile string   `toml:"management_profile"`
		Regions           []string `toml:"regions"`
	} `toml:"aws"`
	Scan struct {
		HTTPProbeConcurrency      int    `toml:"http_probe_concurrency"`
		RoleAssumptionConcurrency int    `toml:"role_assumption_concurrency"`
		HTTPTimeoutSeconds        int    `toml:"http_timeout_seconds"`
		UserAgent                 string `toml:"user_agent"`
	} `toml:"scan"`
	Cost struct {
		Currency                  string  `toml:"currency"`
		RiskMultiplierReclaimable float64 `toml:"risk_multiplier_reclaimable"`
		RiskMultiplierDangling    float64 `toml:"risk_multiplier_dangling"`
		UseCUR                    bool    `toml:"use_cur"`
	} `toml:"cost"`
	Trust struct {
		ApprovedExternalAccounts []string `toml:"approved_external_accounts"`
		StaleThresholdDays       int      `toml:"stale_threshold_days"`
		GhostThresholdDays       int      `toml:"ghost_threshold_days"`
	} `toml:"trust"`
	Output struct {
		DefaultFormat string `toml:"default_format"`
		OutputDir     string `toml:"output_dir"`
	} `toml:"output"`
	Neo4j struct {
		URI         string `toml:"uri"`
		Username    string `toml:"username"`
		PasswordEnv string `toml:"password_env"`
	} `toml:"neo4j"`
	// Embeddings (Phase 3) — IMPORTANT:
	//
	//   • The DEFAULT in Default() is provider "openai". That is the only operational
	//     embedding path today (OpenAI text-embedding-3-small with dimensions=384 for Neo4j).
	//
	//   • provider "local" is NOT supported yet: it is a planned hook for future on-box
	//     all-MiniLM-L6-v2 / ONNX. Selecting local returns a provider whose Embed always
	//     fails with a clear error until that work lands.
	//
	//   • Future vector retrieval must stay gated on provider readiness (see graph package
	//     comments on retrieval integration).
	Embeddings struct {
		Provider        string `toml:"provider"`           // default "openai" — only operational value today
		LocalModel      string `toml:"local_model"`        // planned local model name (no runtime yet)
		OpenaiAPIKeyEnv string `toml:"openai_api_key_env"` // env var holding OpenAI API key (default OPENAI_API_KEY)
	} `toml:"embeddings"`
}

func Default() *Config {
	c := &Config{}
	c.AWS.OrgRoleName = "CloudriftAuditRole"
	c.AWS.ManagementProfile = "default"
	c.Scan.HTTPProbeConcurrency = 50
	c.Scan.RoleAssumptionConcurrency = 10
	c.Scan.HTTPTimeoutSeconds = 10
	c.Scan.UserAgent = "cloudrift/0.1"
	c.Cost.Currency = "USD"
	c.Cost.RiskMultiplierReclaimable = 5.0
	c.Cost.RiskMultiplierDangling = 3.0
	c.Trust.StaleThresholdDays = 90
	c.Trust.GhostThresholdDays = 365
	c.Output.DefaultFormat = "table"
	c.Output.OutputDir = "./cloudrift-output"
	c.Neo4j.URI = "bolt://localhost:7687"
	c.Neo4j.Username = "neo4j"
	c.Neo4j.PasswordEnv = "CLOUDRIFT_NEO4J_PASSWORD"
	// Explicit default: OpenAI is the only operational embeddings provider in this release.
	c.Embeddings.Provider = "openai"
	c.Embeddings.LocalModel = "all-MiniLM-L6-v2"
	c.Embeddings.OpenaiAPIKeyEnv = "OPENAI_API_KEY"
	return c
}

func Load(path string) (*Config, error) {
	cfg := Default()
	if path == "" {
		path = resolveDefaultPath()
	}
	if _, err := os.Stat(path); err != nil {
		return cfg, nil
	}
	_, err := toml.DecodeFile(path, cfg)
	return cfg, err
}

func resolveDefaultPath() string {
	if env := os.Getenv("CLOUDRIFT_CONFIG"); env != "" {
		return env
	}
	if _, err := os.Stat("./cloudrift.toml"); err == nil {
		return "./cloudrift.toml"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "./cloudrift.toml"
	}
	return filepath.Join(home, ".config", "cloudrift", "config.toml")
}
