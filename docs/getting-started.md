# Getting Started

## Prerequisites

1. Go 1.24+ (see `go.mod`)
2. AWS credentials configured for your management account
3. The `CloudriftAuditRole` deployed in member accounts - see [iam-setup.md](iam-setup.md)

## Build and run

```bash
make build
cloudrift version
```

The binary embeds the dashboard UI. If you just want the API without the frontend:

```bash
make dev
```

## Run a scan

```bash
cloudrift scan
cloudrift report --scan-id latest --format table
```

The default `cloudrift scan` creates a scan directory with metadata and an empty `findings.json`. For a fully populated local dataset without AWS, use the demo generator:

```bash
cloudrift demo generate
cloudrift report --scan-id latest --format table
```

The repo ships `cloudrift-output/demo/` with 18 findings (orphaned edges + external trust) for dashboard visualization. Pick it from the scan list after `cloudrift dashboard`, or open `?scan_id=demo` directly. To regenerate it from the bundled fixture:

```bash
cloudrift demo generate --output-dir ./cloudrift-output --scan-id demo --timestamp 2026-04-18T18:00:00Z
```

## Start the dashboard

```bash
cloudrift dashboard --open
```

`/overview` has three modes - Executive Summary, High-Signal, and Operations - via URL state (`?view=...`). UI theme preference is saved under `localStorage` key `cloudrift-dashboard-theme`.

## AWS profile selection

Use `--profile` on any command to specify which named AWS profile to use:

```bash
cloudrift scan --profile prod
cloudrift dashboard --profile staging --open
```

See the [Choosing an AWS profile](../README.md#choosing-an-aws-profile) section in the README for the full resolution order.

## Neo4j (optional graph)

After configuring `[neo4j]` in `cloudrift.toml`:

```bash
cloudrift scan --neo4j
# or use demo data
cloudrift demo generate --neo4j --dense
```

Use `cloudrift query "..."` for retrieval-only output against the projected graph. See [technical.md](technical.md) for full details on embeddings and query behavior.

## Rebuilding the dashboard UI

If you modify the frontend, rebuild and recompile the binary:

```bash
make build
```

## Command reference

| Command | Purpose |
| --- | --- |
| `scan` | Run a scan |
| `report` | Generate a report from a scan |
| `dashboard` | Serve the dashboard and API |
| `query` | Graph-backed retrieval (Phase 3) |
| `demo generate` | Generate deterministic demo data |
| `version` | Print version |
