import { formatQueryError } from "../api/httpError";
import type { ExternalEntityRow, ScanListItem, ScanSummaryResponse } from "../api/types";
import { ExecutiveSummaryStrip } from "../components/overview/ExecutiveSummaryStrip";
import { ExternalEntitiesOverviewStrip } from "../components/overview/ExternalEntitiesOverviewStrip";
import { ExternalEntityByPrincipalTypeStrip } from "../components/overview/ExternalEntityByPrincipalTypeStrip";
import { ExternalPrincipalTypesStrip } from "../components/overview/ExternalPrincipalTypesStrip";
import { TopRiskyExternalEntitiesPreview } from "../components/overview/TopRiskyExternalEntitiesPreview";
import { HighRiskCombinationStrip } from "../components/overview/HighRiskCombinationStrip";
import {
  ClaimabilityDistribution,
  DirectVsRiskCostSplit,
  ModuleDistribution,
  SeverityDistribution
} from "../components/overview/SummaryVisualizations";
import { ScanRiskTrendChart } from "../components/overview/ScanRiskTrendChart";
import { SecondaryMetricsStrip } from "../components/overview/SecondaryMetricsStrip";
import { TopAccountsHorizontalBar } from "../components/overview/TopAccountsHorizontalBar";
import { DashboardViewSwitch } from "../components/overview/DashboardViewSwitch";
import { OperationsScanSummaryCard } from "../components/overview/OperationsScanSummaryCard";
import { PageHeader } from "../components/PageHeader";
import { ScanRequired } from "../components/ScanRequired";
import { StatePanel } from "../components/StatePanel";
import { useAccountsQuery, useSummaryQuery } from "../hooks/useDashboardQueries";
import { useDashboardViewUrlState } from "../hooks/useDashboardViewUrlState";
import { useScanContext } from "../hooks/useScanContext";
import { formatCount } from "../lib/format";
import { Link, useNavigate } from "react-router-dom";

export function OverviewPage() {
  const { selectedScanId, scans, scansQuery } = useScanContext();
  const { view, setView } = useDashboardViewUrlState();
  const query = useSummaryQuery();
  const accountsQuery = useAccountsQuery();
  const navigate = useNavigate();

  const goToFindings = (params: Record<string, string>) => {
    const q = new URLSearchParams(params);
    if (selectedScanId) {
      q.set("scan_id", selectedScanId);
    }
    navigate({ pathname: "/findings", search: `?${q.toString()}` });
  };

  const goToTrust = (params: Record<string, string>) => {
    const q = new URLSearchParams(params);
    if (selectedScanId) {
      q.set("scan_id", selectedScanId);
    }
    navigate({ pathname: "/trust-report", search: `?${q.toString()}` });
  };

  const goToExternalEntities = (params: Record<string, string>) => {
    const q = new URLSearchParams(params);
    if (selectedScanId) {
      q.set("scan_id", selectedScanId);
    }
    navigate({ pathname: "/external-entities", search: `?${q.toString()}` });
  };

  const trendEnabled = Boolean(scansQuery.isSuccess && scans.length >= 2);
  const summary = query.data;

  const modeSwitch = <DashboardViewSwitch view={view} onChange={setView} />;

  return (
    <section className="space-y-8">
      <PageHeader
        title="Dashboard"
        description="Switch modes to emphasize leadership summaries, urgent signals, or day-to-day operations. Drilldowns open the same Findings, Access, and External views as before."
      />

      {!selectedScanId ? (
        <ScanRequired />
      ) : (
        <>
          {modeSwitch}
          {query.isLoading ? (
            <StatePanel>Loading summary…</StatePanel>
          ) : query.isError ? (
            <StatePanel intent="error" title="Failed to load summary">
              <pre className="whitespace-pre-wrap font-sans text-xs">{formatQueryError(query.error)}</pre>
            </StatePanel>
          ) : summary ? (
            <>
              {view === "executive" ? (
                <ExecutiveLayout
                  summary={summary}
                  scans={scans}
                  selectedScanId={selectedScanId}
                  trendEnabled={trendEnabled}
                  goToFindings={goToFindings}
                  goToTrust={goToTrust}
                  goToExternalEntities={goToExternalEntities}
                />
              ) : view === "high-signal" ? (
                <HighSignalLayout
                  summary={summary}
                  selectedScanId={selectedScanId}
                  trendEnabled={trendEnabled}
                  scans={scans}
                  goToFindings={goToFindings}
                  goToTrust={goToTrust}
                  goToExternalEntities={goToExternalEntities}
                />
              ) : (
                <OperationsLayout
                  summary={summary}
                  selectedScanId={selectedScanId}
                  accountsQuery={accountsQuery}
                  goToFindings={goToFindings}
                  goToTrust={goToTrust}
                  goToExternalEntities={goToExternalEntities}
                />
              )}
            </>
          ) : (
            <StatePanel intent="empty" title="No data">
              Summary response was empty.
            </StatePanel>
          )}
        </>
      )}
    </section>
  );
}

