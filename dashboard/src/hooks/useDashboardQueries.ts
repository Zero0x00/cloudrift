import { useMemo } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "../api/client";
import { queryKeys } from "../api/queryKeys";
import type { FindingsQueryParams } from "../api/types";
import { getPreviousScanIdForDiff } from "../lib/scanOrder";
import { useScanContext } from "./useScanContext";

export type TrustActivityBuckets = {
  lt90: number;
  d90_365: number;
  gt365: number;
  never: number;
};

export const TRUST_ACTIVITY_SAMPLE_CAP = 25;

/** Shared with trust activity sampling so `fetchQuery` and `useFindingDetailQuery` hit one cache entry per finding. */
export const FINDING_DETAIL_STALE_TIME_MS = 60_000;

export function getFindingDetailQueryOptions(scanId: string, findingId: string) {
  return {
    queryKey: queryKeys.findingDetail(scanId, findingId),
    queryFn: () => apiClient.getFindingDetail(scanId, findingId),
    staleTime: FINDING_DETAIL_STALE_TIME_MS
  } as const;
}

function bucketForDays(days: number | undefined | null): keyof TrustActivityBuckets {
  if (days === undefined || days === null) {
    return "never";
  }
  if (days < 0) {
    return "never";
  }
  if (days < 90) {
    return "lt90";
  }
  if (days <= 365) {
    return "d90_365";
  }
  return "gt365";
}

/**
 * Buckets IAM last-used days from finding detail (`trust.days_since_used`) for up to
 * {@link TRUST_ACTIVITY_SAMPLE_CAP} findings. Does not load the full findings list — callers pass
 * current page IDs only. Failed detail fetches are counted in `failed` and omitted from buckets.
 *
 * Uses `queryClient.fetchQuery` with {@link getFindingDetailQueryOptions} so responses populate the same cache as
 * {@link useFindingDetailQuery}; expanding a row that was already sampled reuses fresh data without a second HTTP call.
 */
export function useTrustActivityBucketsQuery(scanId: string | null, findingIds: string[]) {
  const queryClient = useQueryClient();
  const capped = useMemo(() => findingIds.slice(0, TRUST_ACTIVITY_SAMPLE_CAP), [findingIds]);
  const idsKey = useMemo(() => capped.join("|"), [capped]);

  return useQuery({
    queryKey: queryKeys.trustActivityBuckets(scanId, idsKey),
    queryFn: async () => {
      const buckets: TrustActivityBuckets = { lt90: 0, d90_365: 0, gt365: 0, never: 0 };
      let failed = 0;
      const chunkSize = 5;
      for (let i = 0; i < capped.length; i += chunkSize) {
        const chunk = capped.slice(i, i + chunkSize);
        const settled = await Promise.allSettled(
          chunk.map((id) => queryClient.fetchQuery(getFindingDetailQueryOptions(scanId!, id)))
        );
        for (const r of settled) {
          if (r.status === "rejected") {
            failed += 1;
            continue;
          }
          const d = r.value.item.trust?.days_since_used;
          buckets[bucketForDays(d)] += 1;
        }
      }
      return { buckets, sampled: capped.length, failed };
    },
    enabled: Boolean(scanId && capped.length > 0),
    staleTime: 60_000
  });
}

export function useSummaryQuery() {
  const { selectedScanId } = useScanContext();
  return useQuery({
    queryKey: queryKeys.summary(selectedScanId),
    queryFn: () => apiClient.getSummary(selectedScanId as string),
    enabled: Boolean(selectedScanId)
  });
}

/** Paginated / filtered findings; params must be memoized in the caller for stable cache keys. */
export function useFindingsListQuery(
  scanId: string | null,
  params: FindingsQueryParams,
  options?: { enabled?: boolean }
) {
  const enabled = options?.enabled ?? Boolean(scanId);
  return useQuery({
    queryKey: queryKeys.findings(scanId, params),
    queryFn: () => apiClient.getFindings(scanId as string, params),
    enabled: Boolean(scanId) && enabled
  });
}

export function useFindingDetailQuery(scanId: string | null, findingId: string | null, enabled: boolean) {
  const canRun = Boolean(scanId && findingId && enabled);
  const detailOpts =
    scanId && findingId ? getFindingDetailQueryOptions(scanId, findingId) : null;
  return useQuery({
    queryKey: queryKeys.findingDetail(scanId, findingId),
    queryFn: detailOpts?.queryFn ?? (() => Promise.reject(new Error("Finding detail query disabled"))),
    enabled: canRun,
    staleTime: FINDING_DETAIL_STALE_TIME_MS
  });
}

/**
 * Per-scan accounts breakdown for filter dropdowns (e.g. Findings).
 * Cache key includes selectedScanId only — no cross-scan leakage.
 * Relaxed refetch defaults avoid noisy traffic when toggling pages beside Findings.
 */
export function useAccountsQuery() {
  const { selectedScanId } = useScanContext();
  return useQuery({
    queryKey: queryKeys.accounts(selectedScanId),
    queryFn: () => apiClient.getAccounts(selectedScanId as string),
    enabled: Boolean(selectedScanId),
    staleTime: 60_000,
    gcTime: 300_000,
    refetchOnWindowFocus: false
  });
}

export function useDiffQuery() {
  const { scans, selectedScanId } = useScanContext();
  const oldScanId = useMemo(
    () => getPreviousScanIdForDiff(scans, selectedScanId),
    [scans, selectedScanId]
  );
  return useQuery({
    queryKey: queryKeys.diff(oldScanId, selectedScanId),
    queryFn: () => apiClient.getDiff(oldScanId as string, selectedScanId as string),
    enabled: Boolean(oldScanId && selectedScanId)
  });
}

