import { useMutation, useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { apiClient } from "../api/client";
import { formatQueryError } from "../api/httpError";
import { queryKeys } from "../api/queryKeys";
import type { BlastGraphEdge, BlastGraphNode } from "../api/types";
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
  const [selectedPathId, setSelectedPathId] = useState<string | null>(null);
  const [graphNodes, setGraphNodes] = useState<BlastGraphNode[]>([]);
  const [graphEdges, setGraphEdges] = useState<BlastGraphEdge[]>([]);
  const [expandedNodeIds, setExpandedNodeIds] = useState<string[]>([]);
  const [expandedEdgeIds, setExpandedEdgeIds] = useState<string[]>([]);
  const [expandedFromNodeIds, setExpandedFromNodeIds] = useState<string[]>([]);

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
  const nodes = graphNodes;
  const edges = graphEdges;
  const pathVariants = payload?.path_variants ?? [];
  const showPathSwitcher = mode === "attack_path" && pathVariants.length > 1;
  const activePath =
    pathVariants.find((p) => p.id === selectedPathId) ??
    pathVariants.find((p) => p.id === payload?.selected_path_id) ??
    pathVariants[0] ??
    null;

  useEffect(() => {
    if (!payload) {
      setSelectedPathId(null);
      return;
    }
    setGraphNodes(payload.nodes ?? []);
    setGraphEdges(payload.edges ?? []);
    setExpandedNodeIds([]);
    setExpandedEdgeIds([]);
    setExpandedFromNodeIds([]);
    if (payload.selected_path_id) {
      setSelectedPathId(payload.selected_path_id);
      return;
    }
    setSelectedPathId(payload.path_variants?.[0]?.id ?? null);
  }, [payload]);

  const expandMutation = useMutation({
    mutationFn: async (nodeID: string) => {
      const rootParams = byPrincipal
        ? { principal_id: principal }
        : byEntity
          ? { entity_id: entity }
          : { finding_id: finding };
      return apiClient.getBlastExplorerExpansion(scan, {
        node_id: nodeID,
        mode,
        ...rootParams
      });
    },
    onSuccess: (delta) => {
      if (!delta.expansion_applied) {
        return;
      }
      const newNodes = delta.nodes ?? [];
      const newEdges = delta.edges ?? [];
      setGraphNodes((prev) => {
        const seen = new Set(prev.map((n) => n.id));
        const add = newNodes.filter((n) => !seen.has(n.id));
        return add.length ? [...prev, ...add] : prev;
      });
      setGraphEdges((prev) => {
        const seen = new Set(prev.map((e) => e.id));
        const add = newEdges.filter((e) => !seen.has(e.id));
        return add.length ? [...prev, ...add] : prev;
      });
      setExpandedNodeIds((prev) => {
        const merged = new Set(prev);
        for (const n of newNodes) {
          merged.add(n.id);
        }
        return Array.from(merged);
      });
      setExpandedEdgeIds((prev) => {
        const merged = new Set(prev);
        for (const e of newEdges) {
          merged.add(e.id);
        }
        return Array.from(merged);
      });
      setExpandedFromNodeIds((prev) => {
        const merged = new Set(prev);
        merged.add(delta.expanded_from_node_id);
        return Array.from(merged);
      });
    }
  });

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
              selectedPathNodeIDs={activePath?.node_ids ?? null}
              selectedPathEdgeIDs={activePath?.edge_ids ?? null}
              expandedNodeIDs={expandedNodeIds}
              expandedEdgeIDs={expandedEdgeIds}
              onSelectNode={(id) => {
                setSelectedNode(id);
              }}
            />
            <p className="mt-2 text-[11px] text-slate-500">
              Drag to rotate, scroll to zoom. Click a node to inspect. Unrelated items are de-emphasized in the
              layout.
            </p>
          </div>
          <aside className="min-w-0 space-y-3 overflow-hidden rounded-lg border border-slate-200 bg-slate-50/50 p-4 text-sm dark:border-slate-800 dark:bg-slate-900/40">
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
            <p className="break-words text-slate-800 dark:text-slate-200">{payload.summary.summary_text}</p>
            {mode === "attack_path" && pathVariants.length > 1 ? (
              <p className="text-xs text-cyan-700 dark:text-cyan-300">
                {pathVariants.length - 1} alternate path{pathVariants.length - 1 > 1 ? "s" : ""} available
                {pathVariants.some((v) => v.dominant_semantics?.includes("CROSS_ACCOUNT_ASSUME_ROLE"))
                  ? "; strongest alternate includes cross-account trust."
                  : "."}
              </p>
            ) : null}
            <ul className="list-inside list-disc text-xs text-slate-600 dark:text-slate-400">
              <li>Resources: {payload.summary.reachable_resource_count}</li>
              <li>Accounts: {payload.summary.reachable_accounts_count}</li>
              <li>Escalation: {payload.summary.escalation_possible ? "yes (trust/pivots or evidence)" : "not flagged"}</li>
            </ul>
            {showPathSwitcher ? (
              <div className="space-y-2 rounded border border-cyan-500/30 bg-cyan-500/5 p-2">
                <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-500">Chain view</h3>
                <div className="flex flex-wrap gap-1.5">
                  {pathVariants.map((variant) => {
                    const active = activePath?.id === variant.id;
                    return (
                      <button
                        key={variant.id}
                        type="button"
                        onClick={() => setSelectedPathId(variant.id)}
                        className={`rounded px-2 py-1 text-[11px] font-medium ${
                          active
                            ? "bg-cyan-500/25 text-cyan-900 dark:text-cyan-100"
                            : "bg-slate-200/70 text-slate-700 hover:bg-slate-300 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
                        }`}
                      >
                        {variant.label}
                      </button>
                    );
                  })}
                </div>
                {activePath ? (
                  <p className="text-[11px] text-slate-600 dark:text-slate-400">
                    {activePath.summary}
                    {activePath.risk_hint ? ` ${activePath.risk_hint}` : ""}
                  </p>
                ) : null}
              </div>
            ) : null}
            <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-500">Legend</h3>
            <dl className="space-y-1.5 text-[11px] text-slate-600 dark:text-slate-400">
              {(
                [
                  ["ASSUME_ROLE", "Trust-based principal pivot (role assumption or vendor trust)."],
                  ["CROSS_ACCOUNT_ASSUME_ROLE", "Cross-account trust pivot between principals."],
                  ["EXTERNAL_TRUST", "External principal trust path into internal roles."],
                  ["IAM_WRITE", "IAM control-plane write/reconfiguration pivot path."],
                  ["RESOURCE_ACCESS", "Directed infra/resource reachability (route/fronting path)."],
                  ["CERT_LINK", "TLS/certificate association context in the path."],
                ] as [string, string][]
              ).map(([label, desc]) => (
                <div key={label}>
                  <dt className="font-mono text-[10px] text-cyan-700 dark:text-cyan-300">{label}</dt>
                  <dd className="ml-0 text-slate-600 dark:text-slate-400">{desc}</dd>
                </div>
              ))}
            </dl>
            <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-500">Selection</h3>
            {selectedNodeObj ? <NodeDetail n={selectedNodeObj} /> : <p className="text-xs text-slate-500">No node selected.</p>}
            {selectedNodeObj && payload.summary.graph_available ? (
              <div className="space-y-1">
                <button
                  type="button"
                  className="rounded border border-slate-300 px-2 py-1 text-[11px] font-medium text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
                  disabled={
                    expandMutation.isPending ||
                    expandedFromNodeIds.includes(selectedNodeObj.id) ||
                    selectedNodeObj.type === "finding"
                  }
                  onClick={() => {
                    if (!expandedFromNodeIds.includes(selectedNodeObj.id)) {
                      expandMutation.mutate(selectedNodeObj.id);
                    }
                  }}
                >
                  {expandedFromNodeIds.includes(selectedNodeObj.id)
                    ? "1-hop expansion added"
                    : expandMutation.isPending
                      ? "Expanding…"
                      : "Expand 1 hop"}
                </button>
                {expandMutation.isSuccess && !expandMutation.data.expansion_applied ? (
                  <p className="text-[11px] text-slate-500">
                    {expandMutation.data.expansion_reason === "no_additional_high_signal_neighbors"
                      ? "No additional high-signal neighbors."
                      : "No additional expansion available."}
                  </p>
                ) : null}
              </div>
            ) : null}
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
    <div className="min-w-0 rounded border border-slate-200 bg-white/80 p-2 text-xs dark:border-slate-700 dark:bg-slate-950/60">
      <p className="break-all font-mono text-[10px] text-slate-500">{n.id}</p>
      <p className="mt-1 break-words font-medium text-slate-800 dark:text-slate-100">{n.label}</p>
      <dl className="mt-1 space-y-0.5 text-[11px] text-slate-600 dark:text-slate-400">
        <div className="flex gap-2">
          <dt className="shrink-0 text-slate-400">type</dt>
          <dd>{n.type}</dd>
        </div>
        {n.account_id ? (
          <div className="flex gap-2">
            <dt className="shrink-0 text-slate-400">account</dt>
            <dd className="font-mono">{n.account_id}</dd>
          </div>
        ) : null}
        <div className="flex gap-2">
          <dt className="shrink-0 text-slate-400">path</dt>
          <dd>{n.is_critical_path ? "highlighted" : "context"}</dd>
        </div>
      </dl>
    </div>
  );
}
