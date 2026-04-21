import { describe, expect, it } from "vitest";
import { sortRemediationGroups } from "../src/components/overview/RemediationGroupingPanel";

describe("sortRemediationGroups", () => {
  it("sorts by total risk, then finding count", () => {
    const groups = sortRemediationGroups([
      {
        key: "a",
        label: "A",
        why: "x",
        findingCount: 4,
        totalMonthlyRiskUsd: 500
      },
      {
        key: "b",
        label: "B",
        why: "x",
        findingCount: 7,
        totalMonthlyRiskUsd: 500
      },
      {
        key: "c",
        label: "C",
        why: "x",
        findingCount: 10,
        totalMonthlyRiskUsd: 400
      }
    ]);

    expect(groups.map((g) => g.key)).toEqual(["b", "a", "c"]);
  });

  it("uses stable lexical fallback when metrics tie", () => {
    const groups = sortRemediationGroups([
      {
        key: "z",
        label: "Zulu",
        why: "x",
        findingCount: 1,
        totalMonthlyRiskUsd: 100
      },
      {
        key: "a",
        label: "Alpha",
        why: "x",
        findingCount: 1,
        totalMonthlyRiskUsd: 100
      }
    ]);

    expect(groups.map((g) => g.key)).toEqual(["a", "z"]);
  });
});