function ExecutiveLayout({
  summary,
  scans,
  selectedScanId,
  trendEnabled,
  goToFindings,
  goToTrust,
  goToExternalEntities
}: {
  summary: ScanSummaryResponse;
  scans: ScanListItem[];
  selectedScanId: string;
  trendEnabled: boolean;
  goToFindings: (params: Record<string, string>) => void;
  goToTrust: (params: Record<string, string>) => void;
  goToExternalEntities: (params: Record<string, string>) => void;
}) {
  return (
    <div className="hs-section space-y-6">
      <ExecutiveSummaryStrip
        summary={summary}
        onOpenCritical={() => goToFindings({ severity: "critical", page: "1" })}
        onOpenRiskCost={() => goToFindings({ page: "1" })}
        onOpenExternal={() => goToTrust({ page: "1" })}
        onOpenReclaimable={() => goToFindings({ claimability: "reclaimable", page: "1" })}
      />

      <ScanRiskTrendChart scans={scans} selectedScanId={selectedScanId} enabled={trendEnabled} />

      <div className="grid gap-6 lg:grid-cols-2">
        <DirectVsRiskCostSplit summary={summary} />
        <SeverityDistribution summary={summary} onSeverityClick={(severity) => goToFindings({ severity, page: "1" })} />
      </div>

      {(summary.external_entity_count ?? 0) > 0 ? (
        <ExternalEntitiesOverviewStrip
          summary={summary}
          onOpenAllEntities={() => goToExternalEntities({ page: "1" })}
          onOpenEntitiesWithStale={() => goToExternalEntities({ page: "1", has_stale_role: "true" })}
          onOpenEntitiesWithPrivileged={() => goToExternalEntities({ page: "1", has_privileged_role: "true" })}
          onOpenEntitiesWithAdminLike={() => goToExternalEntities({ page: "1", has_admin_like_role: "true" })}
        />
      ) : null}
    </div>
  );
}

