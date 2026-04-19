export interface APIErrorResponse {
  error: string;
  code?: string;
  details?: Record<string, unknown>;
}

export interface ScanListItem {
  scan_id: string;
  timestamp: string;
  account_ids: string[];
  finding_count: number;
  critical_count: number;
  high_count: number;
  total_monthly_cost_usd: number;
}

export interface ScanListResponse {
  items: ScanListItem[];
  total_items: number;
}

export interface ScanSummaryResponse {
  scan_id: string;
  finding_count: number;
  critical_count: number;
  high_count: number;
  medium_count: number;
  /** Backend residual: all severities other than critical, high, medium (not a separate "info" field). */
  low_count: number;
  total_monthly_direct_cost_usd: number;
  total_monthly_risk_cost_usd: number;
  reclaimable_count: number;
  dangling_count: number;
  broken_count: number;
  edge_obscured_count: number;
  external_access_count: number;
  orphaned_edge_count: number;
}

export interface FindingsAppliedFilter {
  severity?: string;
  module?: string;
  account_id?: string;
  claimability?: string;
  search?: string;
}

export interface PaginationMeta {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

export interface FindingListItem {
  id: string;
  title: string;
  severity: string;
  module: string;
  claimability: string;
  affected_arn: string;
  account_id: string;
  account_name?: string;
  ou_path?: string;
  team?: string;
  hostname?: string;
  monthly_direct_cost_usd: number;
  monthly_risk_cost_usd: number;
}

export interface FindingsListResponse {
  items: FindingListItem[];
  pagination: PaginationMeta;
  filters: FindingsAppliedFilter;
}

export interface TrustDisplay {
  role_arn?: string;
  role_name?: string;
  external_principal?: string;
  principal_type?: string;
  external_account_id?: string;
  days_since_used?: number;
  verdict?: string;
  reason?: string;
  admin_eval_state?: string;
  unknown_vendor?: boolean;
  activity_status?: string;
}

export interface FindingDetailItem extends FindingListItem {
  impact?: string;
  recommendation?: string;
  remediation_command?: string;
  scan_id: string;
  evidence?: Record<string, unknown>;
  trust?: TrustDisplay;
}

export interface FindingDetailResponse {
  item: FindingDetailItem;
}

export interface AccountBreakdownItem {
  account_id: string;
  account_name?: string;
  ou_path?: string;
  team?: string;
  finding_count: number;
  critical_count: number;
  high_count: number;
  total_monthly_direct_cost_usd: number;
  total_monthly_risk_cost_usd: number;
  top_finding?: string;
}

export interface AccountsBreakdownResponse {
  items: AccountBreakdownItem[];
}

export interface DiffResponse {
  old_scan_id: string;
  new_scan_id: string;
  new_findings: FindingListItem[];
  resolved_findings: FindingListItem[];
  unchanged_count: number;
}

export interface FindingsQueryParams {
  page?: number;
  page_size?: number;
  severity?: string;
  module?: string;
  account_id?: string;
  claimability?: string;
  search?: string;
}

export interface RuntimeStatusResponse {
  aws_profiles: string[];
  default_profile: string;
  openai_configured: boolean;
  neo4j_configured: boolean;
  slack_configured: boolean;
  email_configured: boolean;
}

export interface ValidateProfileRequest {
  profile: string;
}

export interface ValidateProfileResponse {
  ok: boolean;
  profile: string;
  message: string;
}

export interface ScanStartRequest {
  profile: string;
  module: string;
  no_http: boolean;
  neo4j: boolean;
  provider?: string;
}

export interface ScanStartResponse {
  run_id: string;
  status: string;
  message: string;
}

export interface ScanRunStatusResponse {
  run_id: string;
  status: string;
  stage: string;
  message: string;
  scan_id?: string;
  profile?: string;
  module?: string;
  no_http: boolean;
  neo4j: boolean;
  provider?: string;
  started_at?: string;
  finished_at?: string;
  last_updated_at?: string;
}

export interface ScanRunHistoryItem {
  run_id: string;
  started_at?: string;
  finished_at?: string;
  status: string;
  profile?: string;
  module?: string;
  no_http: boolean;
  neo4j: boolean;
  message: string;
}

export interface ScanRunHistoryResponse {
  items: ScanRunHistoryItem[];
}
