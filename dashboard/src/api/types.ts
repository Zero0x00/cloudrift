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
  /** Stable opaque id for blast-radius and cross-links (server-encoded aggregate key). */
  entity_id?: string;
  /** Optional encoded principal root id when a single trusted principal identity is derivable. */
  principal_id?: string;
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
  principal_id?: string;
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
  principal_id?: string;
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
  sso_login_required?: boolean;
  sso_command?: string;
}

export interface SSOLoginRequest {
  profile: string;
}

export interface SSOLoginResponse {
  started: boolean;
  message: string;
  command: string;
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

/** ——— Alerting (GET/POST/PUT /api/alerts/*) ——— */

export interface AlertChannel {
  type: string;
  display_name?: string;
  slack_webhook_url?: string;
}

export interface AlertScope {
  scan_ids?: string[];
  account_ids?: string[];
}

export interface AlertThreshold {
  count_min?: number;
  risk_cost_usd_min?: number;
}

export interface AlertRule {
  id: string;
  name: string;
  type: string;
  enabled: boolean;
  channel: AlertChannel;
  scope: AlertScope;
  threshold: AlertThreshold;
  /** Minutes between successful automatic Slack deliveries for this rule; 0 = off. */
  cooldown_minutes?: number;
  /** API-computed: Slack label / webhook display name */
  effective_destination_label?: string;
  /** e.g. explicit_slack — team routing reserved */
  routing_mode?: string;
  destination_valid?: boolean;
  last_evaluated_at?: string;
  last_triggered_at?: string;
  last_result?: string;
  last_delivery_at?: string;
  last_delivery_ok?: boolean;
  last_delivery_error?: string;
  created_at: string;
  updated_at: string;
}

export interface AlertPayload {
  title: string;
  severity: string;
  summary: string;
  bullets: string[];
  action_label: string;
  action_url: string;
}

export interface AlertContext {
  scan_id: string;
  rule_type: string;
  signal_count: number;
  metadata?: Record<string, unknown>;
  blast_summary?: {
    reachable_resources: number;
    reachable_accounts: number;
    escalation_possible: boolean;
    top_account?: string;
    dominant_motif?: string;
    action_label?: string;
  };
  payload: AlertPayload;
}

export interface AlertEvaluationRunMeta {
  scan_input?: string;
  used_latest_fallback?: boolean;
}

export interface AlertDestinationResolution {
  source: string;
  label: string;
  detail: string;
  valid: boolean;
  team_id?: string;
  resolved_account_id?: string;
}

export interface AlertTeamDestination {
  team_id: string;
  display_name?: string;
  slack_webhook_url: string;
}

export interface AlertAccountTeamBinding {
  account_id: string;
  team_id: string;
}

export interface AlertRoutingCatalog {
  default_team_id?: string;
  teams?: AlertTeamDestination[];
  account_teams?: AlertAccountTeamBinding[];
}

export interface AlertRoutingCatalogResponse {
  catalog: AlertRoutingCatalog;
}

export interface AlertEvaluationResult {
  rule_id: string;
  rule_name: string;
  rule_type: string;
  scan_id: string;
  triggered: boolean;
  summary: string;
  context: AlertContext;
  evaluated_at: string;
  run_meta?: AlertEvaluationRunMeta;
  destination?: AlertDestinationResolution;
}

export interface AlertDeliveryResult {
  provider: string;
  channel: string;
  attempted?: boolean;
  success: boolean;
  message_id?: string;
  error?: string;
  sent_at: string;
}

export interface AlertEvent {
  id: string;
  rule_id: string;
  rule_name: string;
  rule_type: string;
  scan_id: string;
  triggered: boolean;
  summary: string;
  payload_title?: string;
  context: AlertContext;
  delivery: AlertDeliveryResult;
  provider: string;
  channel_type: string;
  error?: string;
  forced_test_send?: boolean;
  delivery_attempted?: boolean;
  forced_test_delivery?: boolean;
  suppressed?: boolean;
  suppression_reason?: string;
  suppression_until?: string;
  cooldown_reference_event_id?: string;
  metadata?: Record<string, unknown>;
  created_at: string;
}

export interface AlertRulesResponse {
  items: AlertRule[];
}

export interface AlertRuleResponse {
  item: AlertRule;
}

export interface AlertEventsResponse {
  items: AlertEvent[];
}

export interface AlertSuppressionPreview {
  cooldown_minutes: number;
  would_suppress: boolean;
  reason?: string;
  active_until?: string;
  reference_event_id?: string;
  anchor_delivered_at?: string;
}

export interface AlertPreviewResponse {
  result: AlertEvaluationResult;
  scan_input?: string;
  used_latest_fallback?: boolean;
  suppression?: AlertSuppressionPreview;
}

export interface AlertTestResponse {
  event: AlertEvent;
  destination?: AlertDestinationResolution;
  scan_input?: string;
  used_latest_fallback?: boolean;
  /** True when test delivery bypassed an active per-rule cooldown. */
  cooldown_bypassed?: boolean;
}

export interface AlertCatalogType {
  type: string;
  label: string;
  description: string;
  supports_thresholds: boolean;
}

export interface AlertCatalogResponse {
  supported_types: AlertCatalogType[];
  supported_channels: string[];
}

// —— Blast radius (Neo4j-backed, optional)

export type BlastRootKind = "finding" | "external_entity" | "principal";

export interface BlastRadiusSummary {
  root_type: BlastRootKind;
  root_id: string;
  scan_id: string;
  mode: string;
  reachable_resource_count: number;
  reachable_accounts_count: number;
  top_resource_types: string[];
  top_impacted_accounts: string[];
  top_impacted_resources: string[];
  dominant_motif?: string;
  escalation_possible: boolean;
  summary_text: string;
  recommended_action_label: string;
  graph_available: boolean;
  graph_unavailable_reason?: string;
  focal_resource_arn?: string;
  source_finding_id?: string;
  source_principal_arn?: string;
  source_principal_id?: string;
  source_entity_id?: string;
}

export interface BlastFocus {
  root_id: string;
  root_type: BlastRootKind;
  finding_id?: string;
  entity_id?: string;
  principal_id?: string;
  mode?: string;
  blast_mode?: string;
}

export interface BlastGraphNode {
  id: string;
  label: string;
  type: string;
  subtype?: string;
  account_id?: string;
  severity_or_tier?: string;
  is_focus: boolean;
  is_critical_path: boolean;
  is_reachable: boolean;
  is_external: boolean;
  impact_score?: number;
  display_name_hint?: string;
}

export interface BlastGraphEdge {
  id: string;
  source: string;
  target: string;
  type: string;
  label: string;
  is_critical_path: boolean;
  explanation?: string;
}

export interface BlastDisplayHints {
  default_focus_id?: string;
  highlight_node_ids?: string[];
  highlight_edge_ids?: string[];
  highlight_path_ids?: string[];
}

export interface BlastPathVariant {
  id: string;
  label: string;
  kind: "primary" | "alternate";
  summary: string;
  node_ids: string[];
  edge_ids: string[];
  dominant_semantics?: string[];
  risk_hint?: string;
}

export interface BlastExplorerResponse {
  focus: BlastFocus;
  summary: BlastRadiusSummary;
  nodes: BlastGraphNode[];
  edges: BlastGraphEdge[];
  path_variants?: BlastPathVariant[];
  selected_path_id?: string;
  display: BlastDisplayHints;
}

export interface BlastExplorerExpansionResponse {
  expanded_from_node_id: string;
  expansion_applied: boolean;
  expansion_reason?: string;
  graph_unavailable: boolean;
  graph_unavailable_reason?: string;
  nodes?: BlastGraphNode[];
  edges?: BlastGraphEdge[];
  display?: BlastDisplayHints;
}

export interface InvestigationQueryRequest {
  query: string;
  scan_id?: string;
  account_id?: string;
  mode_hint?: string;
  top_k?: number;
  finding_id?: string;
  entity_id?: string;
  principal_id?: string;
}

export interface InvestigationSupportingFact {
  label: string;
  value: string;
  source: string;
}

export interface InvestigationRelatedObject {
  type: string;
  id: string;
  label?: string;
  url?: string;
}

export interface InvestigationQueryResponse {
  answer: string;
  answer_type: string;
  intent: string;
  confidence: string;
  support_level: string;
  scan_id?: string;
  graph_used: boolean;
  semantic_used: boolean;
  domain_used: boolean;
  supporting_facts: InvestigationSupportingFact[];
  related_objects: InvestigationRelatedObject[];
  recommended_actions?: string[];
  follow_up_suggestions?: string[];
  notes?: string[];
}
