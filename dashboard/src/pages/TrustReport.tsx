import { Fragment, useEffect, useMemo, useState } from "react";
import { formatQueryError } from "../api/httpError";
import type { FindingListItem, FindingsQueryParams } from "../api/types";
import {
  FindingDetailPanelContent,
  FindingDetailPanelError,
  FindingDetailPanelLoading
} from "../components/findings/FindingDetailPanel";
import { PageHeader } from "../components/PageHeader";
import { SeverityBadge } from "../components/SeverityBadge";
import { TrustActivityCallout } from "../components/trust/TrustActivityCallout";
import { TrustActivityAgingChart } from "../components/trust/TrustActivityAgingChart";
import { ScanRequired } from "../components/ScanRequired";
import { StatePanel } from "../components/StatePanel";
import { useFindingDetailQuery, useFindingsListQuery } from "../hooks/useDashboardQueries";
import { useDebouncedValue } from "../hooks/useDebouncedValue";
import { useScanContext } from "../hooks/useScanContext";
import { formatUsd, shortenArn } from "../lib/format";

const PAGE_SIZE_OPTIONS = [25, 50, 100] as const;
const SEARCH_DEBOUNCE_MS = 350;
const SEVERITY_OPTIONS = ["", "critical", "high", "medium", "low"] as const;

/**
 * Trust report: external_access findings only (module fixed server-side).
 *
 * Principal type is not a query parameter on GET …/findings and is absent from list DTO rows.
 * Filtering by principal type would require per-row detail or API changes — not done here.
 * Users can use search to match principal strings when they appear in indexed fields.
 */
