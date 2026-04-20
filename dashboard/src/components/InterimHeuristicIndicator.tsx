type InterimHeuristicIndicatorProps = {
  className?: string;
  label?: string;
  tooltip?: string;
};

const DEFAULT_TOOLTIP =
  "Interim heuristic: derived from existing list fields and keywords. Useful for triage, not authoritative classification.";

export function InterimHeuristicIndicator({
  className = "",
  label = "heuristic",
  tooltip = DEFAULT_TOOLTIP
}: InterimHeuristicIndicatorProps) {
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full border border-amber-300/70 bg-amber-100/85 px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-amber-900 dark:border-amber-700/70 dark:bg-amber-900/35 dark:text-amber-200 ${className}`}
      title={tooltip}
      aria-label={tooltip}
    >
      <span aria-hidden="true">i</span>
      {label}
    </span>
  );
}
