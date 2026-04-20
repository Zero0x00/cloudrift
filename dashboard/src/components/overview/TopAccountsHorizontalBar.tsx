import type { AccountBreakdownItem } from "../../api/types";
import { formatCount } from "../../lib/format";

const TOP_N = 10;

function pct(part: number, total: number): number {
  if (total <= 0) {
    return 0;
  }
  return Math.round((100 * part) / total);
}

export function TopAccountsHorizontalBar({
  items,
  title = "Top accounts by findings",
  description,
  onAccountClick
}: {
  items: AccountBreakdownItem[];
  title?: string;
  description?: string;
  onAccountClick?: (accountId: string) => void;
}) {
  const safeItems = Array.isArray(items) ? items : [];
  const sorted = [...safeItems].sort((a, b) => b.finding_count - a.finding_count).slice(0, TOP_N);
  const max = Math.max(...sorted.map((a) => a.finding_count), 1);
  const blurb =
    description ??
    `From GET /api/scans/:id/accounts — top ${TOP_N} by finding_count (same numbers as the accounts table).`;

  return (
    <div className="rounded-lg border border-slate-200 bg-white/80 p-5 dark:border-slate-800 dark:bg-slate-900/80">
      <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-400">{title}</h3>
      <p className="mt-1 text-[11px] text-slate-500">{blurb}</p>
      {sorted.length === 0 ? (
        <p className="mt-3 text-sm text-slate-500">No account rows returned for this scan.</p>
      ) : (
        <ul className="mt-4 space-y-3">
          {sorted.map((account) => {
            const label = account.account_name?.trim() || account.account_id;
            const w = pct(account.finding_count, max);
            return (
            <li
              key={account.account_id}
              className={onAccountClick ? "cursor-pointer rounded px-1 py-1 hover:bg-slate-100 dark:hover:bg-slate-800/70" : ""}
              onClick={onAccountClick ? () => onAccountClick(account.account_id) : undefined}
            >
                <div className="mb-1 flex justify-between gap-3 text-xs text-slate-700 dark:text-slate-300">
                  <span className="min-w-0 truncate font-mono" title={`${label} (${account.account_id})`}>
                    {label}
                  </span>
                  <span className="shrink-0 tabular-nums text-slate-600 dark:text-slate-400">{formatCount(account.finding_count)}</span>
                </div>
                <div className="h-2 w-full overflow-hidden rounded bg-slate-800">
                  <div
                    className="h-full rounded bg-indigo-600/90"
                    style={{ width: `${w}%` }}
                    title={`${account.account_id}: ${account.finding_count}`}
                  />
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
