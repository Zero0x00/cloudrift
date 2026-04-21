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
  const latest = historyItems[0];
  const isActive = Boolean(status.data?.scan_id && status.data?.status && status.data.status !== "idle");

  const scanHref = scanId ? `/scan-control?scan_id=${encodeURIComponent(scanId)}` : "/scan-control";

  return (
    <div className="hs-card-soft border-slate-300 p-3.5 dark:border-slate-700">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex items-center gap-2">
          <IconScan className="text-slate-500 dark:text-slate-400" />
          <div>
            <h3 className="cr-section-title">Operational status</h3>
            <p className="cr-helper mt-0.5">
              {status.isLoading
                ? "Checking scanner status…"
                : isActive
                  ? `Active scan: ${status.data?.status ?? "running"}`
                  : "No active scan"}
            </p>
          </div>
        </div>
        <Link
          to={scanHref}
          className="cr-chip inline-flex items-center rounded-md border border-cyan-700/50 bg-cyan-900/15 px-3 py-1.5 text-cyan-900 dark:border-cyan-600/50 dark:bg-cyan-950/40 dark:text-cyan-100"
        >
          {isActive ? "Open Scan Control" : "Start Scan"}
        </Link>
      </div>

      <dl className="mt-2.5 grid gap-2.5 sm:grid-cols-2 lg:grid-cols-4">
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
          <dt className="cr-kpi-label">Last run</dt>
          <dd className="cr-helper mt-0.5 line-clamp-2">
            {history.isLoading
              ? "Loading…"
              : latest
                ? `${latest.status} • ${toShortTs(latest.started_at)}`
                : "No recent runs"}
          </dd>
        </div>
      </dl>
      {history.isError ? <p className="mt-3 text-sm text-rose-600 dark:text-rose-400">{formatQueryError(history.error)}</p> : null}
    </div>
  );
}
