import type { ExternalEntityRow, ScanSummaryResponse } from "../../api/types";
import { SeverityBadge } from "../SeverityBadge";
import { formatCount, formatUsd, shortenArn } from "../../lib/format";

type Props = {
  summary: ScanSummaryResponse;
  onOpenEntityFindings: (row: ExternalEntityRow) => void;
  onOpenExternalEntitiesPage: () => void;
};

export function TopRiskyExternalEntitiesPreview({
  summary,
  onOpenEntityFindings,
  onOpenExternalEntitiesPage
}: Props) {
  const rows = summary.external_entities_preview ?? [];
  if (rows.length === 0) {
    return null;
  }

  return (
    <div className="rounded-lg border border-slate-200 bg-white/90 p-4 dark:border-slate-800 dark:bg-slate-900/80">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-400">
            Highest-severity external entities
          </h3>
          <p
            className="mt-0.5 text-[11px] text-slate-500"
            title="Sort is severity first (not a pure risk-cost ranking). Ties on severity are broken by total monthly risk, then by a stable key. Same order as the External Entities list without filters."
          >
            Preview of up to 5 entities. Ordered by <strong>highest severity first</strong>, then by total monthly risk
            cost within the same severity. Not a pure cost ranking.
          </p>
        </div>
        <button
          type="button"
          onClick={onOpenExternalEntitiesPage}
          className="rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 transition hover:bg-slate-50 dark:border-slate-600 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800"
        >
          Open all entities
        </button>
      </div>
      <div className="mt-3 overflow-x-auto">
        <table className="w-full min-w-[640px] border-collapse text-left text-sm">
          <thead>
            <tr className="border-b border-slate-200 text-[11px] uppercase tracking-wide text-slate-500 dark:border-slate-700 dark:text-slate-400">
              <th
                className="py-2 pr-3 font-medium"
                title="From evidence.external_principal. Empty evidence renders as unknown and may merge multiple distinct-but-unidentified principals into the same row."
              >
                External principal
              </th>
              <th
                className="py-2 pr-3 font-medium"
                title="From evidence.principal_type (lower-cased). Empty evidence renders as unknown."
              >
                Type
              </th>
              <th
                className="py-2 pr-3 font-medium"
                title="From evidence.external_account_id. Empty evidence renders as unknown."
              >
                Ext. account
              </th>
              <th className="py-2 pr-3 font-medium">Severity</th>
              <th className="py-2 pr-3 font-medium">Risk / mo</th>
              <th
                className="py-2 pr-2 text-center font-medium"
                title="Count of DISTINCT trusted roles in this entity with verdict stale_review_now. Does not mean all roles for this entity are stale — only at least this many."
              >
                Stale roles
              </th>
              <th
                className="py-2 pr-2 text-center font-medium"
                title="Count of DISTINCT trusted roles classified as privileged tier (permission_visibility.classification). At least one role hits this signal; other roles for the same entity may not."
              >
                Priv. roles
              </th>
              <th
                className="py-2 pr-2 text-center font-medium"
                title="Count of DISTINCT trusted roles with permission_visibility.capabilities.admin_like. At least one role hits this signal; other roles for the same entity may not."
              >
                Admin∼ roles
              </th>
              <th className="py-2 pr-2 text-center font-medium">Findings</th>
              <th className="py-2 font-medium">Drilldown</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr
                key={`${row.external_principal}|${row.principal_type}|${row.external_account_id}`}
                className="border-b border-slate-100 dark:border-slate-800/80"
              >
                <td className="py-2 pr-3 font-mono text-xs text-slate-800 dark:text-slate-200">
                  {shortenArn(row.external_principal, 42)}
                </td>
                <td className="py-2 pr-3 text-xs text-slate-700 dark:text-slate-300">{row.principal_type}</td>
                <td className="py-2 pr-3 font-mono text-xs text-slate-600 dark:text-slate-400">
                  {row.external_account_id}
                </td>
                <td className="py-2 pr-3">
                  <SeverityBadge severity={row.highest_severity} />
                </td>
                <td className="py-2 pr-3 tabular-nums text-xs text-slate-700 dark:text-slate-300">
                  {formatUsd(row.total_monthly_risk_cost_usd)}
                </td>
                <td className="py-2 pr-2 text-center tabular-nums text-xs">{formatCount(row.stale_role_count)}</td>
                <td className="py-2 pr-2 text-center tabular-nums text-xs">
                  {formatCount(row.privileged_role_count)}
                </td>
                <td className="py-2 pr-2 text-center tabular-nums text-xs">{formatCount(row.admin_like_role_count)}</td>
                <td className="py-2 pr-2 text-center tabular-nums text-xs">
                  {formatCount(row.external_access_finding_count)}
                </td>
                <td className="py-2">
                  <button
                    type="button"
                    onClick={() => onOpenEntityFindings(row)}
                    className="text-xs font-medium text-cyan-700 underline-offset-2 hover:underline dark:text-cyan-400"
                  >
                    Findings
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
