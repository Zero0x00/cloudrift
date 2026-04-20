import type { ScanSummaryResponse } from "../../api/types";
import { formatCount } from "../../lib/format";

function ComboCard({
  title,
  subtitle,
  value,
  tooltip,
  onClick
}: {
  title: string;
  subtitle: string;
  value: string;
  /** Native tooltip: longer distinction (e.g. tier vs capability). */
  tooltip?: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      title={tooltip}
      onClick={onClick}
      className="hs-card w-full p-4 text-left transition hover:border-cyan-300/80 hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-500/60 dark:hover:border-cyan-700/60"
    >
      <p className="text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-400">{title}</p>
      <p className="mt-2 text-2xl font-semibold tabular-nums text-slate-900 dark:text-slate-100">{value}</p>
      <p className="mt-1 text-[11px] leading-snug text-slate-500">{subtitle}</p>
    </button>
  );
}

export function HighRiskCombinationStrip({
  summary,
  onOpenReclaimable,
  onOpenPrivilegedExternal,
  onOpenStaleTrustExternal,
  onOpenAdminLikeExternal,
  onOpenStalePrivilegedExternal
}: {
  summary: ScanSummaryResponse;
  onOpenReclaimable: () => void;
  onOpenPrivilegedExternal: () => void;
  onOpenStaleTrustExternal: () => void;
  onOpenAdminLikeExternal: () => void;
  onOpenStalePrivilegedExternal: () => void;
}) {
  const priv = summary.external_privileged_count ?? 0;
  const stale = summary.external_trust_stale_count ?? 0;
  const admin = summary.external_admin_like_count ?? 0;
  const stalePriv = summary.external_stale_privileged_count ?? 0;

  return (
    <div>
      <h2 className="mb-3 text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
        High-signal combinations
      </h2>
      <p className="mb-3 max-w-3xl text-[11px] text-slate-500">
        Counts mirror GET …/findings on <code className="rounded bg-slate-100 px-0.5 dark:bg-slate-800">external_access</code>.{" "}
        <strong className="font-medium text-slate-600 dark:text-slate-400">Privileged</strong> is the backend{" "}
        <em>permission tier</em> (<code className="rounded bg-slate-100 px-0.5 dark:bg-slate-800">trust_classification</code>).{" "}
        <strong className="font-medium text-slate-600 dark:text-slate-400">Admin-like</strong> is a separate, stronger{" "}
        <em>capability flag</em> (<code className="rounded bg-slate-100 px-0.5 dark:bg-slate-800">admin_like</code>) — a role can be
        neither, one, or both; do not assume one implies the other.
      </p>
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <ComboCard
          title="Reclaimable"
          subtitle="reclaimable_count — opens findings with claimability=reclaimable."
          value={formatCount(summary.reclaimable_count)}
          onClick={onOpenReclaimable}
        />
        <ComboCard
          title="External + privileged (tier)"
          subtitle="Backend permission tier: classification = privileged. Not the same as admin-like."
          tooltip="Privileged counts rows where permission_visibility.classification is privileged (tier label from policy visibility). Admin-like is a different signal: the admin_like capability boolean. Overlap is possible but not guaranteed — hover the Admin-like card for that definition."
          value={formatCount(priv)}
          onClick={onOpenPrivilegedExternal}
        />
        <ComboCard
          title="External + stale trust"
          subtitle="evidence.verdict = stale_review_now (structured stale signal)."
          value={formatCount(stale)}
          onClick={onOpenStaleTrustExternal}
        />
        <ComboCard
          title="External + admin-like (flag)"
          subtitle="Stronger signal: capabilities.admin_like = true. Orthogonal to privileged tier."
          tooltip="Admin-like is the explicit admin_like capability flag on permission_visibility (heuristic-backed policy analysis). Privileged is the coarser permission tier classification. A role may be admin-like without privileged tier, or privileged without admin_like — compare both counts."
          value={formatCount(admin)}
          onClick={onOpenAdminLikeExternal}
        />
      </div>
      <div
        className="hs-card-soft mt-4 flex flex-col gap-2 px-4 py-3 sm:flex-row sm:items-center sm:justify-between"
        title="Uses privileged permission tier only (not admin_like). Stale = trust verdict stale_review_now. Same AND as Findings URL."
      >
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">Stale + privileged (tier)</p>
          <p className="mt-0.5 text-[11px] text-slate-500">
            Stale verdict ∩ privileged <em>tier</em>. Does not require admin_like; for admin-like ∩ stale use filters in Findings.
          </p>
        </div>
        <button
          type="button"
          title="Open Findings: external_access + trust_stale + trust_classification=privileged (tier, not admin_like flag)."
          onClick={onOpenStalePrivilegedExternal}
          className="inline-flex shrink-0 items-center gap-2 rounded-md border border-cyan-600/50 bg-white px-3 py-2 text-sm font-semibold text-cyan-900 shadow-sm transition hover:bg-cyan-50 dark:border-cyan-700/60 dark:bg-slate-900 dark:text-cyan-100 dark:hover:bg-cyan-950/40"
        >
          <span className="tabular-nums text-lg">{formatCount(stalePriv)}</span>
          <span className="text-xs font-medium text-slate-600 dark:text-slate-300">Open filtered</span>
        </button>
      </div>
    </div>
  );
}
