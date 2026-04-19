import type { ScanListItem } from "../api/types";

/**
 * Ordered scans from GET /api/scans: newest first, tie-break scan_id ascending.
 * "Previous" baseline for diff = the next row in this list (index + 1), i.e. the chronologically older scan.
 */
export function getPreviousScanIdForDiff(
  scansNewestFirst: ScanListItem[],
  selectedScanId: string | null
): string | null {
  if (!selectedScanId || scansNewestFirst.length < 2) {
    return null;
  }
  const idx = scansNewestFirst.findIndex((s) => s.scan_id === selectedScanId);
  if (idx < 0) {
    return null;
  }
  const older = scansNewestFirst[idx + 1];
  return older?.scan_id ?? null;
}
