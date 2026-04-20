import { useCallback, useMemo } from "react";
import { useSearchParams } from "react-router-dom";

export type DashboardViewId = "executive" | "high-signal" | "operations";

const VALID: ReadonlySet<string> = new Set(["executive", "high-signal", "operations"]);

export function useDashboardViewUrlState() {
  const [searchParams, setSearchParams] = useSearchParams();

  const view = useMemo((): DashboardViewId => {
    const raw = searchParams.get("view");
    if (raw && VALID.has(raw)) {
      return raw as DashboardViewId;
    }
    return "executive";
  }, [searchParams]);

  const setView = useCallback(
    (next: DashboardViewId) => {
      const nextParams = new URLSearchParams(searchParams);
      if (next === "executive") {
        nextParams.delete("view");
      } else {
        nextParams.set("view", next);
      }
      setSearchParams(nextParams, { replace: true });
    },
    [searchParams, setSearchParams]
  );

  return { view, setView };
}
