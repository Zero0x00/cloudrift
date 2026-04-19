import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  useReactTable
} from "@tanstack/react-table";
import { useQueryClient } from "@tanstack/react-query";
import { Fragment, useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { formatQueryError } from "../api/httpError";
import type { FindingListItem, FindingsQueryParams } from "../api/types";
import {
  FindingDetailPanelContent,
  FindingDetailPanelError,
  FindingDetailPanelLoading
} from "../components/findings/FindingDetailPanel";
import { PageHeader } from "../components/PageHeader";
import { SeverityBadge } from "../components/SeverityBadge";
import { ScanRequired } from "../components/ScanRequired";
import { StatePanel } from "../components/StatePanel";
import {
  getFindingDetailQueryOptions,
  useAccountsQuery,
  useFindingDetailQuery,
  useFindingsListQuery
} from "../hooks/useDashboardQueries";
import { useDebouncedValue } from "../hooks/useDebouncedValue";
import { DEFAULT_PAGE_SIZE, useFindingsUrlState } from "../hooks/useFindingsUrlState";
import { useScanContext } from "../hooks/useScanContext";
import { findingInlineHint } from "../lib/findingHints";
import { displayTarget, formatUsd, shortenArn } from "../lib/format";

const PAGE_SIZE_OPTIONS = [25, 50, 100] as const;
const SEARCH_DEBOUNCE_MS = 350;
const TRIAGE_FETCH_PAGE_SIZE = 200;

const SEVERITY_OPTIONS = ["", "critical", "high", "medium", "low"] as const;
const MODULE_OPTIONS = ["", "orphaned_edge", "external_access"] as const;
const CLAIM_OPTIONS = ["", "reclaimable", "dangling", "broken", "edge_obscured"] as const;

const columnHelper = createColumnHelper<FindingListItem>();

export type FindingsPageProps = {
  /** Critical + high only, merged client-side, sorted by monthly risk (see banner for API limits). */
  triage?: boolean;
};

function bucketFindingsByAccount(items: FindingListItem[]): { accountId: string; items: FindingListItem[] }[] {
  const m = new Map<string, FindingListItem[]>();
  for (const it of items) {
    const acc = it.account_id?.trim() || "—";
    if (!m.has(acc)) {
      m.set(acc, []);
    }
    m.get(acc)!.push(it);
  }
  const groups = [...m.entries()].map(([accountId, list]) => ({ accountId, items: list }));
  groups.sort((a, b) => {
    const maxA = Math.max(0, ...a.items.map((i) => i.monthly_risk_cost_usd));
    const maxB = Math.max(0, ...b.items.map((i) => i.monthly_risk_cost_usd));
    if (maxB !== maxA) {
      return maxB - maxA;
    }
    return a.accountId.localeCompare(b.accountId);
  });
  return groups;
}

export function FindingsPage({ triage = false }: FindingsPageProps) {
  const { selectedScanId } = useScanContext();
  const queryClient = useQueryClient();
  const { state, patch } = useFindingsUrlState();
  const accountsQuery = useAccountsQuery();

  const [searchInput, setSearchInput] = useState(state.search);
  useEffect(() => {
    setSearchInput(state.search);
  }, [state.search]);

  const debouncedSearch = useDebouncedValue(searchInput.trim(), SEARCH_DEBOUNCE_MS);

  useEffect(() => {
    if (debouncedSearch === state.search) {
      return;
    }
    patch({ search: debouncedSearch, page: 1 });
  }, [debouncedSearch, state.search, patch]);

  const [expandedId, setExpandedId] = useState<string | null>(null);
  const prefetchTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const listParams: FindingsQueryParams = useMemo(() => {
    const p: FindingsQueryParams = { page: state.page, page_size: state.pageSize };
    if (state.severity) {
      p.severity = state.severity;
    }
    if (state.module) {
      p.module = state.module;
    }
    if (state.accountId) {
      p.account_id = state.accountId;
    }
    if (state.claimability) {
      p.claimability = state.claimability;
    }
    if (debouncedSearch) {
      p.search = debouncedSearch;
    }
    return p;
  }, [state.page, state.pageSize, state.severity, state.module, state.accountId, state.claimability, debouncedSearch]);

  const triageBackendParams: FindingsQueryParams = useMemo(() => {
    const p: FindingsQueryParams = { page: 1, page_size: TRIAGE_FETCH_PAGE_SIZE };
    if (state.module) {
      p.module = state.module;
    }
    if (state.accountId) {
      p.account_id = state.accountId;
    }
    if (state.claimability) {
      p.claimability = state.claimability;
    }
    if (debouncedSearch) {
      p.search = debouncedSearch;
    }
    return p;
  }, [state.module, state.accountId, state.claimability, debouncedSearch]);

  const defaultQuery = useFindingsListQuery(selectedScanId, listParams, {
    enabled: Boolean(selectedScanId) && !triage
  });

  const triageCrit = useFindingsListQuery(
    selectedScanId,
    { ...triageBackendParams, severity: "critical" },
    { enabled: Boolean(selectedScanId) && triage }
  );
  const triageHigh = useFindingsListQuery(
    selectedScanId,
    { ...triageBackendParams, severity: "high" },
    { enabled: Boolean(selectedScanId) && triage }
  );

  const mergedTriageItems = useMemo(() => {
    if (!triage) {
      return [] as FindingListItem[];
    }
    const map = new Map<string, FindingListItem>();
    for (const it of triageCrit.data?.items ?? []) {
      map.set(it.id, it);
    }
    for (const it of triageHigh.data?.items ?? []) {
      map.set(it.id, it);
    }
    const arr = [...map.values()];
    arr.sort((a, b) => {
      if (b.monthly_risk_cost_usd !== a.monthly_risk_cost_usd) {
        return b.monthly_risk_cost_usd - a.monthly_risk_cost_usd;
      }
      return a.id.localeCompare(b.id);
    });
    return arr;
  }, [triage, triageCrit.data?.items, triageHigh.data?.items]);

  const triageTotalPages = useMemo(
    () => Math.max(1, Math.ceil(mergedTriageItems.length / state.pageSize)),
    [mergedTriageItems.length, state.pageSize]
  );

  useEffect(() => {
    if (!triage || mergedTriageItems.length === 0) {
      return;
    }
    if (state.page > triageTotalPages) {
      patch({ page: triageTotalPages });
    }
  }, [triage, mergedTriageItems.length, state.page, triageTotalPages, patch]);

  const pageItems: FindingListItem[] = useMemo(() => {
    if (triage) {
      const start = (state.page - 1) * state.pageSize;
      return mergedTriageItems.slice(start, start + state.pageSize);
    }
    return defaultQuery.data?.items ?? [];
  }, [triage, mergedTriageItems, state.page, state.pageSize, defaultQuery.data?.items]);

  const pagination = useMemo(() => {
    if (triage) {
      return {
        page: state.page,
        page_size: state.pageSize,
        total_items: mergedTriageItems.length,
        total_pages: triageTotalPages
      };
    }
    return (
      defaultQuery.data?.pagination ?? {
        page: 1,
        page_size: DEFAULT_PAGE_SIZE,
        total_items: 0,
        total_pages: 1
      }
    );
  }, [triage, state.page, state.pageSize, mergedTriageItems.length, triageTotalPages, defaultQuery.data?.pagination]);

  const isLoading = triage
    ? triageCrit.isPending || triageHigh.isPending
    : defaultQuery.isPending;
  const isError = triage ? triageCrit.isError || triageHigh.isError : defaultQuery.isError;
  const error = triage ? triageCrit.error ?? triageHigh.error : defaultQuery.error;
  const isFetching = triage ? triageCrit.isFetching || triageHigh.isFetching : defaultQuery.isFetching;
  const isSuccess = triage ? triageCrit.isSuccess && triageHigh.isSuccess : defaultQuery.isSuccess;

  useEffect(() => {
    setExpandedId(null);
  }, [selectedScanId, listParams, triageBackendParams, triage]);

  const detailQuery = useFindingDetailQuery(selectedScanId, expandedId, expandedId !== null);

  const hasActiveFilters = Boolean(
    triage ||
      state.severity ||
      state.module ||
      state.accountId ||
      state.claimability ||
      debouncedSearch
  );

  const clearPrefetchTimer = useCallback(() => {
    if (prefetchTimer.current) {
      clearTimeout(prefetchTimer.current);
      prefetchTimer.current = null;
    }
  }, []);

  const onRowPointerEnter = useCallback(
    (findingId: string) => {
      if (!selectedScanId || findingId === expandedId) {
        return;
      }
      clearPrefetchTimer();
      prefetchTimer.current = setTimeout(() => {
        queryClient.prefetchQuery(getFindingDetailQueryOptions(selectedScanId, findingId));
      }, 140);
    },
    [selectedScanId, expandedId, queryClient, clearPrefetchTimer]
  );

  const onRowPointerLeave = useCallback(() => {
    clearPrefetchTimer();
  }, [clearPrefetchTimer]);

  useEffect(() => () => clearPrefetchTimer(), [clearPrefetchTimer]);

  const columns = useMemo(
    () => [
      columnHelper.display({
        id: "expand",
        header: "",
        cell: ({ row }) => {
          const open = expandedId === row.original.id;
          return (
            <button
              type="button"
              aria-expanded={open}
              aria-label={open ? "Collapse finding detail" : "Expand finding detail"}
              className="rounded p-1 text-slate-600 hover:bg-slate-200 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-200"
              onClick={(e) => {
                e.stopPropagation();
                setExpandedId((id) => (id === row.original.id ? null : row.original.id));
              }}
            >
              <span className="inline-block text-xs transition-transform" style={{ transform: open ? "rotate(90deg)" : "none" }}>
                ▸
              </span>
            </button>
          );
        },
        size: 36
      }),
      columnHelper.accessor("severity", {
        header: "Severity",
        cell: (info) => <SeverityBadge severity={info.getValue()} />
      }),
      columnHelper.display({
        id: "target",
        header: "Finding",
        cell: ({ row }) => {
          const hint = findingInlineHint(row.original);
          return (
            <div>
              <div className="font-medium text-slate-800 dark:text-slate-200">{row.original.title?.trim() || "—"}</div>
              <div className="mt-0.5 text-xs text-slate-600 dark:text-slate-400">{displayTarget(row.original.hostname, row.original.affected_arn)}</div>
              {row.original.hostname?.trim() ? (
                <div className="mt-0.5 font-mono text-[11px] text-slate-500 break-all">{shortenArn(row.original.affected_arn, 36, 24)}</div>
              ) : null}
              {hint ? (
                <p className="mt-1 text-[11px] font-medium text-amber-900/90 dark:text-amber-200/85" title="Derived from list fields (module, claimability, title keywords)">
                  {hint}
                </p>
              ) : null}
            </div>
          );
        }
      }),
      columnHelper.display({
        id: "account",
        header: "Account",
        cell: ({ row }) => {
          const name = row.original.account_name?.trim();
          const id = row.original.account_id;
          if (name && id) {
            return (
              <div>
                <div className="text-slate-800 dark:text-slate-200">{name}</div>
                <div className="font-mono text-[11px] text-slate-500">{id}</div>
              </div>
            );
          }
          return <span className="font-mono text-xs text-slate-700 dark:text-slate-300">{id || "—"}</span>;
        }
      }),
      columnHelper.accessor("module", {
        header: "Module",
        cell: (info) => <span className="text-slate-700 dark:text-slate-300">{info.getValue() || "—"}</span>
      }),
      columnHelper.accessor("claimability", {
        header: "Claimability",
        cell: (info) => <span className="capitalize text-slate-700 dark:text-slate-300">{info.getValue() || "—"}</span>
      }),
      columnHelper.accessor("monthly_risk_cost_usd", {
        header: () => <span className="whitespace-nowrap">Risk cost / mo</span>,
        cell: (info) => (
          <span className="tabular-nums text-slate-800 dark:text-slate-200">{formatUsd(info.getValue())}</span>
        )
      })
    ],
    [expandedId]
  );

  const table = useReactTable({
    data: pageItems,
    columns,
    getCoreRowModel: getCoreRowModel()
  });

  const totalPages = Math.max(1, pagination.total_pages);
  const totalItems = pagination.total_items;

  const grouped = useMemo(() => {
    if (!triage || !state.groupByAccount) {
      return null;
    }
    return bucketFindingsByAccount(pageItems);
  }, [triage, state.groupByAccount, pageItems]);

  return (
    <section className="space-y-6">
      <PageHeader
        title={triage ? "Triage" : "Findings"}
        description={
          triage
            ? "Critical and high severity only, merged and sorted by estimated monthly risk on this device. Pagination applies after merge. See banner for fetch limits."
            : "Server-paginated findings with filters from GET /api/scans/:id/findings. Detail loads from GET …/findings/:fid when expanded. Filters sync to the URL for sharing."
        }
        scanId={selectedScanId}
      />
      {triage ? (
        <div
          className="rounded-lg border border-amber-800/40 bg-amber-950/20 px-4 py-3 text-sm text-amber-100/95 dark:border-amber-500/35 dark:bg-amber-950/40"
          role="status"
        >
          <p className="font-medium text-amber-50 dark:text-amber-100">Triage mode: showing highest-risk findings</p>
          <p className="mt-1 text-xs leading-relaxed text-amber-100/85 dark:text-amber-200/80">
            Severity is limited to <strong>critical</strong> and <strong>high</strong>. Rows are sorted by{" "}
            <strong>monthly_risk_cost_usd</strong> descending after merge. Up to {TRIAGE_FETCH_PAGE_SIZE} critical and{" "}
            {TRIAGE_FETCH_PAGE_SIZE} high rows are loaded from the API per request; if your scan exceeds those caps,
            increase filters or use the full Findings view.
          </p>
        </div>
      ) : null}

      {!selectedScanId ? (
        <ScanRequired />
      ) : isLoading ? (
        <StatePanel>Loading findings…</StatePanel>
      ) : isError ? (
        <StatePanel intent="error" title="Failed to load findings">
          <pre className="whitespace-pre-wrap font-sans text-xs">{formatQueryError(error)}</pre>
        </StatePanel>
      ) : isSuccess && totalItems === 0 ? (
        <StatePanel intent="empty" title={hasActiveFilters ? "No matching findings" : "No findings in this scan"}>
          {hasActiveFilters
            ? "The API returned zero rows for the current filters. Adjust filters or clear search — this is a successful empty result, not a request error."
            : "The API returned successfully with zero findings for this scan."}
        </StatePanel>
      ) : pageItems.length > 0 || (!isLoading && isSuccess) ? (
        <>
          <div className="flex flex-col gap-4 rounded-lg border border-slate-200 bg-slate-50/90 p-4 dark:border-slate-800 dark:bg-slate-900/50 lg:flex-row lg:flex-wrap lg:items-end">
            {!triage ? (
              <FilterField label="Severity">
                <select
                  value={state.severity}
                  onChange={(e) => patch({ severity: e.target.value, page: 1 })}
                  className="w-full min-w-[8rem] rounded-md border border-slate-700 bg-white px-2 py-1.5 dark:bg-slate-950 text-sm text-slate-800 dark:text-slate-200"
                >
                  {SEVERITY_OPTIONS.map((v) => (
                    <option key={v || "all"} value={v}>
                      {v ? v : "All"}
                    </option>
                  ))}
                </select>
              </FilterField>
            ) : (
              <FilterField label="Severity">
                <div className="rounded-md border border-slate-700 bg-slate-100 px-2 py-2 text-xs text-slate-700 dark:border-slate-600 dark:bg-slate-950 dark:text-slate-300">
                  Critical + high (fixed)
                </div>
              </FilterField>
            )}
            <FilterField label="Module">
              <select
                value={state.module}
                onChange={(e) => patch({ module: e.target.value, page: 1 })}
                className="w-full min-w-[10rem] rounded-md border border-slate-700 bg-white px-2 py-1.5 dark:bg-slate-950 text-sm text-slate-800 dark:text-slate-200"
              >
                {MODULE_OPTIONS.map((v) => (
                  <option key={v || "all"} value={v}>
                    {v ? v.replace(/_/g, " ") : "All"}
                  </option>
                ))}
              </select>
            </FilterField>
            <FilterField label="Account">
              <select
                value={state.accountId}
                onChange={(e) => patch({ accountId: e.target.value, page: 1 })}
                className="w-full min-w-[12rem] rounded-md border border-slate-700 bg-white px-2 py-1.5 dark:bg-slate-950 text-sm text-slate-800 dark:text-slate-200"
                disabled={accountsQuery.isLoading}
              >
                <option value="">All accounts</option>
                {(accountsQuery.data?.items ?? []).map((a) => (
                  <option key={a.account_id} value={a.account_id}>
                    {a.account_name ? `${a.account_name} (${a.account_id})` : a.account_id}
                  </option>
                ))}
              </select>
            </FilterField>
            <FilterField label="Claimability">
              <select
                value={state.claimability}
                onChange={(e) => patch({ claimability: e.target.value, page: 1 })}
                className="w-full min-w-[10rem] rounded-md border border-slate-700 bg-white px-2 py-1.5 dark:bg-slate-950 text-sm text-slate-800 dark:text-slate-200"
              >
                {CLAIM_OPTIONS.map((v) => (
                  <option key={v || "all"} value={v}>
                    {v ? v.replace(/_/g, " ") : "All"}
                  </option>
                ))}
              </select>
            </FilterField>
            <FilterField label="Search" className="min-w-[16rem] flex-1">
              <input
                type="search"
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                placeholder="Title, ARN, account, hostname…"
                className="w-full rounded-md border border-slate-700 bg-white px-2 py-1.5 dark:bg-slate-950 text-sm text-slate-800 dark:text-slate-200 placeholder:text-slate-400 dark:placeholder:text-slate-600"
              />
              {searchInput.trim() !== debouncedSearch ? (
                <p className="mt-1 text-[11px] text-slate-500">Debouncing search…</p>
              ) : null}
            </FilterField>
            <FilterField label="Page size">
              <select
                value={state.pageSize}
                onChange={(e) => patch({ pageSize: Number(e.target.value) as (typeof PAGE_SIZE_OPTIONS)[number], page: 1 })}
                className="rounded-md border border-slate-700 bg-white px-2 py-1.5 dark:bg-slate-950 text-sm text-slate-800 dark:text-slate-200"
              >
                {PAGE_SIZE_OPTIONS.map((n) => (
                  <option key={n} value={n}>
                    {n}
                  </option>
                ))}
              </select>
            </FilterField>
            {triage ? (
              <FilterField label="Layout">
                <label className="flex cursor-pointer items-center gap-2 text-sm text-slate-700 dark:text-slate-300">
                  <input
                    type="checkbox"
                    className="rounded border-slate-600"
                    checked={state.groupByAccount}
                    onChange={(e) => patch({ groupByAccount: e.target.checked, page: 1 })}
                  />
                  Group by account
                </label>
              </FilterField>
            ) : null}
          </div>

          <AppliedFiltersBar
            triage={triage}
            severity={state.severity}
            moduleFilter={state.module}
            accountId={state.accountId}
            claimability={state.claimability}
            search={debouncedSearch}
            groupByAccount={state.groupByAccount}
          />

          <div className="overflow-x-auto rounded-lg border border-slate-200 dark:border-slate-800">
            <table className="w-full min-w-[56rem] border-collapse text-left text-sm">
              <thead className="sticky top-0 z-10 border-b border-slate-200 bg-slate-100/95 backdrop-blur-sm dark:border-b-slate-800 dark:bg-slate-100/95 dark:bg-slate-950/95">
                {table.getHeaderGroups().map((hg) => (
                  <tr key={hg.id}>
                    {hg.headers.map((header) => (
                      <th
                        key={header.id}
                        className="whitespace-nowrap px-3 py-3 text-xs font-semibold uppercase tracking-wide text-slate-500"
                      >
                        {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                      </th>
                    ))}
                  </tr>
                ))}
              </thead>
              <tbody className="divide-y divide-slate-200 dark:divide-slate-800/90">
                {grouped
                  ? grouped.flatMap((g) => [
                      <tr key={`h-${g.accountId}`} className="bg-slate-200/90 dark:bg-slate-800/80">
                        <td colSpan={columns.length} className="px-3 py-2 text-xs font-semibold text-slate-700 dark:text-slate-200">
                          Account <span className="font-mono text-cyan-900 dark:text-cyan-200/90">{g.accountId}</span>
                          <span className="ml-2 font-normal text-slate-500">({g.items.length} on this page)</span>
                        </td>
                      </tr>,
                      ...g.items.map((rowItem) => renderFindingRowFragment(rowItem))
                    ])
                  : table.getRowModel().rows.map((row) => renderFindingRowFragment(row.original))}
              </tbody>
            </table>
          </div>

          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <p className="text-sm text-slate-500">
              Page <span className="tabular-nums text-slate-700 dark:text-slate-300">{pagination.page}</span> of{" "}
              <span className="tabular-nums text-slate-700 dark:text-slate-300">{totalPages}</span>
              <span className="mx-2 text-slate-500 dark:text-slate-600">·</span>
              <span className="tabular-nums text-slate-700 dark:text-slate-300">{totalItems}</span> total
            </p>
            <div className="flex gap-2">
              <button
                type="button"
                disabled={state.page <= 1 || isFetching}
                onClick={() => patch({ page: Math.max(1, state.page - 1) })}
                className="rounded-md border border-slate-700 bg-white px-3 py-1.5 text-sm text-slate-800 dark:bg-slate-900 dark:text-slate-200 disabled:opacity-40"
              >
                Previous
              </button>
              <button
                type="button"
                disabled={state.page >= totalPages || isFetching}
                onClick={() => patch({ page: state.page + 1 })}
                className="rounded-md border border-slate-700 bg-white px-3 py-1.5 text-sm text-slate-800 dark:bg-slate-900 dark:text-slate-200 disabled:opacity-40"
              >
                Next
              </button>
            </div>
          </div>
        </>
      ) : (
        <StatePanel intent="empty" title="No data">Unexpected empty response after success.</StatePanel>
      )}
    </section>
  );

  function renderFindingRowFragment(rowItem: FindingListItem) {
    const row = table.getRowModel().rows.find((r) => r.original.id === rowItem.id);
    if (!row) {
      return null;
    }
    return (
      <Fragment key={row.id}>
        <tr
          className="bg-slate-50/80 hover:bg-slate-100 dark:bg-slate-50/90 dark:bg-slate-900/40 dark:hover:bg-slate-900/80"
          onPointerEnter={() => onRowPointerEnter(rowItem.id)}
          onPointerLeave={onRowPointerLeave}
        >
          {row.getVisibleCells().map((cell) => (
            <td key={cell.id} className="px-3 py-3 align-top text-slate-700 dark:text-slate-300">
              {flexRender(cell.column.columnDef.cell, cell.getContext())}
            </td>
          ))}
        </tr>
        {expandedId === rowItem.id ? (
          <tr className="bg-slate-100/80 dark:bg-slate-950/80">
            <td colSpan={columns.length} className="border-t border-slate-200 px-4 py-4 dark:border-t-slate-800">
              {detailQuery.isLoading ? (
                <FindingDetailPanelLoading />
              ) : detailQuery.isError ? (
                <FindingDetailPanelError error={detailQuery.error} />
              ) : detailQuery.data?.item ? (
                <FindingDetailPanelContent item={detailQuery.data.item} />
              ) : (
                <StatePanel intent="empty">No detail payload.</StatePanel>
              )}
            </td>
          </tr>
        ) : null}
      </Fragment>
    );
  }
}

function FilterField({
  label,
  children,
  className = ""
}: {
  label: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={className}>
      <label className="mb-1 block text-[11px] font-medium uppercase tracking-wide text-slate-500">{label}</label>
      {children}
    </div>
  );
}

function AppliedFiltersBar({
  triage,
  severity,
  moduleFilter,
  accountId,
  claimability,
  search,
  groupByAccount
}: {
  triage: boolean;
  severity: string;
  moduleFilter: string;
  accountId: string;
  claimability: string;
  search: string;
  groupByAccount: boolean;
}) {
  const chips: { label: string; value: string }[] = [];
  if (triage) {
    chips.push({ label: "mode", value: "triage (critical+high)" });
  }
  if (severity) {
    chips.push({ label: "severity", value: severity });
  }
  if (moduleFilter) {
    chips.push({ label: "module", value: moduleFilter });
  }
  if (accountId) {
    chips.push({ label: "account", value: accountId });
  }
  if (claimability) {
    chips.push({ label: "claimability", value: claimability });
  }
  if (search) {
    chips.push({ label: "search", value: search });
  }
  if (triage && groupByAccount) {
    chips.push({ label: "group_by", value: "account" });
  }

  return (
    <div className="flex flex-wrap items-center gap-2 text-xs">
      <span className="text-slate-500">Applied filters</span>
      {chips.length === 0 ? (
        <span className="text-slate-500 dark:text-slate-600">— none (unfiltered within this page set)</span>
      ) : (
        chips.map((c) => (
          <span
            key={`${c.label}-${c.value}`}
            className="rounded-full border border-slate-300 bg-slate-100 px-2 py-0.5 dark:border-slate-700 dark:bg-slate-900 text-slate-700 dark:text-slate-300"
          >
            <span className="text-slate-500">{c.label}=</span>
            <span className="font-mono text-[11px] text-cyan-800 dark:text-cyan-200/90">{c.value}</span>
          </span>
        ))
      )}
    </div>
  );
}
