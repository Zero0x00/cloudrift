import { formatCount } from "../../lib/format";
import { TRUST_ACTIVITY_SAMPLE_CAP, useTrustActivityBucketsQuery, type TrustActivityBuckets } from "../../hooks/useDashboardQueries";

function pct(part: number, total: number): number {
  if (total <= 0) {
    return 0;
  }
  return Math.round((100 * part) / total);
}

const ROWS: { key: keyof TrustActivityBuckets; label: string; bar: string }[] = [
  { key: "lt90", label: "< 90 days", bar: "bg-emerald-600/90" },
  { key: "d90_365", label: "90–365 days", bar: "bg-sky-600/90" },
  { key: "gt365", label: "> 365 days", bar: "bg-amber-600/90" },
  { key: "never", label: "Never / no last-used", bar: "bg-slate-600" }
];

export function TrustActivityAgingChart({
  scanId,
  pageFindingIds,
  pageSize,
  pageTotalItems
}: {
  scanId: string | null;
  pageFindingIds: string[];
  pageSize: number;
  pageTotalItems: number;
}) {
  const q = useTrustActivityBucketsQuery(scanId, pageFindingIds);

  if (!scanId) {
    return null;
  }

  if (pageFindingIds.length === 0) {
    return (
      <div className="rounded-lg border border-slate-200 bg-slate-50/95 p-4 dark:border-slate-800 dark:bg-slate-900/60">
        <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-400">Activity aging (sample)</h3>
        <p className="mt-2 text-sm text-slate-500">No rows on this page to sample.</p>
      </div>
    );
  }

  if (q.isLoading) {
    return (
      <div className="rounded-lg border border-slate-200 bg-slate-50/95 p-4 dark:border-slate-800 dark:bg-slate-900/60">
        <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-400">Activity aging (sample)</h3>
        <p className="mt-2 text-sm text-slate-500">Loading detail for sample…</p>
      </div>
    );
  }

  if (q.isError) {
    return (
      <div className="rounded-lg border border-slate-200 bg-slate-50/95 p-4 dark:border-slate-800 dark:bg-slate-900/60">
        <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-400">Activity aging (sample)</h3>
        <p className="mt-2 text-sm text-rose-400/90">Could not load finding detail for the activity chart.</p>
      </div>
    );
  }

  const buckets = q.data!.buckets;
  const totalInBuckets = ROWS.reduce((s, r) => s + buckets[r.key], 0);

  return (
    <div className="rounded-lg border border-slate-200 bg-slate-50/95 p-4 dark:border-slate-800 dark:bg-slate-900/60">
      <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-400">Activity aging (sample)</h3>
      <p className="mt-1 text-[11px] text-slate-500">
        Buckets use <span className="font-mono text-slate-600 dark:text-slate-400">trust.days_since_used</span> from finding detail (GET
        …/findings/:id). Up to {TRUST_ACTIVITY_SAMPLE_CAP} findings on this page are sampled — not the full scan.
        {pageTotalItems > TRUST_ACTIVITY_SAMPLE_CAP
          ? ` This page shows ${formatCount(pageSize)} rows; chart uses the first ${TRUST_ACTIVITY_SAMPLE_CAP} IDs only.`
          : null}{" "}
        Sampled detail is stored under the same query keys as row expansion, so expanding a row that was already
        sampled reuses cached data (no second HTTP request while fresh).
      </p>
      {q.data!.failed > 0 ? (
        <p className="mt-2 text-[11px] text-amber-500/90">
          {formatCount(q.data!.failed)} detail request(s) failed and were excluded.
        </p>
      ) : null}
      {totalInBuckets === 0 ? (
        <p className="mt-3 text-sm text-slate-500">No IAM last-used values classified into buckets for this sample.</p>
      ) : (
        <>
          <div className="mt-3 flex h-3 w-full overflow-hidden rounded bg-slate-800">
            {ROWS.map(({ key, bar }) => {
              const c = buckets[key];
              return c > 0 ? (
                <div
                  key={key}
                  className={`${bar} h-full min-w-[2px]`}
                  style={{ flexGrow: c, flexBasis: 0 }}
                  title={`${key}: ${c}`}
                />
              ) : null;
            })}
          </div>
          <ul className="mt-4 space-y-2 text-sm text-slate-700 dark:text-slate-300">
            {ROWS.map(({ key, label, bar }) => {
              const c = buckets[key];
              return (
                <li key={key} className="flex justify-between gap-4">
                  <span className="flex items-center gap-2">
                    <span className={`h-2 w-2 shrink-0 rounded-sm ${bar}`} />
                    {label}
                  </span>
                  <span className="tabular-nums text-slate-600 dark:text-slate-400">
                    {formatCount(c)} ({pct(c, totalInBuckets)}%)
                  </span>
                </li>
              );
            })}
          </ul>
          <p className="mt-3 text-[11px] text-slate-500 dark:text-slate-600">
            “Never / no last-used” includes negative or missing <span className="font-mono">days_since_used</span> in
            detail.
          </p>
        </>
      )}
    </div>
  );
}
