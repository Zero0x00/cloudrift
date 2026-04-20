import type { ScanSummaryResponse } from "../../api/types";
import { formatCount, formatUsd } from "../../lib/format";
import { severityBarClass } from "../SeverityBadge";

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
  const total = summary.finding_count;
  const rows: { key: string; label: string; count: number }[] = [
    { key: "critical", label: "Critical", count: summary.critical_count },
    { key: "high", label: "High", count: summary.high_count },
    { key: "medium", label: "Medium", count: summary.medium_count },
    { key: "low", label: "Low / info", count: summary.low_count }
  ];

  const maxRow = Math.max(...rows.map((r) => r.count), 1);

  return (
    <div className="rounded-lg border border-slate-200 bg-white/80 p-5 dark:border-slate-800 dark:bg-slate-900/80">
      <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-400">Risk distribution</h3>
      <p className="mt-1 text-[11px] text-slate-500">
        Severity counts from summary only — percentages are shares of total findings.
      </p>
      {total === 0 ? (
        <p className="mt-3 text-sm text-slate-500">No findings in this scan.</p>
      ) : (
        <>
          <div className="mt-4 flex h-3 w-full overflow-hidden rounded bg-slate-800">
            {rows.map(({ key, count }) =>
              count > 0 ? (
                <div
                  key={key}
                  className={`${severityBarClass(key)} h-full min-w-[2px] transition-[flex-grow] ${onSeverityClick ? "cursor-pointer" : ""}`}
                  style={{ flexGrow: count, flexBasis: 0 }}
                  title={`${key}: ${count}`}
                  onClick={onSeverityClick ? () => onSeverityClick(key as "critical" | "high" | "medium" | "low") : undefined}
                />
              ) : null
            )}
          </div>
          <div className="mt-4 flex items-end justify-between gap-2 border-b border-slate-200 pb-1 dark:border-b-slate-800/80">
            {rows.map(({ key, label, count }) => {
              const barPx =
                count === 0 ? 0 : Math.max(4, Math.round((count / maxRow) * 56));
              return (
                <div key={key} className="flex min-w-0 flex-1 flex-col items-center gap-1">
                  <div className="flex h-14 w-full max-w-[2.75rem] items-end justify-center">
                    <div
                      className={`w-full rounded-t ${severityBarClass(key)} ${onSeverityClick ? "cursor-pointer" : ""}`}
                      style={{ height: barPx }}
                      title={`${label}: ${count} (${pct(count, total)}%)`}
                      onClick={onSeverityClick ? () => onSeverityClick(key as "critical" | "high" | "medium" | "low") : undefined}
                    />
                  </div>
                  <span className="text-[10px] font-medium uppercase tracking-wide text-slate-500">
                    {key === "low" ? "Low" : key.slice(0, 3)}
                  </span>
                </div>
              );
            })}
          </div>
          <ul className="mt-4 space-y-2 text-sm">
            {rows.map(({ key, label, count }) => (
              <li
                key={key}
                className={`flex items-center justify-between gap-4 text-slate-700 dark:text-slate-300 ${
                  onSeverityClick ? "cursor-pointer rounded px-1 py-1 hover:bg-slate-100 dark:hover:bg-slate-800/70" : ""
                }`}
                onClick={onSeverityClick ? () => onSeverityClick(key as "critical" | "high" | "medium" | "low") : undefined}
              >
                <span className="flex items-center gap-2">
                  <span className={`h-2 w-2 shrink-0 rounded-sm ${severityBarClass(key)}`} />
                  {label}
                </span>
                <span className="tabular-nums text-slate-600 dark:text-slate-400">
                  {formatCount(count)} ({pct(count, total)}%)
                </span>
              </li>
            ))}
          </ul>
        </>
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
    <div className="rounded-lg border border-slate-200 bg-white/80 p-5 dark:border-slate-800 dark:bg-slate-900/80">
      <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-400">Findings by module</h3>
      <p className="mt-1 text-[11px] text-slate-500">
        Finding counts by module. Monthly cost is not broken out per module in the summary API — see “Monthly
        cost split” for scan-level direct vs risk USD.
      </p>
      {total === 0 ? (
        <p className="mt-3 text-sm text-slate-500">No orphaned-edge or external-access findings in summary.</p>
      ) : (
        <>
          <div className="mt-4 flex h-3 w-full overflow-hidden rounded bg-slate-800">
            {a > 0 ? (
              <div
                className={`h-full min-w-[2px] bg-cyan-600 ${onModuleClick ? "cursor-pointer" : ""}`}
                style={{ flexGrow: a, flexBasis: 0 }}
                title={`orphaned_edge: ${a}`}
                onClick={onModuleClick ? () => onModuleClick("orphaned_edge") : undefined}
              />
            ) : null}
            {b > 0 ? (
              <div
                className={`h-full min-w-[2px] bg-violet-600 ${onModuleClick ? "cursor-pointer" : ""}`}
                style={{ flexGrow: b, flexBasis: 0 }}
                title={`external_access: ${b}`}
                onClick={onModuleClick ? () => onModuleClick("external_access") : undefined}
              />
            ) : null}
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
        </>
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
    <div className="rounded-lg border border-slate-200 bg-white/80 p-5 dark:border-slate-800 dark:bg-slate-900/80">
      <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-400">Claimability breakdown</h3>
      <p className="mt-1 text-[11px] text-slate-500">Reclaimable, dangling, broken, and edge_obscured counts from summary.</p>
      {total === 0 ? (
        <p className="mt-3 text-sm text-slate-500">No claimability-tagged findings in summary.</p>
      ) : (
        <>
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
        </>
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
    <div className="rounded-lg border border-slate-200 bg-white/80 p-5 dark:border-slate-800 dark:bg-slate-900/80">
      <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-400">Monthly cost split</h3>
      <p className="mt-1 text-[11px] text-slate-500">
        total_monthly_direct_cost_usd vs total_monthly_risk_cost_usd from summary. Represents waste exposure at scan
        level, not per-account or per-service.
      </p>
      {totalUsd <= 0 ? (
        <p className="mt-3 text-sm text-slate-500">No monthly cost totals in summary.</p>
      ) : (
        <>
          <div className="mt-4 flex h-4 w-full overflow-hidden rounded bg-slate-800">
            {direct > 0 ? (
              <div
                className="h-full min-w-[2px] bg-teal-600"
                style={{ flexGrow: direct, flexBasis: 0 }}
                title={`direct: ${direct}`}
              />
            ) : null}
            {risk > 0 ? (
              <div
                className="h-full min-w-[2px] bg-rose-600/90"
                style={{ flexGrow: risk, flexBasis: 0 }}
                title={`risk: ${risk}`}
              />
            ) : null}
          </div>
          <ul className="mt-4 space-y-2 text-sm text-slate-700 dark:text-slate-300">
            <li className="flex justify-between gap-4">
              <span className="flex items-center gap-2">
                <span className="h-2 w-2 shrink-0 rounded-sm bg-teal-600" />
                Direct / mo
              </span>
              <span className="tabular-nums text-slate-600 dark:text-slate-400">
                {formatUsd(direct)} ({pct(direct, totalUsd)}%)
              </span>
            </li>
            <li className="flex justify-between gap-4">
              <span className="flex items-center gap-2">
                <span className="h-2 w-2 shrink-0 rounded-sm bg-rose-600/90" />
                Risk / mo
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
