import type { FindingListItem } from "../../api/types";
import { displayTarget, formatUsd, shortenArn } from "../../lib/format";
import { SeverityBadge } from "../SeverityBadge";

const variantStyles = {
  new: "border-l-4 border-l-amber-500/90",
  resolved: "border-l-4 border-l-emerald-600/85"
} as const;

/** Compact finding summary for diff lists (matches audit styling, lighter than full Findings table). */
export function DiffFindingCard({ item, variant }: { item: FindingListItem; variant: "new" | "resolved" }) {
  const target = displayTarget(item.hostname, item.affected_arn);
  const accent = variantStyles[variant];
  return (
    <article
      className={`rounded-lg border border-slate-200 bg-white/80 px-4 py-3 dark:border-slate-800 dark:bg-slate-900/80 ${accent}`}
    >
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium text-slate-900 dark:text-slate-100">{item.title}</p>
          <p className="mt-1 font-mono text-[11px] text-slate-500 break-all" title={item.affected_arn}>
            {target}
            {item.hostname?.trim() && item.affected_arn ? (
              <span className="block text-slate-500 dark:text-slate-600">{shortenArn(item.affected_arn, 32, 20)}</span>
            ) : null}
          </p>
        </div>
        <SeverityBadge severity={item.severity} />
      </div>
      <dl className="mt-3 grid grid-cols-2 gap-x-4 gap-y-1 text-xs sm:grid-cols-4">
        <div>
          <dt className="text-slate-500">Account</dt>
          <dd className="font-mono text-slate-700 dark:text-slate-300">{item.account_id || "—"}</dd>
        </div>
        <div>
          <dt className="text-slate-500">Module</dt>
          <dd className="capitalize text-slate-700 dark:text-slate-300">{item.module || "—"}</dd>
        </div>
        <div>
          <dt className="text-slate-500">Claimability</dt>
          <dd className="capitalize text-slate-700 dark:text-slate-300">{item.claimability || "—"}</dd>
        </div>
        <div>
          <dt className="text-slate-500">Risk / mo</dt>
          <dd className="tabular-nums text-slate-800 dark:text-slate-200">{formatUsd(item.monthly_risk_cost_usd)}</dd>
        </div>
      </dl>
    </article>
  );
}
