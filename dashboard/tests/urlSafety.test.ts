import { describe, expect, it } from "vitest";
import { safeExternalUrl } from "../src/lib/urlSafety";

describe("safeExternalUrl", () => {
  it("allows http and https absolute URLs", () => {
    expect(safeExternalUrl("https://example.com/path?q=1")).toBe("https://example.com/path?q=1");
    expect(safeExternalUrl("http://example.com")).toBe("http://example.com/");
  });

  it("normalizes bare hostnames to https", () => {
    expect(safeExternalUrl("example.com")).toBe("https://example.com/");
    expect(safeExternalUrl("sub.example.com/path")).toBe("https://sub.example.com/path");
  });

  it("rejects scriptable or unsupported schemes", () => {
    expect(safeExternalUrl("javascript:alert(1)")).toBeNull();
    expect(safeExternalUrl("data:text/html,<script>alert(1)</script>")).toBeNull();
    expect(safeExternalUrl("file:///etc/passwd")).toBeNull();
    expect(safeExternalUrl("mailto:security@example.com")).toBeNull();
  });

  it("rejects credentials in URL", () => {
    expect(safeExternalUrl("https://user:pass@example.com")).toBeNull();
    expect(safeExternalUrl("https://token@example.com")).toBeNull();
  });

  it("rejects invalid or hostless values", () => {
    expect(safeExternalUrl("")).toBeNull();
    expect(safeExternalUrl("   ")).toBeNull();
    expect(safeExternalUrl("https://")).toBeNull();
    expect(safeExternalUrl("http:///path-only")).toBeNull();
  });
});
