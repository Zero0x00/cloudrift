package queryv2

type Intent string

const (
	IntentSummary          Intent = "summary"
	IntentBlastRadius      Intent = "blast_radius"
	IntentRiskExplanation  Intent = "risk_explanation"
	IntentExternalReach    Intent = "external_reach"
	IntentTrustChains      Intent = "relationship_path"
	IntentPrioritizeFixes  Intent = "prioritization"
	IntentLargestImpact    Intent = "impact_ranking"
	IntentOwnership        Intent = "ownership"
	IntentRemediation      Intent = "remediation"
)

type QueryRequest struct {
	Query       string `json:"query"`
	ScanID      string `json:"scan_id,omitempty"`
	AccountID   string `json:"account_id,omitempty"`
	ModeHint    string `json:"mode_hint,omitempty"`
	TopK        int    `json:"top_k,omitempty"`
	FindingID   string `json:"finding_id,omitempty"`
	EntityID    string `json:"entity_id,omitempty"`
	PrincipalID string `json:"principal_id,omitempty"`
}

type SupportingFact struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Source string `json:"source"`
}

type RelatedObject struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
	URL   string `json:"url,omitempty"`
}

type QueryResponse struct {
	Answer            string          `json:"answer"`
	AnswerType        string          `json:"answer_type"`
	Intent            Intent          `json:"intent"`
	Confidence        string          `json:"confidence"`
	SupportLevel      string          `json:"support_level"`
	ScanID            string          `json:"scan_id,omitempty"`
	GraphUsed         bool            `json:"graph_used"`
	SemanticUsed      bool            `json:"semantic_used"`
	DomainUsed        bool            `json:"domain_used"`
	SupportingFacts   []SupportingFact`json:"supporting_facts"`
	RelatedObjects    []RelatedObject `json:"related_objects"`
	RecommendedAction []string        `json:"recommended_actions,omitempty"`
	FollowUps         []string        `json:"follow_up_suggestions,omitempty"`
	Notes             []string        `json:"notes,omitempty"`
}

type Plan struct {
	Intent          Intent
	NeedsGraph      bool
	NeedsSemantic   bool
	NeedsDomain     bool
	AnswerType      string
	ScanID          string
	AccountID       string
	FindingID       string
	EntityID        string
	PrincipalID     string
	ModeHint        string
}
