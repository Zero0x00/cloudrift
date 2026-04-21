import type { ScanSummaryResponse } from "../../api/types";
import { BarChart, DonutChart } from "@tremor/react";
import { formatCount, formatUsd } from "../../lib/format";
import { EXEC_CHART_COLORS } from "./chartColors";

export type SummaryDrilldownHandlers = {
  onSeverityClick?: (severity: "critical" | "high" | "medium" | "low") => void;
  onModuleClick?: (module: "orphaned_edge" | "external_access") => void;
  onClaimabilityClick?: (claimability: "reclaimable" | "dangling" | "broken" | "edge_obscured") => void;
};

function pct(part: number, total: number): number {
  if (total <= 0) {
    return 0;
  }
  return Math.round((100 * part) / total);
}

/**
 * Horizontal stacked bar: shares of findings by severity (API counts only).
 *
 * low_count semantics: the API increments `low_count` for every finding whose severity is
 * not critical, high, or medium (see summarizeScan default branch). That single bucket can
 * therefore mix explicit "low" severities with any other non-medium value the model allows
 * (e.g. info-style labels). UI label "Low / info" reflects that aggregation — not a
 * separate info counter from the backend.
 */
export function SeverityDistribution({
  summary,
  onSeverityClick
}: {
  summary: ScanSummaryResponse;
  onSeverityClick?: SummaryDrilldownHandlers["onSeverityClick"];
}) {
  const rows: { key: "critical" | "high" | "medium" | "low"; label: string; count: number; color: string }[] = [
    { key: "critical", label: "Critical", count: summary.critical_count, color: EXEC_CHART_COLORS.severity.critical },
    { key: "high", label: "High", count: summary.high_count, color: EXEC_CHART_COLORS.severity.high },
    { key: "medium", label: "Medium", count: summary.medium_count, color: EXEC_CHART_COLORS.severity.medium },
    { key: "low", label: "Low / info", count: summary.low_count, color: EXEC_CHART_COLORS.severity.low }
  ];
  const chartData = rows.map((r) => ({
    severity: r.label,
    key: r.key,
    Critical: r.key === "critical" ? r.count : 0,
    High: r.key === "high" ? r.count : 0,
    Medium: r.key === "medium" ? r.count : 0,
    "Low / info": r.key === "low" ? r.count : 0
  }));

  return (
    <div className="hs-card p-5">
      <h3 className="cr-section-title">Risk distribution</h3>
      <p className="cr-helper mt-1">Current severity composition by finding count.</p>
      {summary.finding_count === 0 ? (
        <p className="mt-3 text-sm text-slate-500">No findings in this scan.</p>
      ) : (
        <div className="cr-chart-focusable mt-4 rounded-md" tabIndex={0} aria-label="Risk distribution bar chart by severity">
          <BarChart
            className="h-64"
            data={chartData}
            index="severity"
            categories={["Critical", "High", "Medium", "Low / info"]}
            layout="vertical"
            colors={[
              EXEC_CHART_COLORS.severity.critical,
              EXEC_CHART_COLORS.severity.high,
              EXEC_CHART_COLORS.severity.medium,
              EXEC_CHART_COLORS.severity.low
            ]}
            valueFormatter={(value) => formatCount(value)}
            yAxisWidth={88}
            onValueChange={(value) => {
              if (!onSeverityClick || !value) {
                return;
              }
              const row = chartData.find((item) => item.severity === String(value.severity));
              if (row) {
                onSeverityClick(row.key);
              }
            }}
          />
        </div>
      )}
    </div>
  );
}

