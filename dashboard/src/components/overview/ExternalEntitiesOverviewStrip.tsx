import type { ScanSummaryResponse } from "../../api/types";
import { formatCount } from "../../lib/format";

type Props = {
  summary: ScanSummaryResponse;
  onOpenAllEntities: () => void;
  onOpenEntitiesWithStale: () => void;
  onOpenEntitiesWithPrivileged: () => void;
  onOpenEntitiesWithAdminLike: () => void;
};

/**
 * Entity-centric rollups from GET …/summary (same aggregation as GET …/external-entities without filters).
 */
export function ExternalEntitiesOverviewStrip({
  summary,
  onOpenAllEntities,
  onOpenEntitiesWithStale,
  onOpenEntitiesWithPrivileged,
  onOpenEntitiesWithAdminLike
}: Props) {
  const n = summary.external_entity_count ?? 0;
  if (n === 0) {
    return null;
  }

  return (
    <div className="hs-card p-4">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <div>
          <h3 className="cr-section-title">External entities</h3>
          <p
            className="cr-helper mt-1"
            title="An entity is the tuple (external_principal, principal_type, external_account_id) from trust evidence. Missing evidence in any dimension renders as 'unknown' and may merge multiple unidentified entries into the same bucket. Finding-level counts stay in Access / Findings."
          >
            Distinct external principals by type/account. Missing evidence is grouped as{" "}
            <span className="font-mono">unknown</span>.
          </p>
        </div>
      </div>
      <div className="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <MetricCard
          label="Entities"
          value={formatCount(n)}
          hint="Matches External Entities page total (unfiltered)."
          onClick={onOpenAllEntities}
        />
        <MetricCard
          label="With ≥1 stale role"
          value={formatCount(summary.external_entities_with_stale_role ?? 0)}
          hint="Entity has AT LEAST ONE trusted role with verdict stale_review_now. Other roles for the same entity may still be fresh."
          onClick={onOpenEntitiesWithStale}
        />
        <MetricCard
          label="With ≥1 privileged role"
          value={formatCount(summary.external_entities_with_privileged_tier ?? 0)}
          hint="Entity has AT LEAST ONE role where permission_visibility.classification = privileged. Other roles for the same entity may be lower tier."
          onClick={onOpenEntitiesWithPrivileged}
        />
        <MetricCard
          label="With ≥1 admin-like role"
          value={formatCount(summary.external_entities_with_admin_like_flag ?? 0)}
          hint="Entity has AT LEAST ONE role with permission_visibility.capabilities.admin_like = true. Other roles for the same entity may not be admin-like."
          onClick={onOpenEntitiesWithAdminLike}
        />
      </div>
    </div>
  );
}

function MetricCard({
  label,
  value,
  hint,
  onClick
}: {
  label: string;
  value: string;
  hint: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      title={hint}
      onClick={onClick}
      className="rounded-md border border-slate-200 bg-slate-50 px-3 py-3 text-left transition hover:border-cyan-400/60 hover:bg-cyan-50/50 dark:border-slate-700 dark:bg-slate-900/60 dark:hover:border-cyan-600/50 dark:hover:bg-cyan-950/20"
    >
      <p className="text-[11px] font-medium uppercase tracking-wide text-slate-500 dark:text-slate-400">{label}</p>
      <p className="mt-1 text-2xl font-semibold tabular-nums text-slate-900 dark:text-slate-100">{value}</p>
    </button>
  );
}
