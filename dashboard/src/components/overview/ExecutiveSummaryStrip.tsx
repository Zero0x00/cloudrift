import type { ScanSummaryResponse } from "../../api/types";
import { formatCount, formatUsd } from "../../lib/format";

const emphasisBorder: Record<string, string> = {
  rose: "border-l-rose-500/80",
  violet: "border-l-violet-500/70",
  emerald: "border-l-emerald-500/70",
  neutral: "border-l-slate-300 dark:border-l-slate-600"
};

function ExecCard({
  label,
  value,
  emphasis = "neutral",
  onClick
}: {
  label: string;
  value: string;
  emphasis?: keyof typeof emphasisBorder;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`w-full rounded-lg border border-slate-200 border-l-4 bg-white py-4 pl-4 pr-3 text-left shadow-sm shadow-slate-200/60 transition hover:-translate-y-0.5 hover:shadow-md hover:shadow-slate-300/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-500/60 dark:border-slate-800 dark:bg-slate-900/90 dark:shadow-black/10 dark:hover:shadow-black/30 ${emphasisBorder[emphasis]}`}
    >
      <p className="text-[11px] font-medium uppercase tracking-wide text-slate-500">{label}</p>
      <p className="mt-2 text-2xl font-semibold tabular-nums tracking-tight text-slate-900 dark:text-slate-100">{value}</p>
    </button>
  );
}

export function ExecutiveSummaryStrip({
  summary,
  onOpenCritical,
  onOpenRiskCost,
  onOpenExternal,
  onOpenReclaimable
}: {
  summary: ScanSummaryResponse;
  onOpenCritical: () => void;
  onOpenRiskCost: () => void;
  onOpenExternal: () => void;
  onOpenReclaimable: () => void;
}) {
  return (
    <div>
      <h2 className="mb-3 text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
        Executive summary
      </h2>
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <ExecCard label="Critical findings" value={formatCount(summary.critical_count)} emphasis="rose" onClick={onOpenCritical} />
        <ExecCard
          label="Total risk cost / mo"
          value={formatUsd(summary.total_monthly_risk_cost_usd)}
          emphasis="neutral"
          onClick={onOpenRiskCost}
        />
        <ExecCard
          label="External access"
          value={formatCount(summary.external_access_count)}
          emphasis="violet"
          onClick={onOpenExternal}
        />
        <ExecCard
          label="Reclaimable assets"
          value={formatCount(summary.reclaimable_count)}
          emphasis="emerald"
          onClick={onOpenReclaimable}
        />
      </div>
    </div>
  );
}
