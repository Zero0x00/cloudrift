import { useFindingDetailQuery } from "../../hooks/useDashboardQueries";
import { formatPermissionTierLabel, permissionTierBadgeClass } from "../../lib/permissionVisibility";

const TIER_TOOLTIP =
  "Permission tier from cached finding detail only (no extra request). Shown after expand or when detail was already loaded (e.g. activity chart sample). Not exact IAM evaluation — expand for full visibility.";

/**
 * Compact tier hint for list rows: reads TanStack Query cache for this finding’s detail query.
 * `enabled: false` prevents fetches; the chip appears only when detail is already in cache.
 */
export function CachedPermissionTierChip({ scanId, findingId }: { scanId: string; findingId: string }) {
  const { data } = useFindingDetailQuery(scanId, findingId, false);
  const classification = data?.item.trust?.permission_visibility?.classification;
  if (!classification) {
    return null;
  }

  return (
    <span
      className={`inline-flex max-w-[7.5rem] truncate rounded-full border px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide ${permissionTierBadgeClass(classification)}`}
      title={TIER_TOOLTIP}
    >
      {formatPermissionTierLabel(classification)}
    </span>
  );
}
