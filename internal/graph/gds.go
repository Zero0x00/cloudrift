package graph

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrGDSUnavailable indicates the Neo4j Graph Data Science (GDS) plugin is not installed,
// or its analytics procedures failed. It is intentionally non-fatal: graph projection
// (WriteScan) does NOT depend on GDS, so callers should log-and-continue when they see it
// rather than failing the export.
var ErrGDSUnavailable = errors.New("graph: Neo4j Graph Data Science (GDS) plugin unavailable; skipped clustering/centrality")

// gdsGraphName builds a safe in-memory GDS projection name for a scan. GDS graph names must
// be simple identifiers, so non-alphanumerics are collapsed to underscores.
func gdsGraphName(scanID string) string {
	s := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, scanID)
	if strings.Trim(s, "_") == "" {
		s = "scan"
	}
	return "cloudrift_" + s
}

// RunGDS computes graph clusters and centrality over the already-projected scan graph and
// writes them back as node properties:
//   - "community"  via Louvain community detection (groups related accounts/assets/findings)
//   - "centrality" via PageRank (a proxy for "how central / high blast-radius" a node is)
//
// It is BEST-EFFORT: it first drops any stale projection, projects all nodes + relationships
// UNDIRECTED (community detection needs undirected edges), runs the two analytics, and always
// drops the in-memory projection. If GDS is missing or any step fails, it returns an error
// that wraps ErrGDSUnavailable so the caller can degrade cleanly (the on-disk JSON and the
// Neo4j projection are unaffected). Requires the scan to have been projected first (WriteScan).
func RunGDS(ctx context.Context, ex Execer, scanID string) error {
	if ex == nil {
		return fmt.Errorf("%w (nil execer)", ErrGDSUnavailable)
	}
	g := gdsGraphName(scanID)
	params := map[string]any{"g": g}

	// Drop a stale projection of the same name if present (ignore errors — it may not exist).
	_ = ex.Run(ctx, dropProjectionCypher, params)

	if err := ex.Run(ctx, projectCypher, params); err != nil {
		return gdsErr(err)
	}
	// Always release the in-memory projection, even on a later failure.
	defer func() { _ = ex.Run(ctx, dropProjectionCypher, params) }()

	if err := ex.Run(ctx, louvainWriteCypher, params); err != nil {
		return gdsErr(err)
	}
	if err := ex.Run(ctx, pageRankWriteCypher, params); err != nil {
		return gdsErr(err)
	}
	return nil
}

const (
	dropProjectionCypher = `CALL gds.graph.drop($g, false) YIELD graphName RETURN graphName`
	// Project every node label and relationship type, oriented UNDIRECTED so Louvain treats
	// the trust/edge graph as connectivity rather than direction.
	projectCypher       = `CALL gds.graph.project($g, '*', {REL: {type: '*', orientation: 'UNDIRECTED'}}) YIELD graphName RETURN graphName`
	louvainWriteCypher  = `CALL gds.louvain.write($g, {writeProperty: 'community'}) YIELD communityCount RETURN communityCount`
	pageRankWriteCypher = `CALL gds.pageRank.write($g, {writeProperty: 'centrality'}) YIELD nodePropertiesWritten RETURN nodePropertiesWritten`
)

// gdsErr classifies a low-level failure. A missing-procedure error means the GDS plugin
// isn't installed; other failures are still surfaced as ErrGDSUnavailable because the caller
// treats clustering as optional/best-effort either way (errors.Is(err, ErrGDSUnavailable)).
func gdsErr(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "no procedure") || strings.Contains(msg, "there is no procedure") ||
		strings.Contains(msg, "unknown function") || strings.Contains(msg, "gds.") {
		return fmt.Errorf("%w (plugin missing: %v)", ErrGDSUnavailable, err)
	}
	return fmt.Errorf("%w (analytics error: %v)", ErrGDSUnavailable, err)
}
