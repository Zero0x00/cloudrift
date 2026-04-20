import { useCallback, useMemo } from "react";
import { useSearchParams } from "react-router-dom";

const PAGE_SIZE_OPTIONS = [25, 50, 100] as const;
export type PageSizeOption = (typeof PAGE_SIZE_OPTIONS)[number];

export const DEFAULT_PAGE_SIZE: PageSizeOption = 50;

export type FindingsUrlState = {
  page: number;
  pageSize: PageSizeOption;
  severity: string;
  module: string;
  accountId: string;
  claimability: string;
  search: string;
  trustStale: boolean;
  adminLike: boolean;
  trustClassification: string;
  principalType: string;
  externalPrincipal: string;
  externalAccountId: string;
  /** Triage-only: group rows under account headings */
  groupByAccount: boolean;
};

function clampPageSize(n: number): PageSizeOption {
  if (PAGE_SIZE_OPTIONS.includes(n as PageSizeOption)) {
    return n as PageSizeOption;
  }
  return DEFAULT_PAGE_SIZE;
}

function parsePositiveInt(raw: string | null, fallback: number): number {
  if (!raw) {
    return fallback;
  }
  const n = parseInt(raw, 10);
  return Number.isFinite(n) && n > 0 ? n : fallback;
}

export function parseFindingsUrlState(params: URLSearchParams): FindingsUrlState {
  return {
    page: parsePositiveInt(params.get("page"), 1),
    pageSize: clampPageSize(parsePositiveInt(params.get("page_size"), DEFAULT_PAGE_SIZE)),
    severity: params.get("severity") ?? "",
    module: params.get("module") ?? "",
    accountId: params.get("account_id") ?? "",
    claimability: params.get("claimability") ?? "",
    search: params.get("search") ?? "",
    trustStale: params.get("trust_stale") === "true",
    adminLike: params.get("admin_like") === "true",
    trustClassification: params.get("trust_classification") ?? "",
    principalType: params.get("principal_type") ?? "",
    externalPrincipal: params.get("external_principal") ?? "",
    externalAccountId: params.get("external_account_id") ?? "",
    groupByAccount: params.get("group_by") === "account"
  };
}

function setParam(sp: URLSearchParams, key: string, value: string, omitIf: string) {
  const v = value.trim();
  if (!v || v === omitIf) {
    sp.delete(key);
  } else {
    sp.set(key, v);
  }
}

export function buildFindingsSearchParams(state: FindingsUrlState): URLSearchParams {
  const sp = new URLSearchParams();
  if (state.page > 1) {
    sp.set("page", String(state.page));
  }
  if (state.pageSize !== DEFAULT_PAGE_SIZE) {
    sp.set("page_size", String(state.pageSize));
  }
  setParam(sp, "severity", state.severity, "");
  setParam(sp, "module", state.module, "");
  setParam(sp, "account_id", state.accountId, "");
  setParam(sp, "claimability", state.claimability, "");
  setParam(sp, "search", state.search, "");
  if (state.trustStale) {
    sp.set("trust_stale", "true");
  }
  if (state.adminLike) {
    sp.set("admin_like", "true");
  }
  setParam(sp, "trust_classification", state.trustClassification, "");
  setParam(sp, "principal_type", state.principalType, "");
  setParam(sp, "external_principal", state.externalPrincipal, "");
  setParam(sp, "external_account_id", state.externalAccountId, "");
  if (state.groupByAccount) {
    sp.set("group_by", "account");
  }
  return sp;
}

/**
 * Read/write findings filters to URL query params. Uses replace: true to avoid deep history stacks.
 */
export function useFindingsUrlState() {
  const [searchParams, setSearchParams] = useSearchParams();

  const state = useMemo(() => parseFindingsUrlState(searchParams), [searchParams]);

  const patch = useCallback(
    (partial: Partial<FindingsUrlState>) => {
      const next = { ...state, ...partial };
      if (
        partial.page === undefined &&
        (partial.severity !== undefined ||
          partial.module !== undefined ||
          partial.accountId !== undefined ||
          partial.claimability !== undefined ||
          partial.search !== undefined ||
          partial.pageSize !== undefined ||
          partial.trustStale !== undefined ||
          partial.adminLike !== undefined ||
          partial.trustClassification !== undefined ||
          partial.principalType !== undefined ||
          partial.externalPrincipal !== undefined ||
          partial.externalAccountId !== undefined)
      ) {
        next.page = 1;
      }
      const merged = new URLSearchParams(searchParams);
      const nextParams = buildFindingsSearchParams(next);
      const managedKeys = [
        "page",
        "page_size",
        "severity",
        "module",
        "account_id",
        "claimability",
        "search",
        "trust_stale",
        "admin_like",
        "trust_classification",
        "principal_type",
        "external_principal",
        "external_account_id",
        "group_by"
      ];
      for (const k of managedKeys) {
        merged.delete(k);
      }
      for (const [k, v] of nextParams.entries()) {
        merged.set(k, v);
      }
      if (merged.toString() !== searchParams.toString()) {
        setSearchParams(merged, { replace: true });
      }
    },
    [state, searchParams, setSearchParams]
  );

  return { state, patch, searchParams };
}
