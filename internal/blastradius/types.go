package blastradius

// BlastMode is the viewer mode: attack_path emphasizes a shortest / highest-signal chain;
// blast_radius emphasizes breadth and impacted accounts.
type BlastMode string

const (
	ModeBlastRadius BlastMode = "blast_radius"
	ModeAttackPath  BlastMode = "attack_path"
)

// UnavailableReason is returned when Neo4j is not configured, connectivity fails, or
// the scan projection does not include the start node.
type UnavailableReason string

const (
	ReasonNone              UnavailableReason = ""
	ReasonNeo4jDisabled     UnavailableReason = "neo4j_unconfigured"
	ReasonNeo4jConnectError UnavailableReason = "neo4j_unavailable"
	ReasonNoGraphProjection UnavailableReason = "no_graph_projection"
	ReasonUnknownRoot       UnavailableReason = "unknown_root"
)
