import type { PermissionCapabilityFlags, PermissionVisibilityDisplay } from "../api/types";

type Tier = NonNullable<PermissionVisibilityDisplay["classification"]>;
type Confidence = NonNullable<PermissionVisibilityDisplay["confidence"]>;

const TIER_LABELS: Record<Tier, string> = {
  admin: "Admin",
  privileged: "Privileged",
  scoped: "Scoped",
  limited: "Limited",
  unknown: "Unknown"
};

const TIER_BADGE_CLASS: Record<Tier, string> = {
  admin:
    "border-rose-300/90 bg-rose-100 text-rose-900 dark:border-rose-700/80 dark:bg-rose-900/30 dark:text-rose-200",
  privileged:
    "border-amber-300/90 bg-amber-100 text-amber-900 dark:border-amber-700/80 dark:bg-amber-900/30 dark:text-amber-200",
  scoped:
    "border-sky-300/90 bg-sky-100 text-sky-900 dark:border-sky-700/80 dark:bg-sky-900/30 dark:text-sky-200",
  limited:
    "border-slate-300/90 bg-slate-100 text-slate-700 dark:border-slate-700/80 dark:bg-slate-800 dark:text-slate-300",
  unknown:
    "border-fuchsia-300/90 bg-fuchsia-100 text-fuchsia-900 dark:border-fuchsia-700/80 dark:bg-fuchsia-900/30 dark:text-fuchsia-200"
};

const CONFIDENCE_LABELS: Record<Confidence, string> = {
  high: "Confidence: high",
  medium: "Confidence: medium",
  low: "Confidence: low"
};

const CAPABILITY_ORDER: Array<keyof PermissionCapabilityFlags> = [
  "can_assume_role",
  "iam_write_access",
  "s3_write_access",
  "cloudfront_control",
  "admin_like"
];

const CAPABILITY_LABELS: Record<keyof PermissionCapabilityFlags, string> = {
  can_assume_role: "AssumeRole",
  iam_write_access: "IAM write",
  s3_write_access: "S3 write",
  cloudfront_control: "CloudFront control",
  admin_like: "Admin-like"
};

export function formatPermissionTierLabel(tier: PermissionVisibilityDisplay["classification"]): string {
  if (!tier || !(tier in TIER_LABELS)) {
    return "Unknown";
  }
  return TIER_LABELS[tier as Tier];
}

export function permissionTierBadgeClass(tier: PermissionVisibilityDisplay["classification"]): string {
  if (!tier || !(tier in TIER_BADGE_CLASS)) {
    return TIER_BADGE_CLASS.unknown;
  }
  return TIER_BADGE_CLASS[tier as Tier];
}

export function formatPermissionConfidenceLabel(
  confidence: PermissionVisibilityDisplay["confidence"]
): string | null {
  if (!confidence || !(confidence in CONFIDENCE_LABELS)) {
    return null;
  }
  return CONFIDENCE_LABELS[confidence as Confidence];
}

export function enabledCapabilityLabels(capabilities: PermissionCapabilityFlags | undefined): string[] {
  if (!capabilities) {
    return [];
  }
  return CAPABILITY_ORDER.filter((key) => capabilities[key]).map((key) => CAPABILITY_LABELS[key]);
}

export function permissionAnalysisBadges(permission: PermissionVisibilityDisplay): string[] {
  const badges: string[] = [];
  if (permission.analysis_mode) {
    badges.push(`Mode: ${permission.analysis_mode}`);
  }
  if (permission.policy_parse_ok === false) {
    badges.push("Policy parse issues");
  }
  if (permission.complex_policy_detected) {
    badges.push("Complex policy detected");
  }
  if (permission.used_managed_policy_name_heuristics) {
    badges.push("Managed-policy-name heuristics");
  }
  if (permission.managed_policy_documents_inspected === false) {
    badges.push("Managed policy docs not inspected");
  }
  return badges;
}
