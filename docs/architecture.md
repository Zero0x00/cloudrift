# Cloudrift Architecture

## Phase 1–2 (core)

The primary pipeline is **file-backed**:

1. Collect account/resource data.
2. Validate DNS/HTTP state.
3. Score claimability and cost.
4. Persist findings as JSON and render user reports (CLI, Excel, dashboard).

Storage is intentionally flat-file JSON under `cloudrift-output/<scan-id>/`. Scan directory access uses shared rules in `internal/scans` (`ResolveScanDirectoryName`, `IsSafeScanID`, `latest` resolution).

**Current orchestration note:** the default CLI scan path (`cloudrift scan`) and dashboard Scan Control start path currently create scan metadata plus an empty `findings.json`. The collectors/scorers pipeline exists in `internal/` and is covered by tests, but full end-to-end wiring into the default scan command remains an explicit gap.

## Phase 3 (optional graph)

**Neo4j** is an optional projection: `cloudrift scan --neo4j` (or `cloudrift demo generate --neo4j`) writes graph rows from existing scan JSON; **`findings.json` / `scan-metadata.json` remain the source of truth**. Embeddings and hybrid retrieval live in `internal/graph`; operator-facing CLI entry is `cloudrift query` (retrieval-only).

## Dashboard and API behavior

- Dashboard is served from the Go binary and uses left-rail primary navigation.
- `/overview` supports in-page product modes: `Executive Summary`, `High-Signal`, and `Operations` (`?view=...`).
- High-Signal is optimized for prioritized triage (top fixes + remediation groups); Operations is optimized for action flow (status, ownership risk, next actions).
- Dashboard mode is preserved while navigating within dashboard context; entering dashboard from other routes defaults to executive mode.
- `scan_id` remains URL-driven and is preserved through app navigation.
- Theme is token-driven (`darkMode: class`) with contrast-tuned helper text, table headers, borders, and focus-visible treatment shared across pages.

## Response-shape consistency

List-like API fields are intentionally normalized to stable arrays (`[]`) where practical rather than `null` (for example: scan/list `items`, diff lists, runtime profile lists, scan history items, and summary external-entity arrays). This reduces frontend null-ambiguity and runtime branching complexity.

For API routes, dashboard behavior (including light/dark theme), Mermaid diagrams, debugging, and security notes, see [TECHNICAL.md](TECHNICAL.md).
