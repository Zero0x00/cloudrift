import { Link } from "react-router-dom";
import { AreaChart } from "@tremor/react";
import type { ScanListItem } from "../../api/types";
import { useScanRiskTrendData } from "../../hooks/useDashboardQueries";
import { formatCount } from "../../lib/format";
import { EXEC_CHART_COLORS } from "./chartColors";

export function ScanRiskTrendChart({
  scans,
  selectedScanId,
  enabled
}: {
  scans: ScanListItem[];
  selectedScanId: string | null;
  enabled: boolean;
}) {
  const { points, isLoading, pairCount } = useScanRiskTrendData(scans, enabled);

  if (!enabled || pairCount === 0) {
    return (
      <div className="rounded-lg border border-slate-200 bg-white/80 p-5 dark:border-slate-800 dark:bg-slate-900/80">
        <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-400">Risk trend</h3>
        <p className="mt-2 text-sm text-slate-500">Add at least two scans to see new vs resolved drift between consecutive runs.</p>
      </div>
    );
  }

  const chartData = points.map((p) => ({
    label: p.label,
    New: p.isError ? 0 : p.newCount,
    Resolved: p.isError ? 0 : p.resolvedCount
  }));

  return (
    <div className="hs-card p-5">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <div>
          <h3 className="cr-section-title">Risk trend</h3>
          <p className="cr-helper mt-1">New vs resolved findings across the latest scan transitions.</p>
        </div>
        {selectedScanId ? (
          <Link
            to={`/diff?scan_id=${encodeURIComponent(selectedScanId)}`}
            className="text-xs font-medium text-cyan-700 underline-offset-2 hover:underline dark:text-cyan-300/90"
          >
            Open changes
          </Link>
        ) : null}
      </div>

      {isLoading ? (
        <p className="mt-4 text-sm text-slate-500">Loading diff samples…</p>
      ) : (
        <div
          className="cr-chart-focusable mt-4 rounded-md"
          tabIndex={0}
          aria-label="Risk trend area chart showing new and resolved findings over time"
        >
          <AreaChart
            className="h-64"
            data={chartData}
            index="label"
            categories={["New", "Resolved"]}
            colors={[EXEC_CHART_COLORS.trend.newFindings, EXEC_CHART_COLORS.trend.resolvedFindings]}
            showAnimation
            yAxisWidth={48}
            valueFormatter={(value) => formatCount(value)}
          />
        </div>
      )}

      <div className="mt-3 flex flex-wrap gap-4 text-[11px] text-slate-600 dark:text-slate-400">
        <span>Transitions shown: {points.length}</span>
        <span>Latest net: {formatCount(points[0]?.net ?? 0)}</span>
      </div>
    </div>
  );
}
