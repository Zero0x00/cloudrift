import type { ScanSummaryResponse } from "../../api/types";
import { formatCount } from "../../lib/format";

type Props = {
  summary: ScanSummaryResponse;
  onOpenEntityListForPrincipalType: (principalType: string) => void;
};

/**
 * Entity counts by principal_type (distinct external entities), unlike `external_principal_types` which counts findings.
 */
export function ExternalEntityByPrincipalTypeStrip({ summary, onOpenEntityListForPrincipalType }: Props) {
  const rows = summary.external_entity_by_principal_type ?? [];
  if (rows.length === 0) {
    return null;
  }

  return (
    <div className="rounded-lg border border-slate-200 bg-white/90 p-4 dark:border-slate-800 dark:bg-slate-900/80">
      <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-400">
        Entities by principal type
      </h3>
      <p
        className="mt-0.5 text-[11px] text-slate-500 dark:text-slate-500"
        title="Principal type comes from evidence.principal_type (lower-cased). Missing evidence is bucketed as 'unknown' and may include multiple distinct principals that could not be classified."
      >
        Each chip is a distinct-entity count (not finding count). Entries labeled{" "}
        <span className="font-mono">unknown</span> had no <span className="font-mono">principal_type</span> evidence.
        Opens the External Entities list filtered to that type.
      </p>
      <div className="mt-3 flex flex-wrap gap-2">
        {rows.map((r) => (
          <button
            key={r.principal_type}
            type="button"
            onClick={() => onOpenEntityListForPrincipalType(r.principal_type)}
            className="inline-flex items-center gap-2 rounded-full border border-slate-300 bg-slate-50 px-3 py-1.5 text-xs font-medium text-slate-800 transition hover:border-cyan-500/50 hover:bg-cyan-50/60 dark:border-slate-600 dark:bg-slate-900 dark:text-slate-200 dark:hover:border-cyan-500/40 dark:hover:bg-cyan-950/30"
          >
            <span className="font-mono text-[11px]">{r.principal_type}</span>
            <span className="tabular-nums text-slate-600 dark:text-slate-400">{formatCount(r.entity_count)}</span>
          </button>
        ))}
      </div>
    </div>
  );
}
