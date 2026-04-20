import { Link } from "react-router-dom";
import type { ScanListItem } from "../../api/types";
import { useScanRiskTrendData } from "../../hooks/useDashboardQueries";
import { formatCount } from "../../lib/format";

function barHeightPx(value: number, max: number): number {
  if (max <= 0 || value <= 0) {
    return 0;
  }
  return Math.max(6, Math.round((value / max) * 72));
}

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

  const maxVal = Math.max(
    1,
    ...points.filter((p) => !p.isError).map((p) => Math.max(p.newCount, p.resolvedCount)),
    0
  );

  return (
    <div className="rounded-lg border border-slate-200 bg-white/80 p-5 dark:border-slate-800 dark:bg-slate-900/80">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <div>
          <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-400">Risk trend</h3>
          <p className="mt-1 max-w-xl text-[11px] text-slate-500">
            Each column is GET /api/diff between consecutive scans (newest-first list). Bars use exact{" "}
            <span className="font-mono text-[10px]">new_findings</span> and <span className="font-mono text-[10px]">resolved_findings</span>{" "}
            lengths — capped to {points.length} newest transitions.
          </p>
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
        <div className="mt-6 flex items-end justify-between gap-2 overflow-x-auto pb-1">
          {points.map((p, i) => (
            <div key={`${p.label}-${i}`} className="flex min-w-[3.25rem] flex-1 flex-col items-center gap-2">
              <div className="flex h-20 w-full max-w-[4rem] items-end justify-center gap-1">
                <div
                  className={`w-2.5 rounded-t ${p.isError ? "bg-slate-500/40" : "bg-amber-500/90 dark:bg-amber-400/85"}`}
                  style={{
                    height: p.isError ? 4 : barHeightPx(p.newCount, maxVal)
                  }}
                  title={p.isError ? "Diff request failed" : `New: ${p.newCount}`}
                />
                <div
                  className={`w-2.5 rounded-t ${p.isError ? "bg-slate-500/40" : "bg-emerald-600/90 dark:bg-emerald-500/85"}`}
                  style={{
                    height: p.isError ? 4 : barHeightPx(p.resolvedCount, maxVal)
                  }}
                  title={p.isError ? "Diff request failed" : `Resolved: ${p.resolvedCount}`}
                />
              </div>
              <span className="text-center text-[10px] font-medium uppercase tracking-wide text-slate-500">{p.label}</span>
              <span
                className={`text-[10px] tabular-nums ${
                  p.isError
                    ? "text-slate-500"
                    : p.net > 0
                      ? "text-amber-800 dark:text-amber-300/90"
                      : p.net < 0
                        ? "text-emerald-800 dark:text-emerald-300/90"
                        : "text-slate-500"
                }`}
                title="Net = new − resolved"
              >
                {p.isError ? "—" : `${p.net >= 0 ? "+" : ""}${formatCount(p.net)}`}
              </span>
            </div>
          ))}
        </div>
      )}

      <div className="mt-4 flex flex-wrap gap-4 text-[11px] text-slate-600 dark:text-slate-400">
        <span className="flex items-center gap-1.5">
          <span className="h-2 w-2 rounded-sm bg-amber-500/90" /> New findings
        </span>
        <span className="flex items-center gap-1.5">
          <span className="h-2 w-2 rounded-sm bg-emerald-600/90" /> Resolved findings
        </span>
        <span className="text-slate-500">Net change shown under each column</span>
      </div>
    </div>
  );
}
