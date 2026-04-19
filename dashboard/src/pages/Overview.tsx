import { formatQueryError } from "../api/httpError";
import type { ScanSummaryResponse } from "../api/types";
import { PageHeader } from "../components/PageHeader";
import {
  ClaimabilityDistribution,
  DirectVsRiskCostSplit,
  ModuleDistribution,
  SeverityDistribution
} from "../components/overview/SummaryVisualizations";
import { TopAccountsHorizontalBar } from "../components/overview/TopAccountsHorizontalBar";
import { ScanRequired } from "../components/ScanRequired";
import { StatePanel } from "../components/StatePanel";
import { useAccountsQuery, useSummaryQuery } from "../hooks/useDashboardQueries";
import { useScanContext } from "../hooks/useScanContext";
import { formatCount, formatUsd } from "../lib/format";

export function OverviewPage() {
  const { selectedScanId } = useScanContext();
  const query = useSummaryQuery();
  const accountsQuery = useAccountsQuery();

  return (
    <section className="space-y-8">
      <PageHeader
        title="Overview"
        description="Scan summary from GET /api/scans/:id/summary — counts and costs are shown exactly as returned by the API."
        scanId={selectedScanId}
      />
      {!selectedScanId ? (
        <ScanRequired />
      ) : query.isLoading ? (
        <StatePanel>Loading summary…</StatePanel>
      ) : query.isError ? (
        <StatePanel intent="error" title="Failed to load summary">
          <pre className="whitespace-pre-wrap font-sans text-xs">{formatQueryError(query.error)}</pre>
        </StatePanel>
      ) : query.data ? (
        <>
          <KpiGrid summary={query.data} />
          <div className="grid gap-6 lg:grid-cols-2 xl:grid-cols-4">
            <SeverityDistribution summary={query.data} />
            <ModuleDistribution summary={query.data} />
            <ClaimabilityDistribution summary={query.data} />
            <DirectVsRiskCostSplit summary={query.data} />
          </div>
          {accountsQuery.isLoading ? (
            <StatePanel>Loading accounts breakdown…</StatePanel>
          ) : accountsQuery.isError ? (
            <div className="rounded-lg border border-slate-200 bg-rose-50 p-4 text-sm text-rose-900 dark:border-slate-800 dark:bg-slate-900/70 dark:text-rose-300/95">
              Top accounts chart unavailable ({formatQueryError(accountsQuery.error)}).
            </div>
          ) : accountsQuery.data ? (
            <TopAccountsHorizontalBar items={accountsQuery.data.items} />
          ) : null}
        </>
      ) : (
        <StatePanel intent="empty" title="No data">Summary response was empty.</StatePanel>
      )}
    </section>
  );
}

function KpiGrid({ summary }: { summary: ScanSummaryResponse }) {
  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-6">
      <KpiCard label="Critical findings" value={formatCount(summary.critical_count)} emphasis="rose" />
      <KpiCard label="High findings" value={formatCount(summary.high_count)} emphasis="orange" />
      <KpiCard label="Total findings" value={formatCount(summary.finding_count)} emphasis="neutral" />
      <KpiCard
        label="Monthly risk cost"
        value={formatUsd(summary.total_monthly_risk_cost_usd)}
        emphasis="neutral"
      />
      <KpiCard label="Reclaimable" value={formatCount(summary.reclaimable_count)} emphasis="emerald" />
      <KpiCard label="External access" value={formatCount(summary.external_access_count)} emphasis="violet" />
    </div>
  );
}

const emphasisBorder: Record<string, string> = {
  rose: "border-l-rose-500/80",
  orange: "border-l-orange-500/80",
  emerald: "border-l-emerald-500/70",
  violet: "border-l-violet-500/70",
  neutral: "border-l-slate-300 dark:border-l-slate-600"
};

function KpiCard({
  label,
  value,
  emphasis = "neutral"
}: {
  label: string;
  value: string;
  emphasis?: keyof typeof emphasisBorder;
}) {
  return (
    <div
      className={`rounded-lg border border-slate-200 border-l-4 bg-white py-4 pl-4 pr-3 shadow-sm shadow-slate-200/60 dark:border-slate-800 dark:bg-slate-900/90 dark:shadow-black/10 ${emphasisBorder[emphasis]}`}
    >
      <p className="text-[11px] font-medium uppercase tracking-wide text-slate-500">{label}</p>
      <p className="mt-2 text-2xl font-semibold tabular-nums tracking-tight text-slate-900 dark:text-slate-100">{value}</p>
    </div>
  );
}
