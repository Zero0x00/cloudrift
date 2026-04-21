import { useRemediationGroupsQuery } from "../../hooks/useDashboardQueries";
import { formatCount, formatUsd } from "../../lib/format";

type GroupKey = "reclaimable" | "stale_external_trust" | "dangling_edge" | "admin_like_external" | "broken_edge";

export type RemediationGroup = {
  key: GroupKey;
  label: string;
  why: string;
  findingCount: number;
  totalMonthlyRiskUsd: number;
  topExample?: string;
};

export function sortRemediationGroups(groups: RemediationGroup[]): RemediationGroup[] {
  return [...groups].sort((a, b) => {
    if (b.totalMonthlyRiskUsd !== a.totalMonthlyRiskUsd) {
      return b.totalMonthlyRiskUsd - a.totalMonthlyRiskUsd;
    }
    if (b.findingCount !== a.findingCount) {
      return b.findingCount - a.findingCount;
    }
    return a.label.localeCompare(b.label);
  });
}

export function RemediationGroupingPanel({
  scanId,
  onDrilldown
}: {
  scanId: string;
  onDrilldown: (group: GroupKey) => void;
}) {
  const groupsQuery = useRemediationGroupsQuery(scanId);
  const groups = sortRemediationGroups(
    (groupsQuery.data?.items ?? []).map((item) => ({
      key: item.key,
      label: item.label,
      why: item.why,
      findingCount: item.finding_count,
      totalMonthlyRiskUsd: item.total_monthly_risk_cost_usd,
      topExample: item.top_example
    }))
  ).slice(0, 8);

  return (
    <div className="hs-card p-5">
      <h3 className="cr-section-title">Remediation grouping</h3>
      <p className="cr-helper mt-1">Prioritize remediation patterns that remove the most recurring risk.</p>

      {groupsQuery.isLoading ? (
        <p className="mt-4 text-sm text-slate-500">Loading remediation groups…</p>
      ) : groupsQuery.isError ? (
        <p className="mt-4 text-sm text-rose-600 dark:text-rose-400">Unable to load remediation grouping right now.</p>
      ) : groups.length === 0 ? (
        <p className="mt-4 text-sm text-slate-500">No high-value remediation groups detected for this scan.</p>
      ) : (
        <ul className="mt-4 space-y-3">
          {groups.map((group) => (
            <li key={group.key}>
              <button
                type="button"
                onClick={() => onDrilldown(group.key)}
                className="hs-card-soft w-full rounded-lg px-4 py-3 text-left transition hover:border-cyan-300 hover:bg-cyan-50/40 dark:hover:border-cyan-700 dark:hover:bg-cyan-950/20"
              >
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <p className="text-sm font-semibold text-slate-800 dark:text-slate-100">{group.label}</p>
                  <div className="flex items-center gap-2">
                    <span className="hs-chip-compact border-slate-300 bg-slate-100 text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300">
                      {formatCount(group.findingCount)}
                    </span>
                    <span className="text-sm font-semibold tabular-nums text-slate-900 dark:text-slate-100">
                      {formatUsd(group.totalMonthlyRiskUsd)}
                    </span>
                  </div>
                </div>
                <p className="cr-helper mt-1">{group.why}</p>
                {group.topExample ? <p className="cr-helper mt-1 truncate">Top issue: {group.topExample}</p> : null}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

