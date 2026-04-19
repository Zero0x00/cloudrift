import type { TrustDisplay } from "../../api/types";
import { formatAdminEvalStateLabel } from "../../lib/trustLabels";

/**
 * Surfaces IAM activity semantics for auditors without implying “ghost admin” or other
 * verdicts not present on the finding. Copy aligns with backend evidence activity_status.
 */
export function TrustActivityCallout({ trust }: { trust: TrustDisplay | null | undefined }) {
  if (!trust) {
    return null;
  }

  const status = trust.activity_status?.trim();
  const adminLabel = formatAdminEvalStateLabel(trust.admin_eval_state);

  const lines: { key: string; body: string; intent: "info" | "warn" | "neutral" }[] = [];

  lines.push({
    key: "admin",
    body: `Admin evaluation: ${adminLabel}`,
    intent: "neutral"
  });

  if (status === "iam_never_used") {
    lines.push({
      key: "activity",
      body:
        "Activity: IAM indicates this role was never used (RoleLastUsed). This is confirmed inactivity from AWS telemetry — not missing join data.",
      intent: "info"
    });
  } else if (status === "missing_join") {
    lines.push({
      key: "activity",
      body:
        "Activity: No activity row was joined to this finding. Stale or elevated severity may be conservative; it is not the same as confirmed never-used. Verify RoleLastUsed visibility and collector coverage.",
      intent: "warn"
    });
  } else if (status) {
    lines.push({
      key: "activity",
      body: `Activity status: ${status}`,
      intent: "neutral"
    });
  }

  if (trust.unknown_vendor === true) {
    lines.push({
      key: "vendor",
      body: "Unknown vendor: external account could not be resolved confidently for this principal.",
      intent: "warn"
    });
  }

  if (lines.length === 0) {
    return null;
  }

  const box =
    "rounded-md border px-3 py-2 text-xs leading-relaxed ";
  const tone =
    lines.some((l) => l.intent === "warn")
      ? "border-amber-800/60 bg-amber-950/25 text-amber-100/95"
      : "border-slate-700 bg-slate-100/60 dark:bg-slate-950/60 text-slate-700 dark:text-slate-300";

  return (
    <div className={`${box} ${tone}`} role="note">
      <p className="mb-1 text-[11px] font-semibold uppercase tracking-wide text-slate-500">Trust audit notes</p>
      <ul className="list-inside list-disc space-y-1.5">
        {lines.map((l) => (
          <li key={l.key}>{l.body}</li>
        ))}
      </ul>
    </div>
  );
}