export function TrustReportPage() {
  const { selectedScanId } = useScanContext();
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState<number>(50);
  const [severity, setSeverity] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const debouncedSearch = useDebouncedValue(searchInput.trim(), SEARCH_DEBOUNCE_MS);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const listParams: FindingsQueryParams = useMemo(() => {
    const p: FindingsQueryParams = {
      module: "external_access",
      page,
      page_size: pageSize
    };
    if (severity) {
      p.severity = severity;
    }
    if (debouncedSearch) {
      p.search = debouncedSearch;
    }
    return p;
  }, [page, pageSize, severity, debouncedSearch]);

  useEffect(() => {
    setPage(1);
  }, [severity, debouncedSearch, pageSize]);

  useEffect(() => {
    setExpandedId(null);
  }, [selectedScanId, listParams]);

  const query = useFindingsListQuery(selectedScanId, listParams);
  // Same queryKey/queryFn/staleTime as trust activity sampling (`fetchQuery` in useTrustActivityBucketsQuery):
  // expanding a row that was already in the aging sample reuses cached detail when still fresh.
  const detailQuery = useFindingDetailQuery(selectedScanId, expandedId, expandedId !== null);

  const pageFindingIds = useMemo(() => query.data?.items.map((i) => i.id) ?? [], [query.data?.items]);

  const totalItems = query.data?.pagination.total_items ?? 0;
  const totalPages = Math.max(1, query.data?.pagination.total_pages ?? 1);
  const hasFilters = Boolean(severity || debouncedSearch);

  return (
    <section className="space-y-6">
      <PageHeader
        title="Trust report"
        description="External-access findings via GET …/findings?module=external_access. Trust columns below use list fields; expand a row for full trust metadata and IAM activity notes."
        scanId={selectedScanId}
      />
      {!selectedScanId ? (
        <ScanRequired />
      ) : query.isLoading ? (
        <StatePanel>Loading trust findings…</StatePanel>
      ) : query.isError ? (
        <StatePanel intent="error" title="Failed to load trust findings">
          <pre className="whitespace-pre-wrap font-sans text-xs">{formatQueryError(query.error)}</pre>
        </StatePanel>
      ) : query.isSuccess && totalItems === 0 ? (
        <StatePanel intent="empty" title={hasFilters ? "No matching trust findings" : "No external-access findings"}>
          {hasFilters
            ? "No rows for the current severity/search on this scan (successful empty result)."
            : "No findings with module external_access for this scan."}
        </StatePanel>
      ) : query.data ? (
        <>
          <div className="flex flex-col gap-4 rounded-lg border border-slate-200 bg-slate-50/90 p-4 dark:border-slate-800 dark:bg-slate-900/50 lg:flex-row lg:flex-wrap lg:items-end">
            <div>
              <label className="mb-1 block text-[11px] font-medium uppercase tracking-wide text-slate-500">
                Severity
              </label>
              <select
                value={severity}
                onChange={(e) => setSeverity(e.target.value)}
                className="w-full min-w-[8rem] rounded-md border border-slate-700 bg-white px-2 py-1.5 dark:bg-slate-950 text-sm text-slate-800 dark:text-slate-200"
              >
                {SEVERITY_OPTIONS.map((v) => (
                  <option key={v || "all"} value={v}>
                    {v ? v : "All"}
                  </option>
                ))}
              </select>
            </div>
            <div className="min-w-[16rem] flex-1">
              <label className="mb-1 block text-[11px] font-medium uppercase tracking-wide text-slate-500">
                Search
              </label>
              <input
                type="search"
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                placeholder="Principal, ARN, title, account…"
                className="w-full rounded-md border border-slate-700 bg-white px-2 py-1.5 dark:bg-slate-950 text-sm text-slate-800 dark:text-slate-200 placeholder:text-slate-400 dark:placeholder:text-slate-600"
              />
              {searchInput.trim() !== debouncedSearch ? (
                <p className="mt-1 text-[11px] text-slate-500">Debouncing…</p>
              ) : null}
            </div>
            <div>
              <label className="mb-1 block text-[11px] font-medium uppercase tracking-wide text-slate-500">
                Page size
              </label>
              <select
                value={pageSize}
                onChange={(e) => setPageSize(Number(e.target.value))}
                className="rounded-md border border-slate-700 bg-white px-2 py-1.5 dark:bg-slate-950 text-sm text-slate-800 dark:text-slate-200"
              >
                {PAGE_SIZE_OPTIONS.map((n) => (
                  <option key={n} value={n}>
                    {n}
                  </option>
                ))}
              </select>
            </div>
          </div>

          <p className="text-xs text-slate-500">
            Principal-type filtering is not exposed in the list API; use search or expand rows for principal type from
            detail.
          </p>

          <TrustActivityAgingChart
            scanId={selectedScanId}
            pageFindingIds={pageFindingIds}
            pageSize={pageSize}
            pageTotalItems={totalItems}
          />

          <div className="overflow-x-auto rounded-lg border border-slate-200 dark:border-slate-800">
            <table className="w-full min-w-[64rem] border-collapse text-left text-sm">
              <thead className="border-b border-slate-200 bg-slate-100/95 dark:border-b-slate-800 dark:bg-slate-100/95 dark:bg-slate-950/95">
                <tr>
                  <th className="w-10 px-2 py-3" />
                  <th className="px-3 py-3 text-xs font-semibold uppercase tracking-wide text-slate-500">Severity</th>
                  <th className="px-3 py-3 text-xs font-semibold uppercase tracking-wide text-slate-500">Role (ARN)</th>
                  <th className="px-3 py-3 text-xs font-semibold uppercase tracking-wide text-slate-500">Account</th>
                  <th className="px-3 py-3 text-xs font-semibold uppercase tracking-wide text-slate-500">Principal*</th>
                  <th className="px-3 py-3 text-xs font-semibold uppercase tracking-wide text-slate-500">Type*</th>
                  <th className="px-3 py-3 text-xs font-semibold uppercase tracking-wide text-slate-500">Ext acct*</th>
                  <th className="px-3 py-3 text-xs font-semibold uppercase tracking-wide text-slate-500">Days*</th>
                  <th className="px-3 py-3 text-xs font-semibold uppercase tracking-wide text-slate-500">Verdict*</th>
                  <th className="px-3 py-3 text-xs font-semibold uppercase tracking-wide text-slate-500">Admin*</th>
                  <th className="px-3 py-3 text-xs font-semibold uppercase tracking-wide text-slate-500">Vendor*</th>
                  <th className="px-3 py-3 text-xs font-semibold uppercase tracking-wide text-slate-500">Risk / mo</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-200 dark:divide-slate-800/90">
                {query.data.items.map((item) => (
                  <Fragment key={item.id}>
                    <TrustTableRow
                      item={item}
                      expanded={expandedId === item.id}
                      onToggleExpand={() => setExpandedId((id) => (id === item.id ? null : item.id))}
                    />
                    {expandedId === item.id ? (
                      <tr className="bg-slate-100/90 dark:bg-slate-950/90">
                        <td colSpan={12} className="px-4 py-4">
                          {detailQuery.isLoading ? (
                            <FindingDetailPanelLoading />
                          ) : detailQuery.isError ? (
                            <FindingDetailPanelError error={detailQuery.error} />
                          ) : detailQuery.data?.item ? (
                            <div className="space-y-4">
                              <TrustActivityCallout trust={detailQuery.data.item.trust} />
                              <FindingDetailPanelContent item={detailQuery.data.item} />
                            </div>
                          ) : (
                            <StatePanel intent="empty">No detail payload.</StatePanel>
                          )}
                        </td>
                      </tr>
                    ) : null}
                  </Fragment>
                ))}
              </tbody>
            </table>
          </div>
          <p className="text-[11px] text-slate-500 dark:text-slate-600">* Populated from expanded finding detail (trust block), not the list endpoint.</p>

          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <p className="text-sm text-slate-500">
              Page <span className="tabular-nums text-slate-700 dark:text-slate-300">{query.data.pagination.page}</span> of{" "}
              <span className="tabular-nums text-slate-700 dark:text-slate-300">{totalPages}</span>
              <span className="mx-2 text-slate-500 dark:text-slate-600">·</span>
              <span className="tabular-nums text-slate-700 dark:text-slate-300">{totalItems}</span> trust findings
            </p>
            <div className="flex gap-2">
              <button
                type="button"
                disabled={page <= 1 || query.isFetching}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                className="rounded-md border border-slate-700 bg-white px-3 py-1.5 text-sm text-slate-800 dark:bg-slate-900 dark:text-slate-200 disabled:opacity-40"
              >
                Previous
              </button>
              <button
                type="button"
                disabled={page >= totalPages || query.isFetching}
                onClick={() => setPage((p) => p + 1)}
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
}

function TrustTableRow({
  item,
  expanded,
  onToggleExpand
}: {
  item: FindingListItem;
  expanded: boolean;
  onToggleExpand: () => void;
}) {
  const pending = <span className="text-slate-500 dark:text-slate-600">—</span>;
  return (
    <tr className="bg-slate-50/80 hover:bg-slate-100 dark:bg-slate-50/90 dark:bg-slate-900/40 dark:hover:bg-slate-900/75">
      <td className="px-2 py-3 align-top">
        <button
          type="button"
          aria-expanded={expanded}
          aria-label={expanded ? "Collapse detail" : "Expand detail"}
          className="rounded p-1 text-slate-600 hover:bg-slate-200 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-200"
          onClick={(e) => {
            e.stopPropagation();
            onToggleExpand();
          }}
        >
          <span className="inline-block text-xs" style={{ transform: expanded ? "rotate(90deg)" : "none" }}>
            ▸
          </span>
        </button>
      </td>
      <td className="px-3 py-3 align-top">
        <SeverityBadge severity={item.severity} />
      </td>
      <td className="max-w-[14rem] px-3 py-3 align-top font-mono text-xs text-slate-700 dark:text-slate-300 break-all" title={item.affected_arn}>
        {shortenArn(item.affected_arn, 20, 16)}
      </td>
      <td className="px-3 py-3 align-top text-slate-700 dark:text-slate-300">
        <div className="font-mono text-xs">{item.account_id || "—"}</div>
        {item.account_name ? <div className="text-[11px] text-slate-500">{item.account_name}</div> : null}
      </td>
      <td className="px-3 py-3 align-top text-xs text-slate-500 dark:text-slate-600">{pending}</td>
      <td className="px-3 py-3 align-top text-xs text-slate-500 dark:text-slate-600">{pending}</td>
      <td className="px-3 py-3 align-top text-xs text-slate-500 dark:text-slate-600">{pending}</td>
      <td className="px-3 py-3 align-top text-xs text-slate-500 dark:text-slate-600">{pending}</td>
      <td className="px-3 py-3 align-top text-xs text-slate-500 dark:text-slate-600">{pending}</td>
      <td className="px-3 py-3 align-top text-xs text-slate-500 dark:text-slate-600">{pending}</td>
      <td className="px-3 py-3 align-top text-xs text-slate-500 dark:text-slate-600">{pending}</td>
      <td className="px-3 py-3 align-top tabular-nums text-slate-800 dark:text-slate-200">{formatUsd(item.monthly_risk_cost_usd)}</td>
    </tr>
  );
}
