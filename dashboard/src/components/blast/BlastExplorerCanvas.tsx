import { Canvas } from "@react-three/fiber";
import { Line, OrbitControls } from "@react-three/drei";
import { useMemo, useState } from "react";
import * as THREE from "three";
import type { BlastExplorerResponse, BlastGraphEdge, BlastGraphNode } from "../../api/types";

function sphereLayout(nodes: BlastGraphNode[], focusHint?: string): Map<string, THREE.Vector3> {
  const pos = new Map<string, THREE.Vector3>();
  const n = nodes.length;
  const r = 5.5;
  nodes.forEach((node, i) => {
    const t = (i + 0.5) / Math.max(n, 1);
    const phi = Math.acos(2 * t - 1);
    const theta = Math.PI * (1 + Math.sqrt(5)) * i;
    pos.set(
      node.id,
      new THREE.Vector3(
        r * Math.sin(phi) * Math.cos(theta),
        r * Math.sin(phi) * Math.sin(theta),
        r * Math.cos(phi)
      )
    );
  });
  const focus =
    (focusHint && pos.has(focusHint) && focusHint) ||
    nodes.find((x) => x.is_focus)?.id ||
    nodes[0]?.id;
  if (focus && pos.has(focus)) {
    const c = pos.get(focus)!.clone();
    for (const [id, p] of pos) {
      pos.set(id, p.clone().sub(c));
    }
  }
  return pos;
}

function radialBlastLayout(
  nodes: BlastGraphNode[],
  focusHint?: string
): Map<string, THREE.Vector3> {
  const pos = new Map<string, THREE.Vector3>();
  if (nodes.length === 0) {
    return pos;
  }
  const focus =
    nodes.find((n) => n.id === focusHint)?.id ||
    nodes.find((n) => n.is_focus)?.id ||
    nodes[0]?.id;
  if (!focus) {
    return pos;
  }
  pos.set(focus, new THREE.Vector3(0, 0, 0));
  const critical = nodes.filter((n) => n.id !== focus && n.is_critical_path);
  const context = nodes.filter((n) => n.id !== focus && !n.is_critical_path);
  const placeRing = (arr: BlastGraphNode[], radius: number, z: number) => {
    arr.forEach((n, i) => {
      const theta = (i / Math.max(arr.length, 1)) * Math.PI * 2;
      pos.set(n.id, new THREE.Vector3(radius * Math.cos(theta), radius * Math.sin(theta), z));
    });
  };
  placeRing(critical, 4.4, 0.8);
  placeRing(context, 8.2, -1.4);
  return pos;
}

function layeredAttackPathLayout(
  nodes: BlastGraphNode[],
  edges: BlastGraphEdge[],
  focusHint?: string
): Map<string, THREE.Vector3> {
  const pos = new Map<string, THREE.Vector3>();
  if (nodes.length === 0) {
    return pos;
  }
  const focus =
    nodes.find((n) => n.id === focusHint)?.id ||
    nodes.find((n) => n.is_focus)?.id ||
    nodes[0]?.id;
  if (!focus) {
    return pos;
  }
  const depth = new Map<string, number>();
  depth.set(focus, 0);
  const q = [focus];
  while (q.length > 0) {
    const cur = q.shift() as string;
    const d = depth.get(cur) ?? 0;
    for (const e of edges) {
      if (e.source !== cur) {
        continue;
      }
      if (!depth.has(e.target)) {
        depth.set(e.target, d + 1);
        q.push(e.target);
      }
    }
  }
  const layers = new Map<number, BlastGraphNode[]>();
  for (const n of nodes) {
    const d = depth.get(n.id) ?? 3;
    const arr = layers.get(d) ?? [];
    arr.push(n);
    layers.set(d, arr);
  }
  for (const [d, arr] of layers) {
    arr.sort((a, b) => Number(b.is_critical_path) - Number(a.is_critical_path));
    arr.forEach((n, i) => {
      const spread = (arr.length - 1) * 1.2;
      const y = i * 1.2 - spread / 2;
      pos.set(n.id, new THREE.Vector3(d * 3.1 - 1.5, y, 0));
    });
  }
  return pos;
}

function nodeColor(n: BlastGraphNode): string {
  if (n.type === "finding") {
    return "#ea580c";
  }
  if (n.is_external) {
    return "#8b5cf6";
  }
  if (n.type === "account") {
    return "#64748b";
  }
  if (n.is_critical_path) {
    return "#f97316";
  }
  return "#94a3b8";
}

function EdgeLine({
  e,
  a,
  b,
  selectedPathEdgeIDs,
  expandedEdgeIDs
}: {
  e: BlastGraphEdge;
  a: THREE.Vector3;
  b: THREE.Vector3;
  selectedPathEdgeIDs: Set<string> | null;
  expandedEdgeIDs: Set<string> | null;
}) {
  const inSelectedPath = selectedPathEdgeIDs ? selectedPathEdgeIDs.has(e.id) : false;
  const isExpanded = expandedEdgeIDs ? expandedEdgeIDs.has(e.id) : false;
  const crit = selectedPathEdgeIDs ? inSelectedPath : e.is_critical_path;
  const muted = selectedPathEdgeIDs ? !inSelectedPath : !e.is_critical_path;
  return (
    <Line
      points={[a, b]}
      color={crit ? "#38bdf8" : isExpanded ? "#67e8f9" : "#475569"}
      lineWidth={crit ? 2.2 : isExpanded ? 1.4 : 1}
      transparent
      opacity={crit ? 0.95 : muted ? 0.18 : 0.32}
    />
  );
}

