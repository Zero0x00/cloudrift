import { useMemo } from "react";
import { useSearchParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { apiClient } from "../api/client";
import { queryKeys } from "../api/queryKeys";

export function useScanContext() {
  const [params] = useSearchParams();
  const queryScanId = params.get("scan_id");

  const scansQuery = useQuery({
    queryKey: queryKeys.scans(),
    queryFn: () => apiClient.getScans(),
    staleTime: 60_000,
    gcTime: 300_000
  });

  const selectedScanId = useMemo(() => {
    if (queryScanId) {
      return queryScanId;
    }
    return scansQuery.data?.items[0]?.scan_id ?? null;
  }, [queryScanId, scansQuery.data?.items]);

  return {
    selectedScanId,
    scans: scansQuery.data?.items ?? [],
    scansQuery,
    isResolvingScan: scansQuery.isLoading && !selectedScanId
  };
}
