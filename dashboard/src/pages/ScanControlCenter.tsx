import { useEffect, useMemo, useState, type ReactNode } from "react";
import { formatQueryError } from "../api/httpError";
import { PageHeader } from "../components/PageHeader";
import { StatePanel } from "../components/StatePanel";
import { useRuntimeStatusQuery, useScanRunHistoryQuery, useScanRunStatusQuery, useStartScanMutation, useValidateProfileMutation } from "../hooks/useDashboardQueries";
import { useScanContext } from "../hooks/useScanContext";

const MODULE_OPTIONS = ["all", "orphaned_edge", "external_access"] as const;
const PROVIDER_OPTIONS = ["", "openai", "local"] as const;

function statusTone(ok: boolean): string {
  return ok
    ? "border-emerald-600/60 bg-emerald-900/20 text-emerald-200"
    : "border-slate-700 bg-slate-900/60 text-slate-300";
}

export function ScanControlCenterPage() {
  const { selectedScanId } = useScanContext();
  const runtime = useRuntimeStatusQuery();
  const scanStatus = useScanRunStatusQuery();
  const history = useScanRunHistoryQuery();
  const validateProfile = useValidateProfileMutation();
  const startScan = useStartScanMutation();

  const [profile, setProfile] = useState("");
  const [moduleName, setModuleName] = useState<(typeof MODULE_OPTIONS)[number]>("all");
  const [provider, setProvider] = useState<(typeof PROVIDER_OPTIONS)[number]>("");
  const [noHTTP, setNoHTTP] = useState(false);
  const [neo4j, setNeo4j] = useState(false);
  const [progressMessage, setProgressMessage] = useState("");

  useEffect(() => {
    if (!runtime.data) {
      return;
    }
    setProfile((prev) => prev || runtime.data.default_profile || runtime.data.aws_profiles[0] || "");
  }, [runtime.data]);

  useEffect(() => {
    const protocol = window.location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(`${protocol}://${window.location.host}/api/scan/progress`);
    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data) as { message?: string; stage?: string };
        setProgressMessage(msg.message || msg.stage || "");
      } catch {
        // Ignore malformed websocket payloads.
      }
    };
    return () => ws.close();
  }, []);

  const isRunning = scanStatus.data?.status === "running";
  const effectiveStatus = useMemo(() => {
    if (progressMessage.trim()) {
      return progressMessage;
    }
    return scanStatus.data?.message || "idle";
  }, [progressMessage, scanStatus.data?.message]);

  return (
    <section className="space-y-6">
      <PageHeader
        title="Scan Control Center"
        description="Start scans from the dashboard using profile/provider selections only. Secrets are resolved server-side from env/shared config/role sources."
        scanId={selectedScanId}
      />

      {runtime.isLoading ? (
        <StatePanel>Loading runtime status…</StatePanel>
      ) : runtime.isError ? (
        <StatePanel intent="error" title="Failed to load runtime status">
          <pre className="whitespace-pre-wrap font-sans text-xs">{formatQueryError(runtime.error)}</pre>
        </StatePanel>
      ) : runtime.data ? (
        <>
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
            <Badge title="OpenAI" ok={runtime.data.openai_configured} />
            <Badge title="Neo4j" ok={runtime.data.neo4j_configured} />
            <Badge title="Slack alerts" ok={runtime.data.slack_configured} />
            <Badge title="Email alerts" ok={runtime.data.email_configured} />
            <Badge title="AWS profiles" ok={runtime.data.aws_profiles.length > 0} detail={`${runtime.data.aws_profiles.length} found`} />
          </div>
          <p className="text-xs text-slate-500">
            If no local profiles are listed, ambient AWS auth may still work (instance role, container/task role, or env-based credentials).
          </p>

          <div className="rounded-lg border border-slate-200 bg-slate-50/90 p-4 dark:border-slate-800 dark:bg-slate-900/50">
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <Field label="AWS profile">
                <select
                  value={profile}
                  onChange={(e) => setProfile(e.target.value)}
                  className="w-full rounded-md border border-slate-700 bg-white px-2 py-2 text-sm text-slate-800 dark:bg-slate-950 dark:text-slate-200"
                >
                  <option value="">(ambient default chain)</option>
                  {runtime.data.aws_profiles.map((p) => (
                    <option key={p} value={p}>
                      {p}
                    </option>
                  ))}
                </select>
              </Field>

              <Field label="Module">
                <select
                  value={moduleName}
                  onChange={(e) => setModuleName(e.target.value as (typeof MODULE_OPTIONS)[number])}
                  className="w-full rounded-md border border-slate-700 bg-white px-2 py-2 text-sm text-slate-800 dark:bg-slate-950 dark:text-slate-200"
                >
                  {MODULE_OPTIONS.map((m) => (
                    <option key={m} value={m}>
                      {m}
                    </option>
                  ))}
                </select>
              </Field>

              <Field label="Embedding provider (optional)">
                <select
                  value={provider}
                  onChange={(e) => setProvider(e.target.value as (typeof PROVIDER_OPTIONS)[number])}
                  className="w-full rounded-md border border-slate-700 bg-white px-2 py-2 text-sm text-slate-800 dark:bg-slate-950 dark:text-slate-200"
                >
                  {PROVIDER_OPTIONS.map((p) => (
                    <option key={p || "default"} value={p}>
                      {p || "default"}
                    </option>
                  ))}
                </select>
              </Field>

              <Field label="Flags">
                <label className="flex items-center gap-2 text-sm text-slate-700 dark:text-slate-300">
                  <input type="checkbox" checked={noHTTP} onChange={(e) => setNoHTTP(e.target.checked)} />
                  no-http
                </label>
                <label className="mt-2 flex items-center gap-2 text-sm text-slate-700 dark:text-slate-300">
                  <input
                    type="checkbox"
                    checked={neo4j}
                    onChange={(e) => setNeo4j(e.target.checked)}
                    disabled={!runtime.data.neo4j_configured}
                  />
                  neo4j export
                </label>
              </Field>
            </div>

            <div className="mt-4 flex flex-wrap items-center gap-3">
              <button
                type="button"
                onClick={() => validateProfile.mutate(profile)}
                disabled={validateProfile.isPending}
                className="rounded-md border border-cyan-700 bg-cyan-900/25 px-3 py-2 text-sm text-cyan-100 disabled:opacity-50"
              >
                Validate profile
              </button>
              <button
                type="button"
                onClick={() => startScan.mutate({ profile, module: moduleName, provider: provider || undefined, no_http: noHTTP, neo4j: neo4j })}
                disabled={isRunning || startScan.isPending}
                className="rounded-md border border-emerald-700 bg-emerald-900/25 px-3 py-2 text-sm text-emerald-100 disabled:opacity-50"
              >
                {isRunning ? "Scan running…" : "Start scan"}
              </button>
            </div>
            <p className="mt-2 text-[11px] text-slate-500">
              Limitation: this control center currently supports a single active run at a time (latest run state is shared across tabs/users).
            </p>

            {validateProfile.data ? (
              <p className={`mt-3 text-sm ${validateProfile.data.ok ? "text-emerald-300" : "text-amber-300"}`}>
                {validateProfile.data.message}
              </p>
            ) : null}
            {validateProfile.isError ? (
              <p className="mt-3 text-sm text-rose-300">{formatQueryError(validateProfile.error)}</p>
            ) : null}
            {startScan.isSuccess ? <p className="mt-3 text-sm text-emerald-300">{startScan.data.message}</p> : null}
            {startScan.isError ? <p className="mt-3 text-sm text-rose-300">{formatQueryError(startScan.error)}</p> : null}
          </div>

          <div className="rounded-lg border border-slate-200 bg-white/90 p-4 dark:border-slate-800 dark:bg-slate-900/80">
            <h3 className="text-sm font-semibold text-slate-800 dark:text-slate-100">Run status</h3>
            <dl className="mt-3 grid gap-2 text-sm md:grid-cols-2">
              <Stat label="State" value={scanStatus.data?.status || "idle"} />
              <Stat label="Stage" value={scanStatus.data?.stage || "idle"} />
              <Stat label="Profile" value={scanStatus.data?.profile || profile || "—"} />
              <Stat label="Module" value={scanStatus.data?.module || moduleName} />
              <Stat label="Scan ID" value={scanStatus.data?.scan_id || "—"} />
              <Stat label="Message" value={effectiveStatus || "—"} />
            </dl>
          </div>

          <div className="rounded-lg border border-slate-200 bg-white/90 p-4 dark:border-slate-800 dark:bg-slate-900/80">
            <h3 className="text-sm font-semibold text-slate-800 dark:text-slate-100">Recent runs</h3>
            {history.isLoading ? (
              <p className="mt-3 text-sm text-slate-500">Loading recent runs…</p>
            ) : history.isError ? (
              <p className="mt-3 text-sm text-rose-300">{formatQueryError(history.error)}</p>
            ) : (history.data?.items.length ?? 0) === 0 ? (
              <p className="mt-3 text-sm text-slate-500">No recent runs recorded yet.</p>
            ) : (
              <div className="mt-3 overflow-x-auto">
                <table className="w-full min-w-[52rem] border-collapse text-left text-xs">
                  <thead className="border-b border-slate-200 dark:border-slate-800">
                    <tr className="text-slate-500">
                      <th className="px-2 py-2 font-semibold uppercase tracking-wide">Run</th>
                      <th className="px-2 py-2 font-semibold uppercase tracking-wide">Status</th>
                      <th className="px-2 py-2 font-semibold uppercase tracking-wide">Profile</th>
                      <th className="px-2 py-2 font-semibold uppercase tracking-wide">Module</th>
                      <th className="px-2 py-2 font-semibold uppercase tracking-wide">Flags</th>
                      <th className="px-2 py-2 font-semibold uppercase tracking-wide">Started</th>
                      <th className="px-2 py-2 font-semibold uppercase tracking-wide">Finished</th>
                      <th className="px-2 py-2 font-semibold uppercase tracking-wide">Message</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-200 dark:divide-slate-800/90">
                    {history.data?.items.map((item) => (
                      <tr key={item.run_id} className="text-slate-700 dark:text-slate-300">
                        <td className="px-2 py-2 font-mono">{item.run_id}</td>
                        <td className="px-2 py-2">
                          <span
                            className={`rounded px-1.5 py-0.5 text-[11px] ${
                              item.status === "completed"
                                ? "bg-emerald-900/40 text-emerald-200"
                                : item.status === "failed"
                                ? "bg-rose-900/40 text-rose-200"
                                : "bg-slate-800 text-slate-200"
                            }`}
                          >
                            {item.status}
                          </span>
                        </td>
                        <td className="px-2 py-2 font-mono">{item.profile || "ambient"}</td>
                        <td className="px-2 py-2">{item.module || "all"}</td>
                        <td className="px-2 py-2">
                          {item.no_http ? "no-http" : "http"}
                          {item.neo4j ? ", neo4j" : ""}
                        </td>
                        <td className="px-2 py-2 font-mono">{toShortTs(item.started_at)}</td>
                        <td className="px-2 py-2 font-mono">{toShortTs(item.finished_at)}</td>
                        <td className="px-2 py-2">{item.message}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </>
      ) : null}
    </section>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <label className="mb-1 block text-[11px] font-medium uppercase tracking-wide text-slate-500">{label}</label>
      {children}
    </div>
  );
}

function Badge({ title, ok, detail }: { title: string; ok: boolean; detail?: string }) {
  return (
    <div className={`rounded-lg border px-3 py-2 text-sm ${statusTone(ok)}`}>
      <p className="font-medium">{title}</p>
      <p className="text-xs opacity-90">{detail || (ok ? "Configured" : "Not configured")}</p>
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-xs text-slate-500">{label}</dt>
      <dd className="mt-0.5 font-mono text-xs text-slate-800 dark:text-slate-200 break-all">{value}</dd>
    </div>
  );
}

function toShortTs(ts?: string): string {
  if (!ts) {
    return "—";
  }
  const d = new Date(ts);
  if (Number.isNaN(d.getTime())) {
    return "—";
  }
  return d.toISOString().replace("T", " ").replace("Z", "Z");
}
