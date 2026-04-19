# Cloudrift Architecture

## Phase 1–2 (core)

The primary pipeline is **file-backed**:

1. Collect account/resource data.
2. Validate DNS/HTTP state.
3. Score claimability and cost.
4. Persist findings as JSON and render user reports (CLI, Excel, dashboard).

Storage is intentionally flat-file JSON under `cloudrift-output/<scan-id>/`. Scan directory access uses shared rules in `internal/scans` (`ResolveScanDirectoryName`, `IsSafeScanID`, `latest` resolution).

## Phase 3 (optional graph)

**Neo4j** is an optional projection: `cloudrift scan --neo4j` writes graph rows from existing scan JSON; **`findings.json` / `scan-metadata.json` remain the source of truth**. Embeddings and hybrid retrieval live in `internal/graph`; operator-facing CLI entry is `cloudrift query`.

For API routes, dashboard behavior (including light/dark theme), Mermaid diagrams, debugging, and security notes, see [TECHNICAL.md](TECHNICAL.md).
