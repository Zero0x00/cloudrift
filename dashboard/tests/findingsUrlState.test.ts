import { describe, expect, it } from "vitest";
import { buildFindingsSearchParams, parseFindingsUrlState } from "../src/hooks/useFindingsUrlState";

describe("findings URL state", () => {
  it("parses finding_id as exact-target state", () => {
    const params = new URLSearchParams("finding_id=abc123&page=2");
    const state = parseFindingsUrlState(params);
    expect(state.findingId).toBe("abc123");
    expect(state.page).toBe(2);
  });

  it("builds finding_id into query params", () => {
    const params = buildFindingsSearchParams({
      page: 1,
      pageSize: 50,
      severity: "",
      module: "",
      accountId: "",
      claimability: "",
      search: "",
      trustStale: false,
      adminLike: false,
      trustClassification: "",
      principalType: "",
      externalPrincipal: "",
      externalAccountId: "",
      findingId: "demo-trust-2",
      groupByAccount: false
    });
    expect(params.get("finding_id")).toBe("demo-trust-2");
    expect(params.get("page")).toBeNull();
  });
});
