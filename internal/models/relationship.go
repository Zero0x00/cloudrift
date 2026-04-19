package models

type RelType string

const (
	RelPointsTo RelType = "POINTS_TO"
	RelOwnedBy  RelType = "OWNED_BY"
	RelUsesCert RelType = "USES_CERT"
	RelFronts   RelType = "FRONTS"
	RelTrusts   RelType = "TRUSTS"
)

type Relationship struct {
	SourceARN  string         `json:"source_arn"`
	TargetARN  string         `json:"target_arn"`
	RelType    RelType        `json:"rel_type"`
	Properties map[string]any `json:"properties"`
	ScanID     string         `json:"scan_id"`
}
