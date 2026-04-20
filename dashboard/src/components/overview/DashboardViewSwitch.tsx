import type { DashboardViewId } from "../../hooks/useDashboardViewUrlState";
import { IconBriefcase, IconSliders, IconZap } from "../../lib/icons";

const MODES: { id: DashboardViewId; label: string; Icon: typeof IconBriefcase }[] = [
  { id: "executive", label: "Executive Summary", Icon: IconBriefcase },
  { id: "high-signal", label: "High-Signal", Icon: IconZap },
  { id: "operations", label: "Operations", Icon: IconSliders }
];

export function DashboardViewSwitch({
  view,
  onChange
}: {
  view: DashboardViewId;
  onChange: (v: DashboardViewId) => void;
}) {
  return (
    <div
      className="flex flex-wrap gap-2 rounded-lg border border-slate-200 bg-slate-50/90 p-1.5 dark:border-slate-800 dark:bg-slate-900/50"
      role="tablist"
      aria-label="Dashboard mode"
    >
      {MODES.map(({ id, label, Icon }) => {
        const active = view === id;
        return (
          <button
            key={id}
            type="button"
            role="tab"
            aria-selected={active}
            onClick={() => onChange(id)}
            className={[
              "inline-flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition",
              active
                ? "bg-white text-slate-900 shadow-sm ring-1 ring-slate-200 dark:bg-slate-800 dark:text-slate-100 dark:ring-slate-700"
                : "text-slate-600 hover:bg-white/70 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800/80 dark:hover:text-slate-100"
            ].join(" ")}
          >
            <Icon className="shrink-0 opacity-80" />
            {label}
          </button>
        );
      })}
    </div>
  );
}
