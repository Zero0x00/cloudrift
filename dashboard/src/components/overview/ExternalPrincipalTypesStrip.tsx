import type { ExternalPrincipalTypeCount, ScanSummaryResponse } from "../../api/types";
import { formatCount } from "../../lib/format";

function labelForPrincipalType(t: string): string {
  if (t === "unknown") {
    return "Unknown / unset";
  }
  return t.replace(/_/g, " ");
}

export function ExternalPrincipalTypesStrip({
  summary,
  onOpenPrincipalType
}: {
  summary: ScanSummaryResponse;
  onOpenPrincipalType: (principalType: string) => void;
}) {
  const rows = summary.external_principal_types ?? [];
  if (rows.length === 0) {
    return null;
  }

  return (
    <div className="rounded-lg border border-slate-200 bg-white/90 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900/80">
      <h2 className="text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
        External principal types
      </h2>
      <p className="mt-1 max-w-3xl text-[11px] text-slate-500">
        Counts from scan summary (<code className="rounded bg-slate-100 px-1 dark:bg-slate-800">evidence.principal_type</code> on{" "}
        <code className="rounded bg-slate-100 px-1 dark:bg-slate-800">external_access</code> findings). Click a row to open Findings with structured{" "}
        <code className="rounded bg-slate-100 px-1 dark:bg-slate-800">principal_type</code> filter.
      </p>
      <ul className="mt-4 divide-y divide-slate-200 dark:divide-slate-800">
        {rows.map((row: ExternalPrincipalTypeCount) => (
          <li key={row.principal_type} className="flex items-center justify-between gap-3 py-2 first:pt-0">
            <button
              type="button"
              onClick={() => onOpenPrincipalType(row.principal_type)}
              className="min-w-0 flex-1 rounded-md px-1 py-0.5 text-left text-sm font-medium text-cyan-800 underline-offset-2 hover:underline dark:text-cyan-200/90"
            >
              {labelForPrincipalType(row.principal_type)}
            </button>
            <span className="shrink-0 tabular-nums text-sm font-semibold text-slate-900 dark:text-slate-100">
              {formatCount(row.count)}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}
