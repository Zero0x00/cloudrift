package schema

// BlastRootKind identifies what the analysis is centered on.
type BlastRootKind string

const (
	BlastRootFinding        BlastRootKind = "finding"
	BlastRootExternalEntity BlastRootKind = "external_entity"
	BlastRootPrincipal      BlastRootKind = "principal"
)

// BlastRadiusSummary is a lightweight, card-friendly view for findings / entities / high-signal.
type BlastRadiusSummary struct {
	RootType               BlastRootKind `json:"root_type"`
	RootID                 string        `json:"root_id"`
	ScanID                 string        `json:"scan_id"`
	Mode                   string        `json:"mode"`
	ReachableResourceCount int           `json:"reachable_resource_count"`
	ReachableAccountsCount int           `json:"reachable_accounts_count"`
	TopResourceTypes       []string      `json:"top_resource_types"`
	TopImpactedAccounts    []string      `json:"top_impacted_accounts"`
	TopImpactedResources   []string      `json:"top_impacted_resources"`
	EscalationPossible     bool          `json:"escalation_possible"`
	SummaryText            string        `json:"summary_text"`
	RecommendedActionLabel string        `json:"recommended_action_label"`
	GraphAvailable         bool          `json:"graph_available"`
	// GraphUnavailableReason is empty when graphAvailable is true; otherwise a stable machine string.
	GraphUnavailableReason string `json:"graph_unavailable_reason,omitempty"`
	// AffectedARN is set for finding-root summaries when a focal resource exists in scan JSON.
	FocalResourceARN string `json:"focal_resource_arn,omitempty"`
	// SourceFindingID for finding-root responses.
	SourceFindingID string `json:"source_finding_id,omitempty"`
	// SourcePrincipalARN for principal-root responses.
	SourcePrincipalARN string `json:"source_principal_arn,omitempty"`
	// SourcePrincipalID is the encoded principal id for principal-root responses.
	SourcePrincipalID string `json:"source_principal_id,omitempty"`
	// SourceEntityID is the encoded id for external-entity roots.
	SourceEntityID string `json:"source_entity_id,omitempty"`
}

// BlastExplorerResponse is the visual explorer payload; still curated, not a raw Neo4j dump.
type BlastExplorerResponse struct {
	Focus   BlastFocus         `json:"focus"`
	Summary BlastRadiusSummary `json:"summary"`
	Nodes   []BlastGraphNode   `json:"nodes"`
	Edges   []BlastGraphEdge   `json:"edges"`
	Display BlastDisplayHints  `json:"display"`
}

// BlastFocus ties the visualization to one operational story.
type BlastFocus struct {
	RootID      string        `json:"root_id"`
	RootType    BlastRootKind `json:"root_type"`
	FindingID   string        `json:"finding_id,omitempty"`
	EntityID    string        `json:"entity_id,omitempty"`
	PrincipalID string        `json:"principal_id,omitempty"`
	Mode        string        `json:"mode,omitempty"`
	BlastMode   string        `json:"blast_mode,omitempty"`
}

// BlastGraphNode is a UI-scoped view of a graph row.
type BlastGraphNode struct {
	ID              string  `json:"id"`
	Label           string  `json:"label"`
	Type            string  `json:"type"`
	Subtype         string  `json:"subtype,omitempty"`
	AccountID       string  `json:"account_id,omitempty"`
	SeverityOrTier  string  `json:"severity_or_tier,omitempty"`
	IsFocus         bool    `json:"is_focus"`
	IsCriticalPath  bool    `json:"is_critical_path"`
	IsReachable     bool    `json:"is_reachable"`
	IsExternal      bool    `json:"is_external"`
	ImpactScore     float64 `json:"impact_score,omitempty"`
	DisplayNameHint string  `json:"display_name_hint,omitempty"`
}

// BlastGraphEdge is a single directed relationship in the UI graph.
type BlastGraphEdge struct {
	ID             string `json:"id"`
	Source         string `json:"source"`
	Target         string `json:"target"`
	Type           string `json:"type"`
	Label          string `json:"label"`
	IsCriticalPath bool   `json:"is_critical_path"`
	Explanation    string `json:"explanation,omitempty"`
}

// BlastDisplayHints nudges the camera and selection without exposing raw Cypher.
type BlastDisplayHints struct {
	DefaultFocusID   string   `json:"default_focus_id,omitempty"`
	HighlightNodeIds []string `json:"highlight_node_ids,omitempty"`
	HighlightEdgeIds []string `json:"highlight_edge_ids,omitempty"`
}
