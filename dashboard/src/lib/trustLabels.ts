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

export function formatAdminEvalStateLabel(raw: string | undefined | null): string {
  const s = raw?.trim();
  if (!s) {
    return "Not evaluated";
  }
  return s;
}