/** Two-way split from summary: orphaned edge vs external access (counts — summary has no USD-by-module split). */
export function ModuleDistribution({
  summary,
  onModuleClick
}: {
  summary: ScanSummaryResponse;
  onModuleClick?: SummaryDrilldownHandlers["onModuleClick"];
}) {
  const a = summary.orphaned_edge_count;
  const b = summary.external_access_count;
  const total = a + b;

  return (
    <div className="hs-card p-5">
      <h3 className="cr-section-title">Findings by module</h3>
      <p className="cr-helper mt-1">Composition of orphaned edge vs external access findings.</p>
      {total === 0 ? (
        <p className="mt-3 text-sm text-slate-500">No orphaned-edge or external-access findings in summary.</p>
      ) : (
        <div
          className="cr-chart-focusable mt-4 rounded-md"
          tabIndex={0}
          aria-label="Findings by module donut composition chart"
        >
          <div className="mt-2 flex items-center justify-center">
            <DonutChart
              className="h-44"
              data={[
                { label: "Orphaned edge", value: a },
                { label: "External access", value: b }
              ]}
              category="value"
              index="label"
              colors={["cyan", "violet"]}
              valueFormatter={(value) => formatCount(value)}
              showAnimation
              onValueChange={(value) => {
                if (!onModuleClick || !value?.label) {
                  return;
                }
                if (value.label === "Orphaned edge") {
                  onModuleClick("orphaned_edge");
                  return;
                }
                onModuleClick("external_access");
              }}
            />
          </div>
          <ul className="mt-4 space-y-2 text-sm text-slate-700 dark:text-slate-300">
            <li
              className={`flex justify-between gap-4 ${
                onModuleClick ? "cursor-pointer rounded px-1 py-1 hover:bg-slate-100 dark:hover:bg-slate-800/70" : ""
              }`}
              onClick={onModuleClick ? () => onModuleClick("orphaned_edge") : undefined}
            >
              <span className="flex items-center gap-2">
                <span className="h-2 w-2 shrink-0 rounded-sm bg-cyan-600" />
                Orphaned edge
              </span>
              <span className="tabular-nums text-slate-600 dark:text-slate-400">
                {formatCount(a)} ({pct(a, total)}%)
              </span>
            </li>
            <li
              className={`flex justify-between gap-4 ${
                onModuleClick ? "cursor-pointer rounded px-1 py-1 hover:bg-slate-100 dark:hover:bg-slate-800/70" : ""
              }`}
              onClick={onModuleClick ? () => onModuleClick("external_access") : undefined}
            >
              <span className="flex items-center gap-2">
                <span className="h-2 w-2 shrink-0 rounded-sm bg-violet-600" />
                External access
              </span>
              <span className="tabular-nums text-slate-600 dark:text-slate-400">
                {formatCount(b)} ({pct(b, total)}%)
              </span>
            </li>
          </ul>
        </div>
      )}
    </div>
  );
}

