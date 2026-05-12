/**
 * Human-readable labels for trust evidence fields (detail / trust blocks).
 */

export function formatDaysSinceUsedLabel(days: number | undefined | null): string {
  if (days === undefined || days === null) {
    return "Last used: unknown (no activity data)";
  }
  if (days < 0) {
    return "Last used: unknown";
  }
  if (days === 0) {
    return "Used recently (same day)";
  }
  if (days < 90) {
    return `Active within ~90 days (${days}d since use)`;
  }
  if (days <= 365) {
    return `Idle 90 days – 1 year (${days}d since use)`;
  }
  return `Idle over 1 year (${days}d since use)`;
}

/** Returns a Tailwind color class for the Last Used value based on staleness. */
export function daysSinceUsedColorClass(days: number | undefined | null): string {
  if (days === undefined || days === null || days < 0) {
    return "text-slate-500 dark:text-slate-400";
  }
  if (days < 90) {
    return "text-emerald-700 dark:text-emerald-400";
  }
  if (days <= 365) {
    return "text-amber-700 dark:text-amber-400";
  }
  return "text-red-700 dark:text-red-400";
}

export function formatAdminEvalStateLabel(raw: string | undefined | null): string {
  const s = raw?.trim();
  if (!s) {
    return "Not evaluated";
  }
  return s;
}
