import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { apiClient } from "../api/client";
import { formatQueryError } from "../api/httpError";
import { queryKeys } from "../api/queryKeys";
import type { BlastGraphNode } from "../api/types";
import { BlastExplorerCanvas } from "../components/blast/BlastExplorerCanvas";
import { PageHeader } from "../components/PageHeader";
import { StatePanel } from "../components/StatePanel";
import { useScanContext } from "../hooks/useScanContext";

type BlastModeQuery = "blast_radius" | "attack_path";

function modeFrom(s: string | null): BlastModeQuery {
  return s === "attack_path" ? "attack_path" : "blast_radius";
}

export function BlastExplorerPage() {
  const [sp, setSp] = useSearchParams();
  const { selectedScanId } = useScanContext();
  const scan = sp.get("scan") || selectedScanId || "";
  const finding = sp.get("finding") || "";
  const entity = sp.get("entity") || "";
  const principal = sp.get("principal") || "";
  const mode = modeFrom(sp.get("mode"));

  const [selectedNode, setSelectedNode] = useState<string | null>(null);

  const rootCount = Number(Boolean(finding)) + Number(Boolean(entity)) + Number(Boolean(principal));
  const byPrincipal = Boolean(principal);
  const byEntity = !byPrincipal && Boolean(entity);
  const byFinding = !byPrincipal && !byEntity && Boolean(finding);

  const q = useQuery({
    queryKey: byPrincipal
      ? queryKeys.principalBlastExplorer(scan, principal, mode)
      : byEntity
        ? queryKeys.entityBlastExplorer(scan, entity, mode)
        : queryKeys.blastExplorer(scan, finding, mode),
    queryFn: () =>
      byPrincipal
        ? apiClient.getPrincipalBlastExplorer(scan, principal, { mode })
        : byEntity
        ? apiClient.getEntityBlastExplorer(scan, entity, { mode })
        : apiClient.getBlastRadiusExplorer(scan, finding, { mode }),
    enabled: Boolean(scan && rootCount === 1 && (byPrincipal ? principal : byEntity ? entity : finding))
  });

  const payload = q.data;
  const nodes = payload?.nodes ?? [];
  const edges = payload?.edges ?? [];
  const selectedNodeObj = useMemo(
    () => nodes.find((n) => n.id === selectedNode) ?? null,
    [nodes, selectedNode]
  );
  const modeToggle = (next: BlastModeQuery) => {
    const n = new URLSearchParams(sp);
    n.set("mode", next);
    setSp(n);
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="Blast radius explorer"
        description="Focused operational reachability for one finding or external entity — not a full cloud graph."
      />
      {(!scan || rootCount === 0 || rootCount > 1) && (
        <StatePanel intent="empty" title="Choose a context">
          Open this view from a critical/high finding (Blast radius card) or pass{" "}
          <code className="rounded bg-slate-200 px-1 text-xs dark:bg-slate-800">?scan=…&amp;finding=…</code> or{" "}
          <code className="rounded bg-slate-200 px-1 text-xs dark:bg-slate-800">entity=…</code> or{" "}
          <code className="rounded bg-slate-200 px-1 text-xs dark:bg-slate-800">principal=…</code>.
        </StatePanel>
      )}

      {scan && rootCount === 1 && (
        <div className="flex flex-wrap items-center gap-2 text-sm">
          <span className="text-slate-500">Mode</span>
          <button
            type="button"
            className={`rounded px-2 py-1 text-xs font-medium ${
              mode === "blast_radius"
                ? "bg-cyan-500/20 text-cyan-900 dark:text-cyan-100"
                : "text-slate-500 hover:bg-slate-200 dark:hover:bg-slate-800"
            }`}
            onClick={() => modeToggle("blast_radius")}
          >
            Blast radius
          </button>
          <button
            type="button"
            className={`rounded px-2 py-1 text-xs font-medium ${
              mode === "attack_path"
                ? "bg-cyan-500/20 text-cyan-900 dark:text-cyan-100"
                : "text-slate-500 hover:bg-slate-200 dark:hover:bg-slate-800"
            }`}
            onClick={() => modeToggle("attack_path")}
          >
            Attack path
          </button>
          <Link to="/findings" className="ml-auto text-xs text-cyan-700 hover:underline dark:text-cyan-400">
            ← Back to findings
          </Link>
        </div>
      )}

      {q.isLoading && (
        <p className="text-sm text-slate-500" role="status">
          Loading curated subgraph…
        </p>
      )}
      {q.isError && (
        <StatePanel intent="error" title="Failed to load explorer">
          <pre className="whitespace-pre-wrap font-sans text-xs">{formatQueryError(q.error)}</pre>
        </StatePanel>
      )}

      {payload && (
        <div className="grid gap-4 lg:grid-cols-[1fr_320px]">
          <div>
            {payload.summary.graph_available ? null : (
              <p className="mb-2 rounded border border-amber-800/50 bg-amber-950/40 px-3 py-2 text-xs text-amber-200/90">
                Graph data unavailable (
                {payload.summary.graph_unavailable_reason || "neo4j_unconfigured"}). Summary below still applies for
                operational context.
              </p>
            )}
            <BlastExplorerCanvas
              data={{ ...payload, nodes, edges }}
              selectedNodeId={selectedNode}
              onSelectNode={(id) => {
                setSelectedNode(id);
              }}
            />
            <p className="mt-2 text-[11px] text-slate-500">
              Drag to rotate, scroll to zoom. Click a node to inspect. Unrelated items are de-emphasized in the
              layout.
            </p>
          </div>
          <aside className="space-y-3 rounded-lg border border-slate-200 bg-slate-50/50 p-4 text-sm dark:border-slate-800 dark:bg-slate-900/40">
            <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-500">Summary</h3>
            <p className="text-[11px] font-semibold uppercase tracking-wide text-cyan-700 dark:text-cyan-300">
              {payload.focus.root_type === "principal"
                ? "Principal Blast Radius"
                : payload.focus.root_type === "external_entity"
                  ? "External Entity Blast Radius"
                  : "Finding Blast Radius"}
            </p>
            <p className="text-[11px] text-slate-500">
              {payload.focus.root_type === "principal"
                ? payload.summary.source_principal_arn?.includes(":role/")
                  ? "Impact from IAM Role"
                  : "Impact from External Principal"
                : payload.focus.root_type === "external_entity"
                  ? "Impact from External Principal"
                  : "Impact from Finding Context"}
            </p>
            <p className="text-slate-800 dark:text-slate-200">{payload.summary.summary_text}</p>
            <ul className="list-inside list-disc text-xs text-slate-600 dark:text-slate-400">
              <li>Resources: {payload.summary.reachable_resource_count}</li>
              <li>Accounts: {payload.summary.reachable_accounts_count}</li>
              <li>Escalation: {payload.summary.escalation_possible ? "yes (trust/pivots or evidence)" : "not flagged"}</li>
            </ul>
            <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-500">Legend</h3>
            <dl className="grid grid-cols-[auto_1fr] gap-x-2 gap-y-1 text-[11px] text-slate-600 dark:text-slate-400">
              <dt className="font-mono text-[10px] text-cyan-700 dark:text-cyan-300">ASSUME_ROLE</dt>
              <dd>Trust-based principal pivot (role assumption or vendor trust).</dd>
              <dt className="font-mono text-[10px] text-cyan-700 dark:text-cyan-300">CROSS_ACCOUNT_ASSUME_ROLE</dt>
              <dd>Cross-account trust pivot between principals.</dd>
              <dt className="font-mono text-[10px] text-cyan-700 dark:text-cyan-300">EXTERNAL_TRUST</dt>
              <dd>External principal trust path into internal roles.</dd>
              <dt className="font-mono text-[10px] text-cyan-700 dark:text-cyan-300">IAM_WRITE</dt>
              <dd>IAM control-plane write/reconfiguration pivot path.</dd>
              <dt className="font-mono text-[10px] text-cyan-700 dark:text-cyan-300">RESOURCE_ACCESS</dt>
              <dd>Directed infra/resource reachability (route/fronting path).</dd>
              <dt className="font-mono text-[10px] text-cyan-700 dark:text-cyan-300">CERT_LINK</dt>
              <dd>TLS/certificate association context in the path.</dd>
            </dl>
            <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-500">Selection</h3>
            {selectedNodeObj ? <NodeDetail n={selectedNodeObj} /> : <p className="text-xs text-slate-500">No node selected.</p>}
            {!selectedNodeObj && edges.length > 0 ? (
              <p className="text-[11px] text-slate-500">
                Tip: select nodes in the 3D view. Edge list: {edges.length} (curated cap).
              </p>
            ) : null}
          </aside>
        </div>
      )}
    </div>
  );
}

function NodeDetail({ n }: { n: BlastGraphNode }) {
  return (
    <div className="rounded border border-slate-200 bg-white/80 p-2 text-xs dark:border-slate-700 dark:bg-slate-950/60">
      <p className="font-mono text-[10px] text-slate-500 break-all">{n.id}</p>
      <p className="mt-1 font-medium text-slate-800 dark:text-slate-100">{n.label}</p>
      <dl className="mt-1 grid grid-cols-2 gap-1 text-[11px] text-slate-600 dark:text-slate-400">
        <dt>type</dt>
        <dd>{n.type}</dd>
        {n.account_id ? (
          <>
            <dt>account</dt>
            <dd className="font-mono">{n.account_id}</dd>
          </>
        ) : null}
        <dt>path</dt>
        <dd>{n.is_critical_path ? "highlighted" : "context"}</dd>
      </dl>
    </div>
  );
}
