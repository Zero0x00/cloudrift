import { Link } from "react-router-dom";
import { formatQueryError } from "../../api/httpError";
import { useScanRunHistoryQuery, useScanRunStatusQuery } from "../../hooks/useDashboardQueries";
import { IconScan } from "../../lib/icons";

function toShortTs(ts?: string): string {
  if (!ts) {
    return "—";
  }
  const d = new Date(ts);
  if (Number.isNaN(d.getTime())) {
    return "—";
  }
  return d.toISOString().replace("T", " ").replace("Z", "Z");
}

export function OperationsScanSummaryCard({ scanId }: { scanId: string | null }) {
  const status = useScanRunStatusQuery();
  const history = useScanRunHistoryQuery();
  const historyItems = Array.isArray(history.data?.items) ? history.data.items : [];

  const scanHref = scanId ? `/scan-control?scan_id=${encodeURIComponent(scanId)}` : "/scan-control";

  return (
    <div className="hs-card p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex items-center gap-2">
          <IconScan className="text-slate-500 dark:text-slate-400" />
          <div>
            <h3 className="cr-section-title">Scan runs</h3>
            <p className="cr-helper mt-0.5">Live status and recent activity from the control center.</p>
          </div>
        </div>
        <Link
          to={scanHref}
          className="cr-chip inline-flex items-center rounded-md border border-cyan-700/50 bg-cyan-900/15 px-3 py-1.5 text-cyan-900 dark:border-cyan-600/50 dark:bg-cyan-950/40 dark:text-cyan-100"
        >
          Open Scan Control
        </Link>
      </div>

      <dl className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <div>
          <dt className="cr-kpi-label">Run state</dt>
          <dd className="cr-body mt-0.5 capitalize text-slate-800 dark:text-slate-200">
            {status.isLoading ? "…" : status.data?.status ?? "—"}
          </dd>
        </div>
        <div>
          <dt className="cr-kpi-label">Stage</dt>
          <dd className="cr-body mt-0.5 text-slate-800 dark:text-slate-200">
            {status.isLoading ? "…" : status.data?.stage ?? "—"}
          </dd>
        </div>
        <div>
          <dt className="cr-kpi-label">Active scan ID</dt>
          <dd className="cr-mono mt-0.5 break-all text-xs text-slate-800 dark:text-slate-200">
            {status.data?.scan_id || "—"}
          </dd>
        </div>
        <div>
          <dt className="cr-kpi-label">Message</dt>
          <dd className="cr-helper mt-0.5 line-clamp-2">{status.data?.message || "—"}</dd>
        </div>
      </dl>

      {history.isLoading ? (
        <p className="cr-helper mt-4">Loading recent runs…</p>
      ) : history.isError ? (
        <p className="mt-4 text-sm text-rose-600 dark:text-rose-400">{formatQueryError(history.error)}</p>
      ) : historyItems.length === 0 ? (
        <p className="cr-helper mt-4">No run history yet. Start a scan from Scan Control.</p>
      ) : (
      <div className="hs-table-wrap mt-4 rounded-md">
          <table className="cr-table w-full min-w-[36rem] border-collapse text-left">
            <thead className="border-b border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900/90">
              <tr className="cr-kpi-label">
                <th className="px-2 py-2">Run</th>
                <th className="px-2 py-2">Status</th>
                <th className="px-2 py-2">Module</th>
                <th className="px-2 py-2">Started</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
              {historyItems.slice(0, 5).map((item) => (
                <tr key={item.run_id}>
                  <td className="cr-mono px-2 py-2 text-xs">{item.run_id}</td>
                  <td className="px-2 py-2">
                    <span className="cr-chip capitalize">{item.status}</span>
                  </td>
                  <td className="px-2 py-2 text-sm">{item.module || "all"}</td>
                  <td className="cr-mono px-2 py-2 text-xs">{toShortTs(item.started_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
