import type { ScanSummaryResponse } from "../../api/types";
import { DonutChart } from "@tremor/react";
import { formatCount } from "../../lib/format";

type Props = {
  summary: ScanSummaryResponse;
  onOpenEntityListForPrincipalType: (principalType: string) => void;
};

/**
 * Entity counts by principal_type (distinct external entities), unlike `external_principal_types` which counts findings.
 */
export function ExternalEntityByPrincipalTypeStrip({ summary, onOpenEntityListForPrincipalType }: Props) {
  const rows = [...(summary.external_entity_by_principal_type ?? [])].sort((a, b) => b.entity_count - a.entity_count);
  if (rows.length === 0) {
    return null;
  }
  const donutData = rows.map((r) => ({
    label: r.principal_type,
    key: r.principal_type,
    value: r.entity_count
  }));
  const dotClasses = ["bg-cyan-500", "bg-violet-500", "bg-amber-500", "bg-slate-500"];

  return (
    <div className="hs-card p-4">
      <h3 className="cr-section-title">Entities by principal type</h3>
      <div className="mt-2 flex items-center justify-center">
        <DonutChart
          className="h-44"
          data={donutData}
          category="value"
          index="label"
          colors={["cyan", "violet", "amber", "slate"]}
          valueFormatter={(value) => formatCount(value)}
          onValueChange={(value) => {
            const key = donutData.find((item) => item.label === value?.label)?.key;
            if (key) {
              onOpenEntityListForPrincipalType(key);
            }
          }}
          showAnimation
        />
      </div>
      <ul className="mt-2 space-y-1.5">
        {rows.map((row, idx) => (
          <li key={row.principal_type}>
            <button
              type="button"
              onClick={() => onOpenEntityListForPrincipalType(row.principal_type)}
              className="hs-interactive-row flex w-full items-center justify-between rounded px-2 py-1 text-left text-xs"
            >
              <span className="flex items-center gap-2 text-slate-700 dark:text-slate-200">
                <span className={`h-2 w-2 rounded-full ${dotClasses[idx % dotClasses.length]}`} />
                <span className="font-mono">{row.principal_type}</span>
              </span>
              <span className="tabular-nums text-slate-500 dark:text-slate-400">{formatCount(row.entity_count)}</span>
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
