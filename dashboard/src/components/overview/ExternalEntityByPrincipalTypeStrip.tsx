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
  const rows = [...(summary.external_entity_by_principal_type ?? [])].sort((a, b) => b.entity_count - a.entity_count);
  if (rows.length === 0) {
    return null;
  }
  const max = Math.max(...rows.map((r) => r.entity_count), 1);

  return (
    <div className="hs-card p-4">
      <h3 className="cr-section-title">Entities by principal type</h3>
      <p className="cr-helper mt-0.5">Distinct external entities by evidence principal type.</p>
      <ul className="mt-3 space-y-2">
        {rows.map((r) => {
          const width = Math.max(8, Math.round((r.entity_count / max) * 100));
          return (
            <li key={r.principal_type}>
              <button
                type="button"
                onClick={() => onOpenEntityListForPrincipalType(r.principal_type)}
                className="w-full rounded-md px-2 py-1.5 text-left transition hover:bg-slate-100 dark:hover:bg-slate-800/70"
              >
                <div className="mb-1 flex items-center justify-between gap-2 text-xs">
                  <span className="font-mono text-slate-700 dark:text-slate-200">{r.principal_type}</span>
                  <span className="tabular-nums text-slate-600 dark:text-slate-400">{formatCount(r.entity_count)}</span>
                </div>
                <div className="h-2 w-full overflow-hidden rounded bg-slate-200 dark:bg-slate-800">
                  <div className="h-full rounded bg-cyan-600/85" style={{ width: `${width}%` }} />
                </div>
              </button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
