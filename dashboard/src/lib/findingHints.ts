import type { FindingListItem } from "../api/types";

/**
 * Short operator-facing hint from list-row fields only (no detail fetch).
 */
export function findingInlineHint(item: FindingListItem): string | null {
  const title = item.title?.toLowerCase() ?? "";
  const mod = item.module?.toLowerCase() ?? "";
  const claim = item.claimability?.toLowerCase() ?? "";

  if (mod === "orphaned_edge") {
    if (claim === "reclaimable") {
      return "Reclaimable endpoint";
    }
    if (claim === "dangling") {
      return "Dangling endpoint";
    }
    if (claim === "broken") {
      return "Broken / unreachable endpoint";
    }
    if (claim === "edge_obscured") {
      return "Edge obscured";
    }
  }

  if (mod === "external_access") {
    if (/\bnever used\b|\bnot used\b|\bunused role\b/i.test(item.title ?? "")) {
      return "Never used role";
    }
    if (/\bnot approved\b|\bunapproved\b|\bnon-approved\b|\boutside approved\b|\buntrusted account\b/i.test(item.title ?? "")) {
      return "External account not approved";
    }
    if (/\bapproved\b|\bwhitelisted\b|\btrusted account\b/i.test(item.title ?? "")) {
      return "Approved / known trust pattern";
    }
    return "Cross-account trust";
  }

  if (claim === "reclaimable") {
    return "Reclaimable resource";
  }

  return null;
}
