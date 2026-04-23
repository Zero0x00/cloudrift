// Package blastradius implements Neo4j-backed blast-radius and attack-path analysis (v1).
// Traversal is limited to a fixed allowlist of relationship types; see [V1TraversalRels] and comments.
package blastradius

// V1TraversalRels are the only relationship *types* that may appear in a blast-radius or
// attack-path walk. They align with the Cloudrift graph model in internal/graph and
// internal/models (POINTS_TO, TRUSTS, etc.). These are the edges we consider semantically
// meaningful for “what is reachable from this resource / trust pivot” analysis.
//
// V1 does NOT walk CAPTURED (Scan↔Finding) or AFFECTS (Finding→Asset) as traversal steps:
// those bind findings to the graph but are not “operational” pivot edges. The service starts
// from a Finding or Asset and expands through infrastructure + trust.
//
// OWNED_BY is included so we can count distinct AwsAccount impact and show account boundaries
// in the explorer without inferring org-wide reach from a bare account id.
var V1TraversalRels = []string{
	"TRUSTS",    // cross-principal / cross-account trust (privilege-escalation-relevant)
	"POINTS_TO", // directed infrastructure edges (e.g. DNS to target)
	"FRONTS",    // edge / CDN / fronting
	"USES_CERT", // certificate linkage
	"OWNED_BY",  // Asset → AwsAccount
}

// V1MaxHops is the default maximum length for variable-length graph patterns in Neo4j queries.
// Keep it small: blast radius is an explanation surface, not exhaustive graph discovery.
const V1MaxHops = 4

// V1ExplorerNodeCap is the default upper bound on distinct nodes in an explorer payload.
const V1ExplorerNodeCap = 80

// V1ExplorerEdgeCap bounds distinct relationships returned to the UI.
const V1ExplorerEdgeCap = 120