function NodeBall({
  n,
  position,
  onSelect,
  selected,
  selectedPathNodeIDs,
  expandedNodeIDs
}: {
  n: BlastGraphNode;
  position: THREE.Vector3;
  onSelect: (id: string) => void;
  selected: boolean;
  selectedPathNodeIDs: Set<string> | null;
  expandedNodeIDs: Set<string> | null;
}) {
  const inSelectedPath = selectedPathNodeIDs ? selectedPathNodeIDs.has(n.id) : false;
  const isExpanded = expandedNodeIDs ? expandedNodeIDs.has(n.id) : false;
  const dim = selectedPathNodeIDs
    ? !inSelectedPath && !n.is_focus && !selected
    : !n.is_critical_path && !n.is_focus && !selected;
  const col = nodeColor(n);
  const [hover, setHover] = useState(false);
  return (
    <mesh
      position={position}
      onClick={(ev) => {
        ev.stopPropagation();
        onSelect(n.id);
      }}
      onPointerOver={() => setHover(true)}
      onPointerOut={() => setHover(false)}
    >
      <sphereGeometry args={[n.type === "finding" ? 0.48 : 0.34, 20, 20]} />
      <meshStandardMaterial
        color={col}
        emissive={hover || selected || isExpanded ? col : "#000000"}
        emissiveIntensity={hover || selected ? 0.35 : isExpanded ? 0.14 : 0}
        transparent
        opacity={dim ? 0.3 : isExpanded ? 0.98 : 0.96}
      />
    </mesh>
  );
}

export type BlastExplorerCanvasProps = {
  data: BlastExplorerResponse;
  selectedNodeId: string | null;
  selectedPathNodeIDs: string[] | null;
  selectedPathEdgeIDs: string[] | null;
  expandedNodeIDs: string[];
  expandedEdgeIDs: string[];
  onSelectNode: (id: string) => void;
};

export function BlastExplorerCanvas({
  data,
  selectedNodeId,
  selectedPathNodeIDs,
  selectedPathEdgeIDs,
  expandedNodeIDs,
  expandedEdgeIDs,
  onSelectNode
}: BlastExplorerCanvasProps) {
  const nodes = data.nodes ?? [];
  const edges = data.edges ?? [];
  const focusId = data.display.default_focus_id || nodes.find((x) => x.is_focus)?.id;
  const blastMode = data.focus?.blast_mode ?? "blast_radius";
  const selectedPathNodeSet = useMemo(
    () => (selectedPathNodeIDs && selectedPathNodeIDs.length > 0 ? new Set(selectedPathNodeIDs) : null),
    [selectedPathNodeIDs]
  );
  const selectedPathEdgeSet = useMemo(
    () => (selectedPathEdgeIDs && selectedPathEdgeIDs.length > 0 ? new Set(selectedPathEdgeIDs) : null),
    [selectedPathEdgeIDs]
  );
  const expandedNodeSet = useMemo(() => new Set(expandedNodeIDs), [expandedNodeIDs]);
  const expandedEdgeSet = useMemo(() => new Set(expandedEdgeIDs), [expandedEdgeIDs]);
  const layout = useMemo(
    () => {
      if (blastMode === "attack_path") {
        const layered = layeredAttackPathLayout(nodes, edges, focusId);
        return layered.size > 0 ? layered : sphereLayout(nodes, focusId);
      }
      const radial = radialBlastLayout(nodes, focusId);
      return radial.size > 0 ? radial : sphereLayout(nodes, focusId);
    },
    [nodes, edges, focusId, blastMode]
  );

  if (nodes.length === 0) {
    return (
      <div className="flex h-[320px] max-h-[56vh] min-h-[280px] items-center justify-center rounded-lg border border-slate-700/80 bg-slate-950 text-sm text-slate-500">
        No graph nodes in this view (Neo4j off or no projection).
      </div>
    );
  }

  return (
    <div className="h-[380px] max-h-[56vh] min-h-[300px] w-full overflow-hidden rounded-lg border border-slate-700/80 bg-slate-950">
      <Canvas
        camera={{ position: [0, 0, 16], fov: 48, near: 0.1, far: 200 }}
        dpr={[1, 2]}
        className="touch-none"
      >
        <color attach="background" args={["#020617"]} />
        <ambientLight intensity={0.5} />
        <directionalLight position={[6, 10, 4]} intensity={0.85} />
        {edges.map((e) => {
          const a = layout.get(e.source);
          const b = layout.get(e.target);
          if (!a || !b) {
            return null;
          }
          return (
            <EdgeLine
              key={e.id}
              e={e}
              a={a}
              b={b}
              selectedPathEdgeIDs={selectedPathEdgeSet}
              expandedEdgeIDs={expandedEdgeSet}
            />
          );
        })}
        {nodes.map((n) => {
          const p = layout.get(n.id);
          if (!p) {
            return null;
          }
          return (
            <NodeBall
              key={n.id}
              n={n}
              position={p}
              onSelect={onSelectNode}
              selected={selectedNodeId === n.id}
              selectedPathNodeIDs={selectedPathNodeSet}
              expandedNodeIDs={expandedNodeSet}
            />
          );
        })}
        <OrbitControls
          makeDefault
          enableDamping
          dampingFactor={0.08}
          minDistance={5}
          maxDistance={55}
        />
      </Canvas>
    </div>
  );
}
