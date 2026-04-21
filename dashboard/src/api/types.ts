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

export interface ExternalPrincipalTypeCount {
  principal_type: string;
  count: number;
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
  /** external_access findings with evidence.verdict === stale_review_now */
  external_trust_stale_count?: number;
  /** external_access where permission_visibility.classification is privileged (coarse permission tier; not admin_like). */
  external_privileged_count?: number;
  /** external_access where permission_visibility.capabilities.admin_like is true (capability flag; orthogonal to privileged tier). */
  external_admin_like_count?: number;
  /** external_access matching trust_stale AND privileged classification */
  external_stale_privileged_count?: number;
  /** Counts by evidence.principal_type (missing → unknown), sorted by count desc */
  external_principal_types?: ExternalPrincipalTypeCount[];
  /** Distinct external entities (principal × principal_type × external_account_id); matches unfiltered GET …/external-entities total_items */
  external_entity_count?: number;
  external_entities_with_stale_role?: number;
  external_entities_with_privileged_tier?: number;
  external_entities_with_admin_like_flag?: number;
  external_entity_by_principal_type?: ExternalEntityPrincipalTypeCount[];
  external_entities_preview?: ExternalEntityRow[];
}

export interface ExternalEntityPrincipalTypeCount {
  principal_type: string;
  entity_count: number;
}

export interface ExternalEntityRow {
  external_principal: string;
  principal_type: string;
  external_account_id: string;
  unique_trusted_role_count: number;
  unique_internal_account_count: number;
  highest_severity: string;
  total_monthly_risk_cost_usd: number;
  stale_role_count: number;
  privileged_role_count: number;
  admin_like_role_count: number;
  external_access_finding_count: number;
}

export interface ExternalEntitiesAppliedFilter {
  principal_type?: string;
  external_principal?: string;
  external_account_id?: string;
  has_stale_role?: boolean;
  has_privileged_role?: boolean;
  has_admin_like_role?: boolean;
}

export interface ExternalEntitiesResponse {
  scan_id: string;
  items: ExternalEntityRow[];
  filters: ExternalEntitiesAppliedFilter;
  pagination: PaginationMeta;
}

export interface FindingsAppliedFilter {
  severity?: string;
  module?: string;
  account_id?: string;
  claimability?: string;
  search?: string;
  trust_stale?: boolean;
  admin_like?: boolean;
  trust_classification?: string;
  principal_type?: string;
  external_principal?: string;
  external_account_id?: string;
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

/** GET /api/scans/:id/top-fixes — server-ranked priority queue */
export interface TopFixItem extends FindingListItem {
  priority_score: number;
  reason: string;
}

export interface TopFixesResponse {
  scan_id: string;
  items: TopFixItem[];
  limit: number;
}

export interface RemediationGroupItem {
  key: "reclaimable" | "stale_external_trust" | "dangling_edge" | "admin_like_external" | "broken_edge";
  label: string;
  why: string;
  finding_count: number;
  total_monthly_risk_cost_usd: number;
  top_example?: string;
}

export interface RemediationGroupsResponse {
  scan_id: string;
  items: RemediationGroupItem[];
}

export interface TrustDisplay {
  permission_visibility?: PermissionVisibilityDisplay;
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

export interface PermissionCapabilityFlags {
  can_assume_role?: boolean;
  iam_write_access?: boolean;
  s3_write_access?: boolean;
  cloudfront_control?: boolean;
  admin_like?: boolean;
}

export interface PermissionVisibilityDisplay {
  classification?: "admin" | "privileged" | "scoped" | "limited" | "unknown";
  capabilities?: PermissionCapabilityFlags;
  reasons?: string[];
  confidence?: "high" | "medium" | "low";
  analysis_mode?: string;
  policy_parse_ok?: boolean;
  used_managed_policy_name_heuristics?: boolean;
  complex_policy_detected?: boolean;
  managed_policy_documents_inspected?: boolean;
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
  trust_stale?: boolean;
  admin_like?: boolean;
  trust_classification?: string;
  principal_type?: string;
  external_principal?: string;
  external_account_id?: string;
}

/** Query params for GET /api/scans/:id/external-entities */
export interface ExternalEntitiesQueryParams {
  page?: number;
  page_size?: number;
  principal_type?: string;
  external_principal?: string;
  external_account_id?: string;
  has_stale_role?: boolean;
  has_privileged_role?: boolean;
  has_admin_like_role?: boolean;
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
