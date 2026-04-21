import type { TopFixItem } from "../../api/types";
import { SeverityBadge } from "../SeverityBadge";
import { StatePanel } from "../StatePanel";
import { formatQueryError } from "../../api/httpError";
import { formatUsd } from "../../lib/format";
import { useTopFixesQuery } from "../../hooks/useDashboardQueries";

type TopFixesPanelProps = {
  scanId: string;
  /** Max rows to request from the API (default 12 for dashboard density). */
  limit?: number;
  onDrilldown: (item: TopFixItem) => void;
};

function ownerLine(item: TopFixItem): string {
  const team = item.team?.trim();
  if (team) {
    return team;
  }
  const name = item.account_name?.trim();
  if (name) {
    return `${name} (${item.account_id || "—"})`;
  }
  return item.account_id?.trim() || "—";
}

export function TopFixesPanel({ scanId, limit = 12, onDrilldown }: TopFixesPanelProps) {
  const q = useTopFixesQuery(scanId, { limit });

  return (
    <div className="hs-card p-5">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <h3 className="cr-section-title">Prioritized risk queue</h3>
          <p className="cr-helper mt-1">
            Server-ranked fixes from severity, claimability, modeled monthly risk, and external-trust signals. Click a row
            to open Findings with this item targeted.
          </p>
        </div>
      </div>

      {q.isLoading ? (
        <div className="mt-4">
          <StatePanel>Loading top fixes…</StatePanel>
        </div>
      ) : q.isError ? (
        <div className="mt-4">
          <StatePanel intent="error" title="Could not load top fixes">
            <pre className="whitespace-pre-wrap font-sans text-xs">{formatQueryError(q.error)}</pre>
          </StatePanel>
        </div>
      ) : !q.data?.items?.length ? (
        <p className="cr-helper mt-4">No findings for this scan.</p>
      ) : (
        <ul className="mt-4 divide-y divide-slate-200 dark:divide-slate-700">
          {q.data.items.map((item) => (
            <li key={item.id}>
              <button
                type="button"
                onClick={() => onDrilldown(item)}
                className="flex w-full flex-col gap-1 rounded-md py-3 text-left transition hover:bg-slate-50/90 dark:hover:bg-slate-800/50"
              >
                <div className="flex flex-wrap items-center gap-2">
                  <SeverityBadge severity={item.severity} />
                  <span className="text-sm font-medium text-slate-900 dark:text-slate-100">{item.title}</span>
                  <span className="cr-chip-compact font-mono text-[10px] text-slate-500" title="Composite priority score">
                    {item.priority_score.toFixed(1)} pts
                  </span>
                </div>
                <div className="flex flex-wrap gap-x-4 gap-y-0.5 text-xs text-slate-600 dark:text-slate-400">
                  <span>Risk {formatUsd(item.monthly_risk_cost_usd)}/mo</span>
                  <span>Owner {ownerLine(item)}</span>
                </div>
                <p className="text-xs text-slate-500 dark:text-slate-500">{item.reason}</p>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
