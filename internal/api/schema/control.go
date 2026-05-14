package schema

import "time"

type RuntimeStatusResponse struct {
	AWSProfiles      []string `json:"aws_profiles"`
	DefaultProfile   string   `json:"default_profile"`
	OpenAIConfigured bool     `json:"openai_configured"`
	Neo4jConfigured  bool     `json:"neo4j_configured"`
	SlackConfigured  bool     `json:"slack_configured"`
	EmailConfigured  bool     `json:"email_configured"`
}

type ValidateProfileRequest struct {
	Profile string `json:"profile"`
}

type ValidateProfileResponse struct {
	OK               bool   `json:"ok"`
	Profile          string `json:"profile"`
	Message          string `json:"message"`
	SSOLoginRequired bool   `json:"sso_login_required,omitempty"`
	SSOCommand       string `json:"sso_command,omitempty"`
}

type SSOLoginRequest struct {
	Profile string `json:"profile"`
}

type SSOLoginResponse struct {
	Started bool   `json:"started"`
	Message string `json:"message"`
	Command string `json:"command"`
}

type ScanStartRequest struct {
	Profile   string `json:"profile"`
	Module    string `json:"module"`
	NoHTTP    bool   `json:"no_http"`
	Neo4j     bool   `json:"neo4j"`
	Provider  string `json:"provider,omitempty"`
}

type ScanStartResponse struct {
	RunID   string `json:"run_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type ScanRunStatusResponse struct {
	RunID         string    `json:"run_id"`
	Status        string    `json:"status"`
	Stage         string    `json:"stage"`
	Message       string    `json:"message"`
	ScanID        string    `json:"scan_id,omitempty"`
	Profile       string    `json:"profile,omitempty"`
	Module        string    `json:"module,omitempty"`
	NoHTTP        bool      `json:"no_http"`
	Neo4j         bool      `json:"neo4j"`
	Provider      string    `json:"provider,omitempty"`
	StartedAt     time.Time `json:"started_at,omitempty"`
	FinishedAt    time.Time `json:"finished_at,omitempty"`
	LastUpdatedAt time.Time `json:"last_updated_at,omitempty"`
}

type ScanRunHistoryItem struct {
	RunID      string    `json:"run_id"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	Status     string    `json:"status"`
	Profile    string    `json:"profile,omitempty"`
	Module     string    `json:"module,omitempty"`
	NoHTTP     bool      `json:"no_http"`
	Neo4j      bool      `json:"neo4j"`
	Message    string    `json:"message"`
}

type ScanRunHistoryResponse struct {
	Items []ScanRunHistoryItem `json:"items"`
}
