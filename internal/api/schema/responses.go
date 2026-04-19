package schema

import "time"

// ScanListResponse is the envelope for GET /api/scans.
// Data is derived from output_dir scan artifacts, not from a database.
type ScanListResponse struct {
	Items      []ScanListItem `json:"items"`
	TotalItems int            `json:"total_items"`
}

type ScanListItem struct {
	ScanID              string    `json:"scan_id"`
	Timestamp           time.Time `json:"timestamp"`
	AccountIDs          []string  `json:"account_ids"`
	FindingCount        int       `json:"finding_count"`
	CriticalCount       int       `json:"critical_count"`
	HighCount           int       `json:"high_count"`
	TotalMonthlyCostUSD float64   `json:"total_monthly_cost_usd"`
}

// ScanSummaryResponse is the payload for GET /api/scans/:id/summary.
type ScanSummaryResponse struct {
	ScanID                    string  `json:"scan_id"`
	FindingCount              int     `json:"finding_count"`
	CriticalCount             int     `json:"critical_count"`
	HighCount                 int     `json:"high_count"`
	MediumCount               int     `json:"medium_count"`
	// LowCount: residual severity bucket (not critical/high/medium), not a separate INFO counter.
	LowCount                  int     `json:"low_count"`
	TotalMonthlyDirectCostUSD float64 `json:"total_monthly_direct_cost_usd"`
	TotalMonthlyRiskCostUSD   float64 `json:"total_monthly_risk_cost_usd"`
	ReclaimableCount          int     `json:"reclaimable_count"`
	DanglingCount             int     `json:"dangling_count"`
	BrokenCount               int     `json:"broken_count"`
	EdgeObscuredCount         int     `json:"edge_obscured_count"`
	ExternalAccessCount       int     `json:"external_access_count"`
	OrphanedEdgeCount         int     `json:"orphaned_edge_count"`
}

// FindingsListResponse is the payload for GET /api/scans/:id/findings.
type FindingsListResponse struct {
	Items      []FindingListItem     `json:"items"`
	Pagination PaginationMeta        `json:"pagination"`
	Filters    FindingsAppliedFilter `json:"filters"`
}

type PaginationMeta struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

type FindingsAppliedFilter struct {
	Severity    string `json:"severity,omitempty"`
	Module      string `json:"module,omitempty"`
	AccountID   string `json:"account_id,omitempty"`
	Claimability string `json:"claimability,omitempty"`
	Search      string `json:"search,omitempty"`
}

// FindingListItem is optimized for table/card/list rendering.
type FindingListItem struct {
	ID                  string  `json:"id"`
	Title               string  `json:"title"`
	Severity            string  `json:"severity"`
	Module              string  `json:"module"`
	Claimability        string  `json:"claimability"`
	AffectedARN         string  `json:"affected_arn"`
	AccountID           string  `json:"account_id"`
	AccountName         string  `json:"account_name,omitempty"`
	OUPath              string  `json:"ou_path,omitempty"`
	Team                string  `json:"team,omitempty"`
	Hostname            string  `json:"hostname,omitempty"`
	MonthlyDirectCostUSD float64 `json:"monthly_direct_cost_usd"`
	MonthlyRiskCostUSD  float64 `json:"monthly_risk_cost_usd"`
}

// FindingDetailResponse is the payload for GET /api/scans/:id/findings/:fid.
type FindingDetailResponse struct {
	Item FindingDetailItem `json:"item"`
}

type FindingDetailItem struct {
	FindingListItem
	Impact          string         `json:"impact,omitempty"`
	Recommendation  string         `json:"recommendation,omitempty"`
	RemediationCmd  string         `json:"remediation_command,omitempty"`
	ScanID          string         `json:"scan_id"`
	Evidence        map[string]any `json:"evidence,omitempty"`
	Trust           *TrustDisplay  `json:"trust,omitempty"`
}

// TrustDisplay is optional and populated when module == external_access.
type TrustDisplay struct {
	RoleARN          string `json:"role_arn,omitempty"`
	RoleName         string `json:"role_name,omitempty"`
	ExternalPrincipal string `json:"external_principal,omitempty"`
	PrincipalType    string `json:"principal_type,omitempty"`
	ExternalAccountID string `json:"external_account_id,omitempty"`
	DaysSinceUsed    *int   `json:"days_since_used,omitempty"`
	Verdict          string `json:"verdict,omitempty"`
	Reason           string `json:"reason,omitempty"`
	AdminEvalState   string `json:"admin_eval_state,omitempty"`
	UnknownVendor    *bool  `json:"unknown_vendor,omitempty"`
	ActivityStatus   string `json:"activity_status,omitempty"`
}

// AccountsBreakdownResponse is the payload for GET /api/scans/:id/accounts.
type AccountsBreakdownResponse struct {
	Items []AccountBreakdownItem `json:"items"`
}

type AccountBreakdownItem struct {
	AccountID                string  `json:"account_id"`
	AccountName              string  `json:"account_name,omitempty"`
	OUPath                   string  `json:"ou_path,omitempty"`
	Team                     string  `json:"team,omitempty"`
	FindingCount             int     `json:"finding_count"`
	CriticalCount            int     `json:"critical_count"`
	HighCount                int     `json:"high_count"`
	TotalMonthlyDirectCostUSD float64 `json:"total_monthly_direct_cost_usd"`
	TotalMonthlyRiskCostUSD  float64 `json:"total_monthly_risk_cost_usd"`
	TopFinding               string  `json:"top_finding,omitempty"`
}

// DiffResponse is the payload for GET /api/diff?old=:id&new=:id.
type DiffResponse struct {
	OldScanID      string            `json:"old_scan_id"`
	NewScanID      string            `json:"new_scan_id"`
	NewFindings    []FindingListItem `json:"new_findings"`
	ResolvedFindings []FindingListItem `json:"resolved_findings"`
	UnchangedCount int               `json:"unchanged_count"`
}

// ScanProgressEvent is the websocket payload for /api/scan/progress.
type ScanProgressEvent struct {
	EventType         string    `json:"event_type"`
	ScanID            string    `json:"scan_id,omitempty"`
	Stage             string    `json:"stage"`
	Message           string    `json:"message,omitempty"`
	CompletedAccounts int       `json:"completed_accounts"`
	TotalAccounts     int       `json:"total_accounts"`
	Timestamp         time.Time `json:"timestamp"`
}

// APIErrorResponse is the standard API error envelope.
type APIErrorResponse struct {
	Error   string         `json:"error"`
	Code    string         `json:"code"`
	Details map[string]any `json:"details,omitempty"`
}

