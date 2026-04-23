package blastradius

import (
	"context"
	"os"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"cloudrift/internal/config"
)

// TryConnect returns a verified Neo4j driver, or nil when the graph is not available
// (unconfigured, missing credentials, driver creation failure, or connectivity check failure).
// Blast-radius treats nil as a normal degraded state, not a server error.
func TryConnect(ctx context.Context, cfg *config.Config) neo4j.DriverWithContext {
	if cfg == nil {
		return nil
	}
	uri := strings.TrimSpace(cfg.Neo4j.URI)
	if uri == "" {
		return nil
	}
	pwName := strings.TrimSpace(cfg.Neo4j.PasswordEnv)
	if pwName == "" {
		return nil
	}
	pw := strings.TrimSpace(os.Getenv(pwName))
	if pw == "" {
		return nil
	}
	username := strings.TrimSpace(cfg.Neo4j.Username)
	if username == "" {
		username = "neo4j"
	}
	dr, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(username, pw, ""))
	if err != nil {
		return nil
	}
	if err := dr.VerifyConnectivity(ctx); err != nil {
		_ = dr.Close(ctx)
		return nil
	}
	return dr
}
