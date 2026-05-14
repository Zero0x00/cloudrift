# Cloudrift

Cloudrift is a Go CLI plus an embedded React dashboard for discovering and reporting on orphaned AWS edge assets (DNS, S3 websites, CloudFront, etc.) and cross-account IAM trust relationships. It produces evidence-backed findings with severity, claimability, and estimated monthly cost/risk signals, stored as JSON under a scan directory (no database required).

**Full system documentation:** [docs/technical.md](docs/technical.md) (API reference, diagrams, debugging, security).

---

## Installation

### Option 1 - Download a pre-built binary (recommended)

Grab the latest release for your platform from the [Releases page](https://github.com/Zero0x00/cloudrift/releases):

```bash
# Linux / macOS (example for linux_amd64)
curl -sSL https://github.com/Zero0x00/cloudrift/releases/latest/download/cloudrift_Linux_x86_64.tar.gz | tar -xz
sudo mv cloudrift /usr/local/bin/
cloudrift version
```

Verify the checksum before use:

```bash
sha256sum -c checksums.txt --ignore-missing
```

### Option 2 - `go install`

Requires Go 1.24+. The dashboard UI is embedded in the binary at release time; `go install` builds without it (API-only mode).

```bash
go install github.com/Zero0x00/cloudrift/cmd/cloudrift@latest
cloudrift version
```

### Option 3 - Build from source

Requires Go 1.24+ and Node.js 20+.

```bash
git clone https://github.com/Zero0x00/cloudrift.git
cd cloudrift
make build
sudo mv cloudrift /usr/local/bin/
cloudrift version
```

---

## What it solves

- **Orphaned edge:** Hostnames that still resolve but point at deleted buckets, broken origins, or ambiguous CloudFront mappings - with a structured verdict (e.g. reclaimable vs dangling).
- **External trust:** IAM roles that trust external principals, scored using role last-used, admin posture, and approved vendor accounts.
- **Visibility:** CLI reports and a local dashboard served over the same HTTP server.

---

## Requirements

- **AWS credentials** with read permissions for the accounts you want to scan (see [docs/iam-setup.md](docs/iam-setup.md))
- **Neo4j 5+** - required for graph features (blast radius explorer, query). See [Neo4j setup](#neo4j-graph) below.

Build-time only (not needed to run a downloaded binary):
- Go 1.24+
- Node.js 20+ (for the embedded dashboard UI)

---

## Build from source

```bash
make build    # builds dashboard + binary (requires npm + go)
make dev      # binary only, no npm step (no UI, API still works)
make test     # go test ./...
```

The version string is injected from the latest git tag (`git describe`). Tagged releases use the semver tag (e.g. `v0.2.0`); untagged dev builds show `dev`.

---

## Configuration

Optional TOML (defaults apply if missing). Search order: `CLOUDRIFT_CONFIG` env var, then `./cloudrift.toml`.

Notable sections:

- `[aws]` - org role name, management profile, regions
- `[scan]` - HTTP concurrency, role assumption concurrency, timeouts
- `[cost]` - `use_cur` for optional Cost Explorer enrichment
- `[trust]` - `approved_external_accounts`, stale/ghost day thresholds
- `[output]` - `output_dir` (default `./cloudrift-output`)
- `[neo4j]` - `uri`, `username`, `password_env`
- `[embeddings]` - Phase 3 only; default provider is `openai` (`text-embedding-3-small`, 384 dimensions). Set `OPENAI_API_KEY` (or the env name in `openai_api_key_env`) when using graph embedding features.

Environment: `CLOUDRIFT_APP_BASE_URL` - optional base URL for alert action links in Slack (e.g. `https://your-host:8080`). Defaults to `http://127.0.0.1:8080`.

---

## Commands

| Command | Description |
| --- | --- |
| `cloudrift scan` | Runs a scan and writes output under `--output-dir`. Flags: `--neo4j` (export to Neo4j), `--output-dir`. |
| `cloudrift demo generate` | Generates a deterministic demo scan with mixed severities, relationships, and assets. Flags: `--neo4j`, `--dense` (multi-hop trust chains), `--output-dir`. |
| `cloudrift report` | Reads `findings.json` for `--scan-id` (default `latest`) and emits `table`, `json`, `csv`, or `markdown`. |
| `cloudrift query` | Hybrid vector retrieval against Neo4j for a scan (no LLM answer synthesis). Flags: `--scan-id`, `--query`, `--format table\|json`, `--output-dir`. |
| `cloudrift dashboard` | Serves REST API + embedded SPA on `--port` (default `8080`). Flags: `--open`, `--scan-id`, `--output-dir`. |
| `cloudrift version` | Prints the version string. |

---

## Dashboard

```bash
cloudrift dashboard --output-dir ./cloudrift-output --port 8080 --open
```

Open `http://127.0.0.1:8080` (optional `?scan_id=<id>`).

| Path | Purpose |
| --- | --- |
| `/overview` | Executive Summary, High-Signal, and Operations views with finding/trust/entity drilldowns |
| `/scan-control` | Start scans from the UI, validate AWS profile, runtime capability badges, live progress |
| `/findings` | Paginated findings table with filters |
| `/triage` | Findings in triage mode |
| `/accounts` | Per-account rollups |
| `/diff` | Compare two scans |
| `/trust-report` | Trust-focused view for `external_access` findings |
| `/external-entities` | Entity-centric table with entity and principal blast actions |
| `/blast-explorer` | 3D blast radius explorer (finding, entity, and principal root views) |
| `/alerting` | Alert rules (Slack webhooks), evaluation preview, test send, and event history |

The SPA supports light and dark themes. Toggle with the header control; preference is stored in `localStorage` under `cloudrift-dashboard-theme` (default dark).

---

## Neo4j graph

### Setup

1. Run Neo4j 5+ with Bolt reachable from your machine and create a database user.
2. Add `[neo4j]` to `cloudrift.toml`:

```toml
[neo4j]
uri = "bolt://127.0.0.1:7687"
username = "neo4j"
password_env = "CLOUDRIFT_NEO4J_PASSWORD"
```

3. Export: `cloudrift scan --neo4j` or `cloudrift demo generate --neo4j --dense` for a sample graph.

Local dev with Docker:

```bash
export CLOUDRIFT_NEO4J_PASSWORD='change-me-dev-only'
docker run --name cloudrift-neo4j -p 7474:7474 -p 7687:7687 \
  -e NEO4J_AUTH=neo4j/${CLOUDRIFT_NEO4J_PASSWORD} \
  -d neo4j:5
```

Example Cypher:

```cypher
MATCH (s:ScanSnapshot) RETURN s.scan_id, s.finding_count LIMIT 25;
MATCH (f:Finding) WHERE f.scan_id = $scan RETURN f.id, f.title, f.severity LIMIT 50;
```

### Graph-optional behavior

Neo4j is a power feature - the dashboard and API core surfaces work without it. When Neo4j is not configured or unreachable, blast-radius endpoints return `graph_available: false` with a `graph_unavailable_reason` field; all other pages continue normally.

---

## API overview

All endpoints under `/api`, JSON in/out:

```
GET  /api/scans
GET  /api/scans/{id}/summary
GET  /api/scans/{id}/findings
GET  /api/scans/{id}/findings/{fid}
GET  /api/scans/{id}/external-entities
GET  /api/scans/{id}/accounts
GET  /api/scans/{id}/top-fixes
GET  /api/scans/{id}/remediation-groups
GET  /api/scans/{id}/blast-radius/summary
GET  /api/scans/{id}/blast-radius/explorer
GET  /api/diff?old=<scan>&new=<scan>
GET  /api/runtime/status
POST /api/runtime/validate-profile
POST /api/scan/start
GET  /api/scan/status
GET  /api/scan/history
```

WebSocket: `GET /api/scan/progress` - live scan progress events; loopback origins only.

Details, examples, and error format: [docs/technical.md](docs/technical.md#3-api-documentation).

---

## Scan layout

```
cloudrift-output/
└── <scan-id>/
    ├── scan-metadata.json
    ├── findings.json
    ├── relationships.json    # optional; used by Neo4j export when present
    └── assets/               # optional; *.json arrays of AssetNode
```

`latest` resolves to the scan with the newest `scan-metadata.json` timestamp; directory name is used as a tie-break.

---

## IAM / organization

See [docs/iam-setup.md](docs/iam-setup.md) and `iam/stackset-template.yaml` for org-wide audit role deployment patterns.

---

## Contributing

- **Packages:** AWS I/O in `collectors`, pure logic in `scorers`/`validators`, HTTP in `internal/api`.
- **Tests:** `go test ./...` covers API handlers, scorer golden behavior, and collector fakes.
- **Frontend:** React 18, Vite 5, Tailwind (`darkMode: class`), TanStack Query. `npm run dev` proxies `/api` to `http://127.0.0.1:8080` by default. Override with `VITE_API_PROXY_TARGET` in `dashboard/.env.local`.
