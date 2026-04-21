import type { ExternalPrincipalTypeCount, ScanSummaryResponse } from "../../api/types";
import { DonutChart } from "@tremor/react";
import { formatCount } from "../../lib/format";

function labelForPrincipalType(t: string): string {
  if (t === "unknown") {
    return "Unknown / unset";
  }
  return t.replace(/_/g, " ");
}

export function ExternalPrincipalTypesStrip({
  summary,
  onOpenPrincipalType
}: {
  summary: ScanSummaryResponse;
  onOpenPrincipalType: (principalType: string) => void;
}) {
  const rows = [...(summary.external_principal_types ?? [])].sort((a, b) => b.count - a.count);
  if (rows.length === 0) {
    return null;
  }
  const donutData = rows.map((row) => ({
    label: labelForPrincipalType(row.principal_type),
    key: row.principal_type,
    value: row.count
  }));
  const colors = ["cyan", "violet", "amber", "slate"];
  const dotClasses = ["bg-cyan-500", "bg-violet-500", "bg-amber-500", "bg-slate-500"];

  return (
    <div className="hs-card p-4">
      <h3 className="cr-section-title">External principal types</h3>
      <div className="mt-2 flex items-center justify-center">
        <DonutChart
          className="h-44"
          data={donutData}
          category="value"
          index="label"
          colors={colors}
          valueFormatter={(value) => formatCount(value)}
          onValueChange={(value) => {
            const key = donutData.find((item) => item.label === value?.label)?.key;
            if (key) {
              onOpenPrincipalType(key);
            }
          }}
          showAnimation
        />
      </div>
      <ul className="mt-2 space-y-1.5">
        {rows.map((row: ExternalPrincipalTypeCount, idx) => (
          <li key={row.principal_type}>
            <button
              type="button"
              onClick={() => onOpenPrincipalType(row.principal_type)}
              className="hs-interactive-row flex w-full items-center justify-between rounded px-2 py-1 text-left text-xs"
            >
              <span className="flex items-center gap-2 text-slate-700 dark:text-slate-200">
                <span className={`h-2 w-2 rounded-full ${dotClasses[idx % dotClasses.length]}`} />
                {labelForPrincipalType(row.principal_type)}
              </span>
              <span className="tabular-nums text-slate-500 dark:text-slate-400">{formatCount(row.count)}</span>
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
