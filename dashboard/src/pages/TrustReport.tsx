import { Fragment, useEffect, useMemo, useState } from "react";
import { formatQueryError } from "../api/httpError";
import type { FindingListItem, FindingsQueryParams } from "../api/types";
import {
  FindingDetailPanelContent,
  FindingDetailPanelError,
  FindingDetailPanelLoading
} from "../components/findings/FindingDetailPanel";
import { InterimHeuristicIndicator } from "../components/InterimHeuristicIndicator";
import { PageHeader } from "../components/PageHeader";
import { SeverityBadge } from "../components/SeverityBadge";
import { TrustActivityCallout } from "../components/trust/TrustActivityCallout";
import { TrustActivityAgingChart } from "../components/trust/TrustActivityAgingChart";
import { CachedPermissionTierChip } from "../components/trust/CachedPermissionTierChip";
import { ScanRequired } from "../components/ScanRequired";
import { StatePanel } from "../components/StatePanel";
import { useFindingDetailQuery, useFindingsListQuery } from "../hooks/useDashboardQueries";
import { useDebouncedValue } from "../hooks/useDebouncedValue";
import { useScanContext } from "../hooks/useScanContext";
import { useTrustReportUrlState } from "../hooks/useTrustReportUrlState";
import { formatUsd, shortenArn } from "../lib/format";

const PAGE_SIZE_OPTIONS = [25, 50, 100] as const;
const SEARCH_DEBOUNCE_MS = 350;
const SEVERITY_OPTIONS = ["", "critical", "high", "medium", "low"] as const;

// Best-effort UX heuristic based on existing list-row text only.
// Not an authoritative trust classification until structured trust fields
// are exposed on the list endpoint.
function deriveActivityLabel(item: FindingListItem): "Never used" | "Stale" | "Active" {
  const title = item.title.toLowerCase();
  if (title.includes("never used")) {
    return "Never used";
  }
  if (title.includes("stale") || title.includes("unused")) {
    return "Stale";
  }
  return "Active";
}

// Best-effort UX heuristic based on existing list-row text only.
// This is intentionally non-authoritative and should be replaced by
// structured trust/admin fields when available.
function deriveAdminSignal(item: FindingListItem): "Admin-like" | "Not evaluated" {
  const title = item.title.toLowerCase();
  return title.includes("admin") || title.includes("privilege") ? "Admin-like" : "Not evaluated";
}