/** Claimability buckets from summary (API fields only). */
export function ClaimabilityDistribution({
  summary,
  onClaimabilityClick
}: {
  summary: ScanSummaryResponse;
  onClaimabilityClick?: SummaryDrilldownHandlers["onClaimabilityClick"];
}) {
  const rows: { key: string; label: string; count: number; bar: string }[] = [
    { key: "reclaimable", label: "Reclaimable", count: summary.reclaimable_count, bar: "bg-emerald-600" },
    { key: "dangling", label: "Dangling", count: summary.dangling_count, bar: "bg-sky-600" },
    { key: "broken", label: "Broken", count: summary.broken_count, bar: "bg-fuchsia-600" },
    { key: "edge_obscured", label: "Edge obscured", count: summary.edge_obscured_count, bar: "bg-amber-600" }
  ];
  const total = rows.reduce((s, r) => s + r.count, 0);

  return (
    <div className="hs-card p-5">
      <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-400">Claimability breakdown</h3>
      <p className="mt-1 text-[11px] text-slate-500">Reclaimable, dangling, broken, and edge_obscured counts from summary.</p>
      {total === 0 ? (
        <p className="mt-3 text-sm text-slate-500">No claimability-tagged findings in summary.</p>
      ) : (
        <div
          className="cr-chart-focusable mt-4 rounded-md"
          tabIndex={0}
          aria-label="Claimability breakdown chart"
        >
          <div className="mt-4 flex h-3 w-full overflow-hidden rounded bg-slate-800">
            {rows.map(({ key, count, bar }) =>
              count > 0 ? (
                <div
                  key={key}
                  className={`${bar} h-full min-w-[2px] ${onClaimabilityClick ? "cursor-pointer" : ""}`}
                  style={{ flexGrow: count, flexBasis: 0 }}
                  title={`${key}: ${count}`}
                  onClick={
                    onClaimabilityClick
                      ? () => onClaimabilityClick(key as "reclaimable" | "dangling" | "broken" | "edge_obscured")
                      : undefined
                  }
                />
              ) : null
            )}
          </div>
          <ul className="mt-4 space-y-3">
          {rows.map(({ key, label, count, bar }) => (
            <li
              key={key}
              className={onClaimabilityClick ? "cursor-pointer rounded px-1 py-1 hover:bg-slate-100 dark:hover:bg-slate-800/70" : ""}
              onClick={
                onClaimabilityClick
                  ? () => onClaimabilityClick(key as "reclaimable" | "dangling" | "broken" | "edge_obscured")
                  : undefined
              }
            >
              <div className="mb-1 flex justify-between text-sm text-slate-700 dark:text-slate-300">
                <span>{label}</span>
                <span className="tabular-nums text-slate-600 dark:text-slate-400">
                  {formatCount(count)} ({pct(count, total)}%)
                </span>
              </div>
              <div className="h-1.5 w-full overflow-hidden rounded bg-slate-800">
                <div
                  className={`h-full rounded ${bar} ${onClaimabilityClick ? "cursor-pointer" : ""}`}
                  style={{ width: `${pct(count, total)}%` }}
                  onClick={
                    onClaimabilityClick
                      ? () => onClaimabilityClick(key as "reclaimable" | "dangling" | "broken" | "edge_obscured")
                      : undefined
                  }
                />
              </div>
            </li>
          ))}
        </ul>
        </div>
      )}
    </div>
  );
}

/** Scan-level monthly USD split: direct vs risk (summary totals only — not per service). */
export function DirectVsRiskCostSplit({ summary }: { summary: ScanSummaryResponse }) {
  const direct = summary.total_monthly_direct_cost_usd;
  const risk = summary.total_monthly_risk_cost_usd;
  const totalUsd = direct + risk;

  return (
    <div className="hs-card p-5">
      <h3 className="cr-section-title">Cost split</h3>
      <p className="cr-helper mt-1">Direct cost vs risk-adjusted cost (monthly).</p>
      {totalUsd <= 0 ? (
        <p className="mt-3 text-sm text-slate-500">No monthly cost totals in summary.</p>
      ) : (
        <>
          <div
            className="cr-chart-focusable mt-3 flex items-center justify-center rounded-md"
            tabIndex={0}
            aria-label="Cost split donut chart for direct and risk-adjusted monthly cost"
          >
            <DonutChart
              className="h-52"
              data={[
                { label: "Direct cost", value: Math.round(direct * 100) / 100 },
                { label: "Risk-adjusted cost", value: Math.round(risk * 100) / 100 }
              ]}
              category="value"
              index="label"
              colors={[EXEC_CHART_COLORS.costSplit.direct, EXEC_CHART_COLORS.costSplit.riskAdjusted]}
              valueFormatter={(value) => formatUsd(value)}
              showAnimation
            />
          </div>
          <ul className="mt-4 space-y-2 text-sm text-slate-700 dark:text-slate-300">
            <li className="flex justify-between gap-4">
              <span className="flex items-center gap-2">
                <span className="h-2 w-2 shrink-0 rounded-sm bg-emerald-600" />
                Direct / mo
              </span>
              <span className="tabular-nums text-slate-600 dark:text-slate-400">
                {formatUsd(direct)} ({pct(direct, totalUsd)}%)
              </span>
            </li>
            <li className="flex justify-between gap-4">
              <span className="flex items-center gap-2">
                <span className="h-2 w-2 shrink-0 rounded-sm bg-rose-600/90" />
                Risk-adjusted / mo
              </span>
              <span className="tabular-nums text-slate-600 dark:text-slate-400">
                {formatUsd(risk)} ({pct(risk, totalUsd)}%)
              </span>
            </li>
          </ul>
        </>
      )}
    </div>
  );
}
