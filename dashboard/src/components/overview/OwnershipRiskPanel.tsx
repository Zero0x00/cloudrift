import { BarList } from "@tremor/react";
import type { AccountBreakdownItem } from "../../api/types";
import { formatUsd } from "../../lib/format";

const DEFAULT_LIMIT = 6;

export type OwnershipRiskItem = {
  accountId: string;
  accountName: string;
  team?: string;
  riskUsd: number;
  criticalCount: number;
  highCount: number;
  topFinding?: string;
};

function normalizeOwnershipItems(items: AccountBreakdownItem[]): OwnershipRiskItem[] {
  return items
    .filter((item) => Boolean(item?.account_id))
    .map((item) => ({
      accountId: item.account_id,
      accountName: item.account_name?.trim() || item.account_id,
      team: item.team?.trim() || undefined,
      riskUsd: Number.isFinite(item.total_monthly_risk_cost_usd) ? item.total_monthly_risk_cost_usd : 0,
      criticalCount: Number.isFinite(item.critical_count) ? item.critical_count : 0,
      highCount: Number.isFinite(item.high_count) ? item.high_count : 0,
      topFinding: item.top_finding?.trim() || undefined
    }));
}

export function sortOwnershipRiskItems(items: OwnershipRiskItem[]): OwnershipRiskItem[] {
  return [...items].sort((a, b) => {
    if (b.riskUsd !== a.riskUsd) {
      return b.riskUsd - a.riskUsd;
    }
    if (b.criticalCount !== a.criticalCount) {
      return b.criticalCount - a.criticalCount;
    }
    if (b.highCount !== a.highCount) {
      return b.highCount - a.highCount;
    }
    const byName = a.accountName.localeCompare(b.accountName);
    if (byName !== 0) {
      return byName;
    }
    return a.accountId.localeCompare(b.accountId);
  });
}

export function OwnershipRiskPanel({
  items,
  limit = DEFAULT_LIMIT,
  onAccountClick
}: {
  items: AccountBreakdownItem[];
  limit?: number;
  onAccountClick?: (accountId: string) => void;
}) {
  const normalized = normalizeOwnershipItems(Array.isArray(items) ? items : []);
  const ranked = sortOwnershipRiskItems(normalized).slice(0, Math.max(1, Math.min(limit, 8)));

  return (
    <div className="hs-card p-5">
      <h3 className="cr-section-title">Ownership risk</h3>
      <p className="cr-helper mt-1">Top owning accounts by monthly risk, then critical and high findings.</p>

      {ranked.length === 0 ? (
        <p className="mt-4 text-sm text-slate-500">No risk detected across accounts</p>
      ) : (
        <div
          className="cr-chart-focusable mt-4 rounded-md"
          tabIndex={0}
          aria-label="Ownership risk chart ranked by account monthly risk"
        >
          <BarList
            data={ranked.map((item) => ({
              key: item.accountId,
              name: (
                <button
                  type="button"
                  className="group w-full text-left"
                  onClick={() => onAccountClick?.(item.accountId)}
                >
                  <div className="flex items-center gap-2">
                    <span className="truncate text-xs font-medium text-slate-800 group-hover:text-cyan-700 dark:text-slate-200 dark:group-hover:text-cyan-300">
                      {item.team ? `${item.team} - ${item.accountName}` : item.accountName}
                    </span>
                    <span className="hs-chip-compact border-rose-300 bg-rose-100 text-rose-700 dark:border-rose-700/50 dark:bg-rose-950/40 dark:text-rose-300">
                      C {item.criticalCount}
                    </span>
                    <span className="hs-chip-compact border-amber-300 bg-amber-100 text-amber-700 dark:border-amber-700/50 dark:bg-amber-950/40 dark:text-amber-300">
                      H {item.highCount}
                    </span>
                  </div>
                  <p className="cr-helper mt-0.5 truncate">Top issue: {item.topFinding ?? "No finding title"}</p>
                </button>
              ),
              value: item.riskUsd,
              color: "emerald"
            }))}
            valueFormatter={(value: number) => formatUsd(value)}
            showAnimation
            onValueChange={(value: { key?: string | number }) => {
              if (value?.key) {
                onAccountClick?.(String(value.key));
              }
            }}
          />
        </div>
      )}
    </div>
  );
}

