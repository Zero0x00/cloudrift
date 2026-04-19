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
      if (partial.page === undefined && (partial.severity !== undefined || partial.module !== undefined || partial.accountId !== undefined || partial.claimability !== undefined || partial.search !== undefined || partial.pageSize !== undefined)) {
        next.page = 1;
      }
      setSearchParams(buildFindingsSearchParams(next), { replace: true });
    },
    [state, setSearchParams]
  );

  return { state, patch, searchParams };
}
