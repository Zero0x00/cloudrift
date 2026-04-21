import type { ExternalEntitiesQueryParams, FindingsQueryParams } from "./types";

export const queryKeys = {
  scans: () => ["scans"] as const,
  summary: (scanId: string | null) => ["summary", scanId] as const,
  externalEntities: (scanId: string | null, params: ExternalEntitiesQueryParams) =>
    ["external-entities", scanId, params] as const,
  findings: (scanId: string | null, params: FindingsQueryParams) => ["findings", scanId, params] as const,
  remediationGroups: (scanId: string | null) => ["remediation-groups", scanId] as const,
  topFixes: (scanId: string | null, limit: number) => ["top-fixes", scanId, limit] as const,
  findingDetail: (scanId: string | null, findingId: string | null) =>
    ["finding", scanId, findingId] as const,
  accounts: (scanId: string | null) => ["accounts", scanId] as const,
  /** Fingerprint of finding IDs (order-preserving) for batched trust activity buckets. */
  trustActivityBuckets: (scanId: string | null, idsKey: string) =>
    ["trust-activity-buckets", scanId, idsKey] as const,
  diff: (oldScanId: string | null, newScanId: string | null) => ["diff", oldScanId, newScanId] as const,
  runtimeStatus: () => ["runtime-status"] as const,
  scanRunStatus: () => ["scan-run-status"] as const,
  scanRunHistory: () => ["scan-run-history"] as const
};
