import type { ScanSummaryResponse } from "../../api/types";
import { formatCount } from "../../lib/format";

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
  const items = [
    { label: "Reclaimable", count: summary.reclaimable_count, onClick: onOpenReclaimable },
    { label: "External + Privileged", count: priv, onClick: onOpenPrivilegedExternal },
    { label: "Stale Trust", count: stale, onClick: onOpenStaleTrustExternal },
    { label: "Admin-like", count: admin, onClick: onOpenAdminLikeExternal },
    { label: "Stale + Privileged", count: stalePriv, onClick: onOpenStalePrivilegedExternal }
  ];

  return (
    <div className="hs-card-soft border-slate-300 p-4 dark:border-slate-700">
      <div className="mb-2 flex items-center justify-between gap-2">
        <h3 className="cr-section-title !text-xs">High-signal combinations</h3>
      </div>
      <div className="flex flex-wrap gap-2">
        {items.map((item) => (
          <button
            key={item.label}
            type="button"
            onClick={item.onClick}
            className="hs-action-pill gap-2"
          >
            <span>{item.label}</span>
            <span className="tabular-nums text-slate-500 dark:text-slate-400">{formatCount(item.count)}</span>
          </button>
        ))}
      </div>
    </div>
  );
}
