import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { apiClient } from "../api/client";
import { ScanRequired } from "../components/ScanRequired";
import { StatePanel } from "../components/StatePanel";
import { useScanContext } from "../hooks/useScanContext";

export function QueryPage() {
  const { selectedScanId } = useScanContext();
  const [query, setQuery] = useState("");

  const runQuery = useMutation({
    mutationFn: async () => {
      if (!selectedScanId) {
        throw new Error("No scan selected");
      }
      return apiClient.queryInvestigation({ query, scan_id: selectedScanId });
    }
  });

  if (!selectedScanId) {
    return <ScanRequired />;
  }

  return (
    <div className="space-y-4">
      <header>
        <h1 className="text-xl font-semibold text-slate-900 dark:text-slate-100">Investigation Query</h1>
        <p className="mt-1 text-sm text-slate-600 dark:text-slate-300">
          Ask Cloudrift-native operational questions scoped to the selected scan.
        </p>
      </header>

      <section className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <label htmlFor="query-input" className="mb-2 block text-sm font-medium text-slate-700 dark:text-slate-200">
          Question
        </label>
        <textarea
          id="query-input"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          rows={3}
          className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm outline-none ring-cyan-500 focus:ring-2 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100"
          placeholder="e.g. What should I fix first in this scan?"
        />
        <div className="mt-3 flex items-center justify-between">
          <p className="text-xs text-slate-500 dark:text-slate-400">Scan scope: {selectedScanId}</p>
          <button
            type="button"
            disabled={runQuery.isPending || query.trim().length < 3}
            onClick={() => runQuery.mutate()}
            className="rounded-md bg-cyan-600 px-3 py-1.5 text-sm font-medium text-white disabled:cursor-not-allowed disabled:opacity-50"
          >
            {runQuery.isPending ? "Running..." : "Run query"}
          </button>
        </div>
      </section>

      {runQuery.isError ? (
        <StatePanel intent="error" title="Query failed">
          {(runQuery.error as Error).message}
        </StatePanel>
      ) : null}

      {runQuery.data ? (
        <section className="space-y-3 rounded-xl border border-slate-200 bg-white p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <div>
            <p className="text-xs uppercase tracking-wide text-slate-500 dark:text-slate-400">
              {runQuery.data.intent} · {runQuery.data.answer_type}
            </p>
            <p className="mt-1 text-sm text-slate-900 dark:text-slate-100">{runQuery.data.answer}</p>
          </div>
          <div className="grid gap-2 text-xs text-slate-600 dark:text-slate-300 sm:grid-cols-3">
            <div>Graph: {runQuery.data.graph_used ? "used" : "not used"}</div>
            <div>Semantic: {runQuery.data.semantic_used ? "used" : "not used"}</div>
            <div>Domain: {runQuery.data.domain_used ? "used" : "not used"}</div>
          </div>
          <div className="space-y-1">
            {runQuery.data.supporting_facts.map((fact, idx) => (
              <div key={`${fact.label}-${idx}`} className="text-xs text-slate-600 dark:text-slate-300">
                <span className="font-medium">{fact.label}:</span> {fact.value}{" "}
                <span className="text-slate-400">({fact.source})</span>
              </div>
            ))}
          </div>
        </section>
      ) : null}
    </div>
  );
}
