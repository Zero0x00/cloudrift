# Getting Started

1. Install Go 1.24+ (see `go.mod`).
2. Configure AWS credentials/profile for your management account.
3. Deploy the audit role StackSet (`docs/iam-setup.md`).

**Phase 3 graph embeddings:** Defaults in code use **`embeddings.provider = "openai"`** (the only
operational provider today). Set **`OPENAI_API_KEY`** when you enable embedding features.
`local` is planned only, not supported yet — see `internal/config/config.go` and `docs/TECHNICAL.md`.
4. Run:

```bash
go mod tidy
go build -o cloudrift ./cmd/cloudrift
./cloudrift scan
./cloudrift report --scan-id latest --format table
```

**Optional Phase 3:** After Neo4j is configured, `./cloudrift scan --neo4j` exports the new scan to the graph (JSON remains canonical). Use `./cloudrift query "…"` for retrieval-only CLI output (see `docs/TECHNICAL.md`).

**Dashboard:** `./cloudrift dashboard` serves the SPA; use the header theme toggle (preference: `localStorage` key `cloudrift-dashboard-theme`). Rebuild embedded assets with `cd dashboard && npm run build` after UI changes.
