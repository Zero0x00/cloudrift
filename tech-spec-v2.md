# Technical specification (v2) — reference

This file exists as a **historical / umbrella spec anchor** for Cloudrift. **The running code is the source of truth** for behavior; this document is not a frozen contract.

> Status update: the detection engine is now wired — a scan runs the full collect → validate → score → persist pipeline (`internal/pipeline`), the orchestration seam both CLI `scan` and the dashboard use. `cloudrift query` also supports optional LLM answer synthesis (`internal/synth`), which runs when a provider API key is configured and otherwise degrades to retrieval-only.

## Current code vs original intent

Implementation details and **known deviations** from early product sketches are maintained in:

- [`docs/cloudrift-docs.md`](docs/cloudrift-docs.md) — the consolidated documentation: API & Technical Reference (contracts, security notes, embeddings, query CLI, debugging), Architecture (phase model, diagrams), Security Coverage, and more.
- [`docs.html`](docs.html) — the same content as an interactive site (audience views, sidebar, search).

When product intent changes, update the consolidated docs first; keep this file as a short pointer unless you are reviving a formal versioned spec.

If a future formal spec is authored, link it here and move deviation bullets into that document’s changelog section.
