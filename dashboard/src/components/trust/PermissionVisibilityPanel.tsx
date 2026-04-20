import type { PermissionVisibilityDisplay } from "../../api/types";
import {
  enabledCapabilityLabels,
  formatPermissionConfidenceLabel,
  formatPermissionTierLabel,
  permissionAnalysisBadges,
  permissionTierBadgeClass
} from "../../lib/permissionVisibility";

function Badge({ children, className }: { children: string; className?: string }) {
  return (
    <span
      className={`inline-flex rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide ${className ?? ""}`}
    >
      {children}
    </span>
  );
}

export function PermissionVisibilityPanel({
  permission
}: {
  permission: PermissionVisibilityDisplay | undefined;
}) {
  if (!permission) {
    return (
      <p className="text-xs text-slate-500 dark:text-slate-400">
        Permission visibility is unavailable for this finding.
      </p>
    );
  }

  const tierLabel = formatPermissionTierLabel(permission.classification);
  const confidenceLabel = formatPermissionConfidenceLabel(permission.confidence);
  const capabilityLabels = enabledCapabilityLabels(permission.capabilities);
  const analysisBadges = permissionAnalysisBadges(permission);
  const reasons = permission.reasons ?? [];

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-1.5">
        <Badge className={permissionTierBadgeClass(permission.classification)}>{tierLabel}</Badge>
        {confidenceLabel ? (
          <Badge className="border-slate-300/90 bg-slate-100 text-slate-700 dark:border-slate-700/80 dark:bg-slate-800 dark:text-slate-300">
            {confidenceLabel}
          </Badge>
        ) : null}
      </div>

      <div className="space-y-1">
        <p className="text-[11px] font-medium uppercase tracking-wide text-slate-500 dark:text-slate-400">Capabilities</p>
        {capabilityLabels.length > 0 ? (
          <div className="flex flex-wrap gap-1.5">
            {capabilityLabels.map((label) => (
              <Badge
                key={label}
                className="border-indigo-300/90 bg-indigo-100 text-indigo-900 dark:border-indigo-700/80 dark:bg-indigo-900/30 dark:text-indigo-200"
              >
                {label}
              </Badge>
            ))}
          </div>
        ) : (
          <p className="text-xs text-slate-500 dark:text-slate-400">No elevated capability flags were detected.</p>
        )}
      </div>

      {analysisBadges.length > 0 ? (
        <div className="space-y-1">
          <p className="text-[11px] font-medium uppercase tracking-wide text-slate-500 dark:text-slate-400">Analysis status</p>
          <div className="flex flex-wrap gap-1.5">
            {analysisBadges.map((label) => (
              <Badge
                key={label}
                className="border-slate-300/90 bg-slate-100 text-slate-600 dark:border-slate-700/80 dark:bg-slate-800 dark:text-slate-300"
              >
                {label}
              </Badge>
            ))}
          </div>
        </div>
      ) : null}

      {reasons.length > 0 ? (
        <div className="space-y-1">
          <p className="text-[11px] font-medium uppercase tracking-wide text-slate-500 dark:text-slate-400">Why this tier</p>
          <ul className="list-inside list-disc space-y-1 text-xs text-slate-700 dark:text-slate-300">
            {reasons.map((reason) => (
              <li key={reason}>{reason}</li>
            ))}
          </ul>
        </div>
      ) : null}

      <p className="text-[11px] text-slate-500 dark:text-slate-400">
        Tier/capability labels are conservative visibility signals from backend analysis, not exact IAM permission simulation.
      </p>
    </div>
  );
}