/** Trust report: external_access findings with structured trust filters on GET …/findings. */
export function TrustReportPage() {
  const { selectedScanId } = useScanContext();
  const { state, patch } = useTrustReportUrlState();
  const [searchInput, setSearchInput] = useState(state.search);
  const debouncedSearch = useDebouncedValue(searchInput.trim(), SEARCH_DEBOUNCE_MS);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  useEffect(() => {
    setSearchInput(state.search);
  }, [state.search]);

  useEffect(() => {
    if (debouncedSearch === state.search) {
      return;
    }
    patch({ search: debouncedSearch, page: 1 });
  }, [debouncedSearch, state.search, patch]);

  const listParams: FindingsQueryParams = useMemo(() => {
    const p: FindingsQueryParams = {
      module: "external_access",
      page: state.page,
      page_size: state.pageSize
    };
    if (state.severity) {
      p.severity = state.severity;
    }
    if (debouncedSearch) {
      p.search = debouncedSearch;
    }
    if (state.trustStale) {
      p.trust_stale = true;
    }
    if (state.adminLike) {
      p.admin_like = true;
    }
    if (state.trustClassification.trim()) {
      p.trust_classification = state.trustClassification.trim();
    }
    if (state.principalType.trim()) {
      p.principal_type = state.principalType.trim();
    }
    return p;
  }, [
    state.page,
    state.pageSize,
    state.severity,
    debouncedSearch,
    state.trustStale,
    state.adminLike,
    state.trustClassification,
    state.principalType
  ]);

  useEffect(() => {
    setExpandedId(null);
  }, [selectedScanId, listParams]);

  const query = useFindingsListQuery(selectedScanId, listParams);
  // Same queryKey/queryFn/staleTime as trust activity sampling (`fetchQuery` in useTrustActivityBucketsQuery):
  // expanding a row that was already in the aging sample reuses cached detail when still fresh.
  const detailQuery = useFindingDetailQuery(selectedScanId, expandedId, expandedId !== null);
  const trustItems = query.data?.items ?? [];

  const pageFindingIds = useMemo(() => trustItems.map((i) => i.id), [trustItems]);

  const totalItems = query.data?.pagination.total_items ?? 0;
  const totalPages = Math.max(1, query.data?.pagination.total_pages ?? 1);
  const hasFilters = Boolean(
    state.severity ||
      debouncedSearch ||
      state.trustStale ||
      state.adminLike ||
      state.trustClassification.trim() ||
      state.principalType.trim()
  );

  return (
    <section className="space-y-6">
      <PageHeader
        title="Access"
        description="GET …/findings?module=external_access. Privileged = permission tier (trust_classification). Admin-like = separate admin_like capability flag (stronger signal); neither implies the other. Row * columns still use expand/detail or heuristics."
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
                value={state.pageSize}
                onChange={(e) => patch({ pageSize: Number(e.target.value) as 25 | 50 | 100, page: 1 })}
                className="rounded-md border border-slate-700 bg-white px-2 py-1.5 dark:bg-slate-950 text-sm text-slate-800 dark:text-slate-200"
              >
                {PAGE_SIZE_OPTIONS.map((n) => (
                  <option key={n} value={n}>
                    {n}
                  </option>
                ))}
              </select>
            </div>
            <div title="Structured: evidence.verdict === stale_review_now. Independent of permission tier and admin_like.">
              <label className="mb-1 block text-[11px] font-medium uppercase tracking-wide text-slate-500">
                Trust stale
              </label>
              <label className="flex cursor-pointer items-center gap-2 rounded-md border border-slate-700 bg-white px-2 py-2 text-sm text-slate-800 dark:border-slate-600 dark:bg-slate-950 dark:text-slate-200">
                <input
                  type="checkbox"
                  className="rounded border-slate-600"
                  checked={state.trustStale}
                  onChange={(e) => patch({ trustStale: e.target.checked, page: 1 })}
                />
                Verdict stale_review_now
              </label>
            </div>
            <div title="Capability flag: permission_visibility.capabilities.admin_like. Distinct from privileged tier — neither implies the other; overlap is possible.">
              <label className="mb-1 block text-[11px] font-medium uppercase tracking-wide text-slate-500">
                Admin-like (flag)
              </label>
              <label className="flex cursor-pointer items-center gap-2 rounded-md border border-slate-700 bg-white px-2 py-2 text-sm text-slate-800 dark:border-slate-600 dark:bg-slate-950 dark:text-slate-200">
                <input
                  type="checkbox"
                  className="rounded border-slate-600"
                  checked={state.adminLike}
                  onChange={(e) => patch({ adminLike: e.target.checked, page: 1 })}
                />
                admin_like
              </label>
            </div>
            <div title="Backend permission tier: permission_visibility.classification (e.g. privileged, admin). Not the admin_like capability — use both filters if you need overlap logic.">
              <label className="mb-1 block text-[11px] font-medium uppercase tracking-wide text-slate-500">
                Permission tier
              </label>
              <select
                value={state.trustClassification}
                onChange={(e) => patch({ trustClassification: e.target.value, page: 1 })}
                className="w-full min-w-[9rem] rounded-md border border-slate-700 bg-white px-2 py-1.5 dark:bg-slate-950 text-sm text-slate-800 dark:text-slate-200"
              >
                <option value="">Any</option>
                <option value="admin">admin</option>
                <option value="privileged">privileged</option>
                <option value="scoped">scoped</option>
                <option value="limited">limited</option>
                <option value="unknown">unknown</option>
              </select>
            </div>
            <div className="min-w-[10rem]">
              <label className="mb-1 block text-[11px] font-medium uppercase tracking-wide text-slate-500">
                Principal type
              </label>
              <input
                type="text"
                value={state.principalType}
                onChange={(e) => patch({ principalType: e.target.value, page: 1 })}
                placeholder="oidc, aws_account…"
                className="w-full rounded-md border border-slate-700 bg-white px-2 py-1.5 dark:bg-slate-950 text-sm text-slate-800 dark:text-slate-200 placeholder:text-slate-400 dark:placeholder:text-slate-600"
              />
            </div>
          </div>

          <p className="text-xs text-slate-500">
            Table columns marked * remain expand/detail-backed or title heuristics; list filtering uses structured evidence
            fields above.
            <span className="ml-2 inline-flex align-middle">
              <InterimHeuristicIndicator label="interim" className="align-middle" />
            </span>
          </p>
          <p className="text-xs text-slate-500">
            Permission visibility (capabilities, confidence, analysis status, reasons) is primary in expanded detail. A
            compact permission tier may appear beside a row only when that finding’s detail is already in the client
            cache (e.g. after expand or activity chart sampling) — no extra list-only fetches.
          </p>

          <TrustActivityAgingChart
            scanId={selectedScanId}
            pageFindingIds={pageFindingIds}
            pageSize={state.pageSize}
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
                {trustItems.map((item) => (
                  <Fragment key={item.id}>
                    <TrustTableRow
                      scanId={selectedScanId}
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
                disabled={state.page <= 1 || query.isFetching}
                onClick={() => patch({ page: Math.max(1, state.page - 1) })}
                className="rounded-md border border-slate-700 bg-white px-3 py-1.5 text-sm text-slate-800 dark:bg-slate-900 dark:text-slate-200 disabled:opacity-40"
              >
                Previous
              </button>
              <button
                type="button"
                disabled={state.page >= totalPages || query.isFetching}
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
}

function TrustTableRow({
  scanId,
  item,
  expanded,
  onToggleExpand
}: {
  scanId: string;
  item: FindingListItem;
  expanded: boolean;
  onToggleExpand: () => void;
}) {
  const pending = <span className="text-slate-500 dark:text-slate-600">Open detail</span>;
  const activity = deriveActivityLabel(item);
  const adminSignal = deriveAdminSignal(item);
  const highRiskCombination =
    (item.severity === "critical" || item.severity === "high") &&
    (activity === "Never used" || activity === "Stale") &&
    adminSignal === "Admin-like";

  return (
    <tr
      className={`bg-slate-50/80 hover:bg-slate-100 dark:bg-slate-50/90 dark:bg-slate-900/40 dark:hover:bg-slate-900/75 ${
        highRiskCombination ? "border-l-4 border-l-rose-500/80" : ""
      }`}
    >
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
        <div>{shortenArn(item.affected_arn, 20, 16)}</div>
        <div className="mt-1 flex flex-wrap items-center gap-1">
          <CachedPermissionTierChip scanId={scanId} findingId={item.id} />
          <span className="rounded-full border border-slate-300/80 bg-slate-100 px-1.5 py-0.5 text-[10px] uppercase tracking-wide text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300">
            {activity}
          </span>
          <span
            className={`rounded-full border px-1.5 py-0.5 text-[10px] uppercase tracking-wide ${
              adminSignal === "Admin-like"
                ? "border-rose-300/80 bg-rose-100 text-rose-800 dark:border-rose-700/80 dark:bg-rose-900/30 dark:text-rose-200"
                : "border-slate-300/80 bg-slate-100 text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-400"
            }`}
          >
            {adminSignal}
          </span>
          {highRiskCombination ? (
            <span className="rounded-full border border-amber-300/80 bg-amber-100 px-1.5 py-0.5 text-[10px] uppercase tracking-wide text-amber-900 dark:border-amber-700/70 dark:bg-amber-900/35 dark:text-amber-200">
              High-risk combo
            </span>
          ) : null}
          <InterimHeuristicIndicator
            label="inferred"
            tooltip="Activity/admin/high-risk-combo chips are inferred from list-row text keywords and severity, not structured trust fields."
          />
        </div>
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
