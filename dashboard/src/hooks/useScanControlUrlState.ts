import { useCallback, useMemo } from "react";
import { useSearchParams } from "react-router-dom";

const MODULE_OPTIONS = ["all", "orphaned_edge", "external_access"] as const;
const PROVIDER_OPTIONS = ["", "openai", "local"] as const;

export type ScanControlUrlState = {
  profile: string;
  moduleName: (typeof MODULE_OPTIONS)[number];
  provider: (typeof PROVIDER_OPTIONS)[number];
  noHTTP: boolean;
  neo4j: boolean;
};

function parseModule(raw: string | null): ScanControlUrlState["moduleName"] {
  if (MODULE_OPTIONS.includes((raw ?? "") as ScanControlUrlState["moduleName"])) {
    return (raw ?? "all") as ScanControlUrlState["moduleName"];
  }
  return "all";
}

function parseProvider(raw: string | null): ScanControlUrlState["provider"] {
  if (PROVIDER_OPTIONS.includes((raw ?? "") as ScanControlUrlState["provider"])) {
    return (raw ?? "") as ScanControlUrlState["provider"];
  }
  return "";
}

function parseBool(raw: string | null): boolean {
  return raw === "1" || raw === "true";
}

function parseState(sp: URLSearchParams): ScanControlUrlState {
  return {
    profile: sp.get("profile") ?? "",
    moduleName: parseModule(sp.get("module")),
    provider: parseProvider(sp.get("provider")),
    noHTTP: parseBool(sp.get("no_http")),
    neo4j: parseBool(sp.get("neo4j"))
  };
}

export function useScanControlUrlState() {
  const [searchParams, setSearchParams] = useSearchParams();
  const state = useMemo(() => parseState(searchParams), [searchParams]);

  const patch = useCallback(
    (partial: Partial<ScanControlUrlState>) => {
      const next = { ...state, ...partial };
      const merged = new URLSearchParams(searchParams);
      const managedKeys = ["profile", "module", "provider", "no_http", "neo4j"];
      for (const key of managedKeys) {
        merged.delete(key);
      }
      if (next.profile.trim()) {
        merged.set("profile", next.profile.trim());
      }
      if (next.moduleName !== "all") {
        merged.set("module", next.moduleName);
      }
      if (next.provider) {
        merged.set("provider", next.provider);
      }
      if (next.noHTTP) {
        merged.set("no_http", "1");
      }
      if (next.neo4j) {
        merged.set("neo4j", "1");
      }
      if (merged.toString() !== searchParams.toString()) {
        setSearchParams(merged, { replace: true });
      }
    },
    [searchParams, setSearchParams, state]
  );

  return { state, patch, searchParams };
}
