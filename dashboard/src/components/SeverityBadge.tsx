const severityRing: Record<string, string> = {
  critical: "bg-rose-950/80 text-rose-200 ring-rose-700/60",
  high: "bg-orange-950/70 text-orange-200 ring-orange-700/50",
  medium: "bg-amber-950/60 text-amber-100 ring-amber-700/40",
  low: "bg-slate-200 text-slate-800 ring-slate-400/50 dark:bg-slate-800 dark:text-slate-200 dark:ring-slate-600/50",
  info: "bg-slate-200 text-slate-700 ring-slate-400/50 dark:bg-slate-800 dark:text-slate-300 dark:ring-slate-600/50"
};

export function severityBadgeClass(severity: string): string {
  const key = severity.toLowerCase();
  return severityRing[key] ?? severityRing.info;
}

export function SeverityBadge({ severity }: { severity: string }) {
  const label = severity ? severity : "—";
  return (
    <span
      className={`inline-flex rounded px-2 py-0.5 text-xs font-medium capitalize ring-1 ${severityBadgeClass(label)}`}
    >
      {label}
    </span>
  );
}

/** Bar segment color (Tailwind bg class) aligned with badges. */
export function severityBarClass(severity: string): string {
  switch (severity.toLowerCase()) {
    case "critical":
      return "bg-rose-500";
    case "high":
      return "bg-orange-500";
    case "medium":
      return "bg-amber-400";
    case "low":
      return "bg-slate-500";
    default:
      return "bg-slate-600";
  }
}
