import { useCallback, useMemo } from "react";
import { useSearchParams } from "react-router-dom";

const PAGE_SIZE_OPTIONS = [25, 50, 100] as const;
type PageSizeOption = (typeof PAGE_SIZE_OPTIONS)[number];
const DEFAULT_PAGE_SIZE: PageSizeOption = 50;

export type TrustReportUrlState = {
  page: number;
  pageSize: PageSizeOption;
  severity: string;
  search: string;
  trustStale: boolean;
  adminLike: boolean;
  trustClassification: string;
  principalType: string;
};

function parsePositiveInt(raw: string | null, fallback: number): number {
  if (!raw) {
    return fallback;
  }
  const n = parseInt(raw, 10);
  return Number.isFinite(n) && n > 0 ? n : fallback;
}

function clampPageSize(n: number): PageSizeOption {
  if (PAGE_SIZE_OPTIONS.includes(n as PageSizeOption)) {
    return n as PageSizeOption;
  }
  return DEFAULT_PAGE_SIZE;
}

function parseState(sp: URLSearchParams): TrustReportUrlState {
  return {
    page: parsePositiveInt(sp.get("page"), 1),
    pageSize: clampPageSize(parsePositiveInt(sp.get("page_size"), DEFAULT_PAGE_SIZE)),
    severity: sp.get("severity") ?? "",
    search: sp.get("search") ?? "",
    trustStale: sp.get("trust_stale") === "true",
    adminLike: sp.get("admin_like") === "true",
    trustClassification: sp.get("trust_classification") ?? "",
    principalType: sp.get("principal_type") ?? ""
  };
}

export function useTrustReportUrlState() {
  const [searchParams, setSearchParams] = useSearchParams();
  const state = useMemo(() => parseState(searchParams), [searchParams]);

  const patch = useCallback(
    (partial: Partial<TrustReportUrlState>) => {
      const next = { ...state, ...partial };
      if (
        partial.page === undefined &&
        (partial.pageSize !== undefined ||
          partial.severity !== undefined ||
          partial.search !== undefined ||
          partial.trustStale !== undefined ||
          partial.adminLike !== undefined ||
          partial.trustClassification !== undefined ||
          partial.principalType !== undefined)
      ) {
        next.page = 1;
      }

      const sp = new URLSearchParams(searchParams);
      const keys = [
        "page",
        "page_size",
        "severity",
        "search",
        "trust_stale",
        "admin_like",
        "trust_classification",
        "principal_type"
      ];
      for (const k of keys) {
        sp.delete(k);
      }
      if (next.page > 1) {
        sp.set("page", String(next.page));
      }
      if (next.pageSize !== DEFAULT_PAGE_SIZE) {
        sp.set("page_size", String(next.pageSize));
      }
      if (next.severity) {
        sp.set("severity", next.severity);
      }
      if (next.search.trim()) {
        sp.set("search", next.search.trim());
      }
      if (next.trustStale) {
        sp.set("trust_stale", "true");
      }
      if (next.adminLike) {
        sp.set("admin_like", "true");
      }
      if (next.trustClassification.trim()) {
        sp.set("trust_classification", next.trustClassification.trim());
      }
      if (next.principalType.trim()) {
        sp.set("principal_type", next.principalType.trim());
      }

      if (sp.toString() !== searchParams.toString()) {
        setSearchParams(sp, { replace: true });
      }
    },
    [state, searchParams, setSearchParams]
  );

  return { state, patch, searchParams };
}
