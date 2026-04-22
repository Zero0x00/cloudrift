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

**Important current behavior:** default `cloudrift scan` writes scan metadata and an empty `findings.json` in the scan directory. For a populated local dataset suitable for dashboard/API/demo workflows, use:

```bash
./cloudrift demo generate
./cloudrift report --scan-id latest --format table
```

The repository includes **`cloudrift-output/demo/`** (18 findings: orphaned edges + external trust) tuned for dashboard visualization. Open the dashboard with `?scan_id=demo` or pick **demo** from the scan list after `./cloudrift dashboard`. To refresh that directory from the embedded bundle after editing `cmd/cloudrift/testdata/bundled_demo_findings.json`:

```bash
./cloudrift demo generate --output-dir ./cloudrift-output --scan-id demo --timestamp 2026-04-18T18:00:00Z
```

**Optional Phase 3:** After Neo4j is configured, `./cloudrift scan --neo4j` exports the new scan to the graph (JSON remains canonical). Use `./cloudrift query "…"` for retrieval-only CLI output (see `docs/TECHNICAL.md`).

**Dashboard:**

- `./cloudrift dashboard` serves the SPA and API.
- `/overview` includes 3 modes (`Executive Summary`, `High-Signal`, `Operations`) via URL state (`?view=...`).
- High-Signal includes prioritized risk queue + remediation grouping; Operations emphasizes ownership risk and next actions.
- `/scan-control` includes explicit runtime states (loading, error, empty/unconfigured, ready) and safe runtime checks.
- UI theme preference is saved under `localStorage` key `cloudrift-dashboard-theme`.
- Both themes are maintained as first-class surfaces (dark-mode contrast and focus states are tuned across cards/tables/charts).

Rebuild embedded assets after UI changes:

```bash
cd dashboard
npm ci
npm run build
cd ..
go build -o cloudrift ./cmd/cloudrift
```

## Command reality check

Current CLI command set:

- `scan`
- `report`
- `dashboard`
- `query` (Phase 3 retrieval-only)
- `demo generate`
- `version`

`diff` and `remediate` are not active CLI commands in the current command tree.