function HighSignalLayout({
  summary,
  selectedScanId,
  trendEnabled,
  scans,
  goToFindings,
  goToTrust,
  goToExternalEntities
}: {
  summary: ScanSummaryResponse;
  selectedScanId: string;
  trendEnabled: boolean;
  scans: ScanListItem[];
  goToFindings: (params: Record<string, string>) => void;
  goToTrust: (params: Record<string, string>) => void;
  goToExternalEntities: (params: Record<string, string>) => void;
}) {
  return (
    <div className="hs-section space-y-6">
      <HighRiskCombinationStrip
        summary={summary}
        onOpenReclaimable={() => goToFindings({ claimability: "reclaimable", page: "1" })}
        onOpenPrivilegedExternal={() =>
          goToFindings({ module: "external_access", trust_classification: "privileged", page: "1" })
        }
        onOpenStaleTrustExternal={() => goToFindings({ module: "external_access", trust_stale: "true", page: "1" })}
        onOpenAdminLikeExternal={() => goToFindings({ module: "external_access", admin_like: "true", page: "1" })}
        onOpenStalePrivilegedExternal={() =>
          goToFindings({
            module: "external_access",
            trust_stale: "true",
            trust_classification: "privileged",
            page: "1"
          })
        }
      />

      <div className="grid gap-6 xl:grid-cols-2">
        <ExternalPrincipalTypesStrip
          summary={summary}
          onOpenPrincipalType={(principalType) =>
            goToFindings({
              module: "external_access",
              principal_type: principalType,
              page: "1"
            })
          }
        />
        {(summary.external_entities_preview?.length ?? 0) > 0 ? (
          <TopRiskyExternalEntitiesPreview
            summary={summary}
            onOpenExternalEntitiesPage={() => goToExternalEntities({ page: "1" })}
            onOpenEntityFindings={(row: ExternalEntityRow) =>
              goToFindings({
                module: "external_access",
                page: "1",
                principal_type: row.principal_type,
                external_principal: row.external_principal,
                external_account_id: row.external_account_id
              })
            }
          />
        ) : (
          <div className="hs-card-soft border-dashed border-slate-300 p-6 dark:border-slate-700">
            <p className="hs-section-title">Top external entities</p>
            <p className="cr-helper mt-2">No high-signal external preview for this scan.</p>
          </div>
        )}
      </div>

      <FocusPanel summary={summary} goToFindings={goToFindings} goToTrust={goToTrust} />

      <SecondaryMetricsStrip
        summary={summary}
        onOpenHigh={() => goToFindings({ severity: "high", page: "1" })}
        onOpenMedium={() => goToFindings({ severity: "medium", page: "1" })}
        onOpenAll={() => goToFindings({ page: "1" })}
        onOpenOrphaned={() => goToFindings({ module: "orphaned_edge", page: "1" })}
      />

      <ScanRiskTrendChart scans={scans} selectedScanId={selectedScanId} enabled={trendEnabled} />
    </div>
  );
}

function OperationsLayout({
  summary,
  selectedScanId,
  accountsQuery,
  goToFindings,
  goToTrust,
  goToExternalEntities
}: {
  summary: ScanSummaryResponse;
  selectedScanId: string;
  accountsQuery: ReturnType<typeof useAccountsQuery>;
  goToFindings: (params: Record<string, string>) => void;
  goToTrust: (params: Record<string, string>) => void;
  goToExternalEntities: (params: Record<string, string>) => void;
}) {
  return (
    <div className="hs-section space-y-6">
      <OperationsScanSummaryCard scanId={selectedScanId} />

      <div className="hs-card p-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h3 className="cr-section-title !normal-case !tracking-normal text-sm text-slate-800 dark:text-slate-200">
            Top risky accounts
          </h3>
          <Link
            to={selectedScanId ? `/accounts?scan_id=${encodeURIComponent(selectedScanId)}` : "/accounts"}
            className="hs-btn-default cr-chip px-2.5 py-1.5 font-medium"
          >
            Open Accounts
          </Link>
        </div>
        {accountsQuery.isLoading ? (
          <p className="cr-helper mt-3">Loading accounts…</p>
        ) : accountsQuery.isError ? (
          <p className="mt-3 text-sm text-rose-600 dark:text-rose-400">Chart unavailable.</p>
        ) : accountsQuery.data ? (
          <div className="mt-3">
            <TopAccountsHorizontalBar
              items={accountsQuery.data.items}
              onAccountClick={(accountId) => goToFindings({ account_id: accountId, page: "1" })}
            />
          </div>
        ) : null}
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <ModuleDistribution summary={summary} onModuleClick={(module) => goToFindings({ module, page: "1" })} />
        <ClaimabilityDistribution
          summary={summary}
          onClaimabilityClick={(claimability) => goToFindings({ claimability, page: "1" })}
        />
      </div>

      <SecondaryMetricsStrip
        summary={summary}
        onOpenHigh={() => goToFindings({ severity: "high", page: "1" })}
        onOpenMedium={() => goToFindings({ severity: "medium", page: "1" })}
        onOpenAll={() => goToFindings({ page: "1" })}
        onOpenOrphaned={() => goToFindings({ module: "orphaned_edge", page: "1" })}
      />

      <div className="grid gap-4 md:grid-cols-3">
        <OpsNextLink
          title="Findings inbox"
          detail="Server-paginated filters and expanded detail"
          onClick={() => goToFindings({ page: "1" })}
        />
        <OpsNextLink title="Access review" detail="External trust posture" onClick={() => goToTrust({ page: "1" })} />
        <OpsNextLink
          title="External entities"
          detail="Entity-centric rollups"
          onClick={() => goToExternalEntities({ page: "1" })}
        />
      </div>

      {(summary.external_entity_count ?? 0) > 0 ? (
        <ExternalEntityByPrincipalTypeStrip
          summary={summary}
          onOpenEntityListForPrincipalType={(pt) => goToExternalEntities({ page: "1", principal_type: pt })}
        />
      ) : null}
    </div>
  );
}

