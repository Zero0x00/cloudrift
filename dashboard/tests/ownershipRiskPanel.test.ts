import { describe, expect, it } from "vitest";
import { sortOwnershipRiskItems } from "../src/components/overview/OwnershipRiskPanel";

describe("sortOwnershipRiskItems", () => {
  it("orders by risk, then critical, then high", () => {
    const sorted = sortOwnershipRiskItems([
      {
        accountId: "c",
        accountName: "charlie",
        riskUsd: 300,
        criticalCount: 0,
        highCount: 10
      },
      {
        accountId: "a",
        accountName: "alpha",
        riskUsd: 500,
        criticalCount: 1,
        highCount: 1
      },
      {
        accountId: "b",
        accountName: "beta",
        riskUsd: 500,
        criticalCount: 2,
        highCount: 0
      }
    ]);

    expect(sorted.map((item) => item.accountId)).toEqual(["b", "a", "c"]);
  });

  it("uses stable lexical fallback when numeric metrics tie", () => {
    const sorted = sortOwnershipRiskItems([
      {
        accountId: "z-2",
        accountName: "zulu",
        riskUsd: 100,
        criticalCount: 1,
        highCount: 1
      },
      {
        accountId: "a-1",
        accountName: "alpha",
        riskUsd: 100,
        criticalCount: 1,
        highCount: 1
      }
    ]);

    expect(sorted.map((item) => item.accountId)).toEqual(["a-1", "z-2"]);
  });
});

