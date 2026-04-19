import { formatQueryError } from "../api/httpError";
import type { FindingListItem } from "../api/types";
import { DiffFindingCard } from "../components/diff/DiffFindingCard";
import { PageHeader } from "../components/PageHeader";
import { ScanRequired } from "../components/ScanRequired";
import { StatePanel } from "../components/StatePanel";
import { useDiffQuery } from "../hooks/useDashboardQueries";
import { useScanContext } from "../hooks/useScanContext";
import { getPreviousScanIdForDiff } from "../lib/scanOrder";
import { formatCount } from "../lib/format";

export function DiffPage() {
  const { selectedScanId, scans } = useScanContext();
  const query = useDiffQuery();
  const oldScanId = getPreviousScanIdForDiff(scans, selectedScanId);

  const data = query.data;
  const newCount = data?.new_findings.length ?? 0;
  const resolvedCount = data?.resolved_findings.length ?? 0;
  const unchanged = data?.unchanged_count ?? 0;
  const noDrift = data && newCount === 0 && resolvedCount === 0;

  return (
    <section className="space-y-6">
      <PageHeader
        title="Diff"
        description="Compares the selected scan (new) to the chronologically previous scan in the newest-first list from GET /api/scans. Baseline = next row after the selected scan."
        scanId={selectedScanId}
      />
      {!selectedScanId ? (
        <ScanRequired />
      ) : !oldScanId ? (
        <StatePanel intent="empty" title="No baseline scan">
          Need at least two scans, or select a scan that is not the oldest in the list. The baseline is always the{" "}
          <strong className="text-slate-700 dark:text-slate-300">next</strong> scan after the current one in newest-first order from{" "}
          <code className="text-cyan-800 dark:text-cyan-200/80">/api/scans</code>.
        </StatePanel>
      ) : query.isLoading ? (
        <StatePanel>Loading diff…</StatePanel>
      ) : query.isError ? (
        <StatePanel intent="error" title="Failed to load diff">
          <pre className="whitespace-pre-wrap font-sans text-xs">{formatQueryError(query.error)}</pre>
        </StatePanel>
      ) : !data ? (
        <StatePanel intent="empty" title="Empty response">The diff request succeeded but returned no payload.</StatePanel>
      ) : (
        <>
          <div
            className="flex flex-wrap items-baseline justify-between gap-3 rounded-lg border border-slate-300 bg-white/95 px-4 py-3 dark:border-slate-700 dark:bg-slate-950/80"
            role="status"
            aria-live="polite"
          >
            <p className="text-sm text-slate-700 dark:text-slate-200">
              <span className="font-semibold text-amber-800 dark:text-amber-300">+{formatCount(newCount)}</span>
              <span className="text-slate-500"> new</span>
              <span className="mx-2 text-slate-400 dark:text-slate-600">·</span>
              <span className="font-semibold text-emerald-800 dark:text-emerald-300">−{formatCount(resolvedCount)}</span>
              <span className="text-slate-500"> resolved</span>
              <span className="mx-2 text-slate-400 dark:text-slate-600">·</span>
              <span className="font-semibold tabular-nums text-slate-800 dark:text-slate-100">{formatCount(unchanged)}</span>
              <span className="text-slate-500"> unchanged</span>
            </p>
            <p className="text-xs text-slate-500">Title + ARN identity (API). Lists below use severity from each scan.</p>
          </div>

          <div className="rounded-lg border border-slate-200 bg-slate-50/95 p-4 text-sm text-slate-700 dark:border-slate-800 dark:bg-slate-900/60 dark:text-slate-300">
            <p>
              <span className="text-slate-500">Old (baseline): </span>
              <code className="text-cyan-800 dark:text-cyan-200/90">{data.old_scan_id}</code>
              <span className="mx-2 text-slate-400 dark:text-slate-600">→</span>
              <span className="text-slate-500">New: </span>
              <code className="text-cyan-800 dark:text-cyan-200/90">{data.new_scan_id}</code>
            </p>
            <p className="mt-2 text-xs text-slate-500">
              Identity for matching: finding title + affected ARN (per API). GET /api/diff?old=…&amp;new=…
            </p>
          </div>

          <div className="grid gap-4 sm:grid-cols-3">
            <DiffMetric label="New findings" value={newCount} />
            <DiffMetric label="Resolved findings" value={resolvedCount} />
            <DiffMetric label="Unchanged" value={unchanged} />
          </div>

          {noDrift && unchanged > 0 ? (
            <StatePanel intent="empty" title="No drift in tracked identities">
              No new or resolved findings by title+ARN identity.{" "}
              <span className="tabular-nums text-slate-600 dark:text-slate-400">{formatCount(unchanged)}</span> finding identities unchanged.
            </StatePanel>
          ) : null}

          {newCount > 0 ? (
            <DiffSection variant="new" title="New findings" subtitle="Present in the new scan only." items={data.new_findings} />
          ) : null}

          {resolvedCount > 0 ? (
            <DiffSection
              variant="resolved"
              title="Resolved findings"
              subtitle="Present in the baseline scan only."
              items={data.resolved_findings}
            />
          ) : null}

          {!noDrift && newCount === 0 && resolvedCount === 0 ? (
            <StatePanel intent="empty" title="No new or resolved rows">
              Diff lists are empty; see unchanged count above.
            </StatePanel>
          ) : null}
        </>
      )}
    </section>
  );
}

function DiffMetric({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-lg border border-slate-200 bg-white/90 p-4 dark:border-slate-800 dark:bg-slate-900/85">
      <p className="text-xs uppercase tracking-wide text-slate-500">{label}</p>
      <p className="mt-2 text-2xl font-semibold tabular-nums text-slate-900 dark:text-slate-100">{formatCount(value)}</p>
    </div>
  );
}

function DiffSection({
  title,
  subtitle,
  items,
  variant
}: {
  title: string;
  subtitle: string;
  items: FindingListItem[];
  variant: "new" | "resolved";
}) {
  return (
    <div>
      <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-400">{title}</h2>
      <p className="mt-1 text-xs text-slate-500">{subtitle}</p>
      <div className="mt-3 space-y-2">
        {items.map((item) => (
          <DiffFindingCard key={`${item.id}-${item.affected_arn}`} item={item} variant={variant} />
        ))}
      </div>
    </div>
  );
}