function OpsNextLink({ title, detail, onClick }: { title: string; detail: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="hs-card-soft px-3 py-3 text-left transition hover:border-cyan-300 hover:bg-cyan-50/60 dark:hover:border-cyan-700 dark:hover:bg-cyan-950/25"
    >
      <p className="text-sm font-semibold text-slate-800 dark:text-slate-100">{title}</p>
      <p className="cr-helper mt-1">{detail}</p>
    </button>
  );
}

function FocusPanel({
  summary,
  goToFindings,
  goToTrust
}: {
  summary: ScanSummaryResponse;
  goToFindings: (params: Record<string, string>) => void;
  goToTrust: (params: Record<string, string>) => void;
}) {
  return (
    <div className="hs-card p-5">
      <h3 className="cr-section-title">Investigation shortcuts</h3>
      <p className="cr-helper mt-1">Structured trust filters on Findings (aligned with summary rollups).</p>
      <div className="mt-4 grid gap-3 md:grid-cols-3">
        <FocusPanelItem
          title="All external access"
          detail={`${formatCount(summary.external_access_count)} findings — trust report UI`}
          onClick={() => goToTrust({ page: "1" })}
        />
        <FocusPanelItem
          title="Stale trust (verdict)"
          detail="Findings: trust_stale — verdict stale_review_now; unrelated to tier or admin_like."
          tooltip="Structured stale signal from trust scorer (verdict). Does not encode privileged vs admin_like; combine filters in Findings if you need those dimensions."
          onClick={() => goToFindings({ module: "external_access", trust_stale: "true", page: "1" })}
        />
        <FocusPanelItem
          title="Admin-like (capability flag)"
          detail="Findings: admin_like=true — not the same as privileged tier; hover for distinction."
          tooltip="admin_like is a specific capability boolean from policy visibility analysis. Privileged is trust_classification (coarse tier). A role can match one, both, or neither."
          onClick={() => goToFindings({ module: "external_access", admin_like: "true", page: "1" })}
        />
      </div>
    </div>
  );
}

function FocusPanelItem({
  title,
  detail,
  onClick,
  tooltip
}: {
  title: string;
  detail: string;
  onClick: () => void;
  tooltip?: string;
}) {
  return (
    <button
      type="button"
      title={tooltip}
      onClick={onClick}
      className="hs-card-soft rounded-md px-3 py-3 text-left transition hover:border-cyan-300 hover:bg-cyan-50/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-500/60 dark:hover:border-cyan-600 dark:hover:bg-cyan-950/30"
    >
      <p className="text-sm font-semibold text-slate-800 dark:text-slate-100">{title}</p>
      <p className="cr-helper mt-1">{detail}</p>
    </button>
  );
}
