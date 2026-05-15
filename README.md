# Cloudrift

**In one sentence:** Cloudrift is built to **call AWS APIs** and inventory org-wide edge + IAM-trust risk, writing evidence to **local JSON** per scan. The **dashboard** and **`cloudrift report`** read those files. **`demo generate`** exists only to **populate the UI** with deterministic sample data when you are not hitting AWS. **Neo4j is a coupled graph tier** for relationships, blast-radius, embeddings, **`cloudrift query`**, and future RAG-style investigation — the **main** tables/diff/trust flows still run on JSON alone (vector retrieval remains **retrieval-only**, no LLM answer synthesis).

**Who it is for:** security engineers, cloud engineers, and leaders who want evidence-backed answers without standing up a database first.

| Path | Role |
| --- | --- |
| [starter-doc.html](starter-doc.html) | **Main beginner guide** — 20 topics, diagrams, copy buttons, honest status |
| [docs/cli-commands.md](docs/cli-commands.md) | What each `cloudrift` subcommand does |
| [docs/architecture.md](docs/architecture.md) | System shape and where code lives |
| [docs/technical.md](docs/technical.md) | API contracts, embeddings, debugging |
| [docs/iam-setup.md](docs/iam-setup.md) | Org role and StackSet |
| [docs/security-coverage.md](docs/security-coverage.md) | What is detected, what is not, severity caveats |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute |
| [tech-spec-v2.md](tech-spec-v2.md) | Historical spec anchor; live code may differ on purpose |

---

## Current status (read this first)

| Area | Status |
| --- | --- |
| Default `cloudrift scan` | Writes **metadata + empty `findings.json`** (orchestration gap). |
| `cloudrift demo generate` | **Fully populated** deterministic scan for UI/report learning. |
| Dashboard + `/api/*` | Works from **`--output-dir`** JSON; **Neo4j not required**. |
| Neo4j + `query` | **Graph tier** (coupled): needs Bolt + config + embeddings for advanced graph/vector flows; **not** required for core JSON workflows. |
| Local embeddings | **Stubbed** (`provider=local` errors until implemented). |
| CLI `--profile` | **Does not exist** — use TOML `management_profile` or default chain. |

**Fastest try (UI + reports, no AWS inventory):** `cloudrift demo generate && cloudrift dashboard --open` — see [getting-started](docs/getting-started.md). **Real org value** still assumes **AWS APIs** + correct IAM role rollout once `scan` writes populated findings.

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

- **AWS credentials** for the management account path you use to reach member accounts (see [docs/iam-setup.md](docs/iam-setup.md)).
- **Neo4j 5+** — **Graph tier (coupled).** Brings relationship graph, blast-radius APIs, vector retrieval, and `cloudrift query` (and room for future RAG-style investigation). **Main workflows** (listings, findings, diff, trust views) use JSON on disk only — the app degrades cleanly when Neo4j is off. See [Neo4j setup](#neo4j-graph) below.

### AWS profile (CLI vs dashboard)

The **CLI does not implement `--profile`**. Credentials for `cloudrift scan` (and other commands) come from:

1. `[aws].management_profile` in `cloudrift.toml` when set (default in code is `"default"`).
2. If `management_profile` is **empty**, the AWS SDK default chain applies (this includes honoring **`AWS_PROFILE`** the same way other AWS tools do).

The **dashboard** exposes a profile picker on **Scan Control**; selected profiles are sent in API JSON bodies (`POST /api/scan/start`, `POST /api/runtime/validate-profile`), not via a global CLI flag.

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

Global flag: `--config` path to TOML (optional; see [Configuration](#configuration)).

| Command | Description |
| --- | --- |
| `cloudrift scan` | **Implemented:** writes `output_dir/<scan-id>/` with `scan-metadata.json` + **empty** `findings.json`, then optionally exports to Neo4j (`--neo4j`). **Flags:** `--output-dir`, `--neo4j`. **`--no-http` / `--concurrency`:** registered but **not wired** to `scanrun.Run` (stub). |
| `cloudrift demo generate` | **Implemented:** deterministic populated scan (`findings.json`, `relationships.json`, `assets/*.json`). Flags: `--output-dir`, `--neo4j`, `--dense`, `--scan-id`, hidden `--timestamp` (tests). |
| `cloudrift report` | Reads `findings.json` for `--scan-id` (default `latest`). `--format` `table\|json\|csv\|markdown`; optional `--output`. |
| `cloudrift query` | **Graph tier:** hybrid vector retrieval over Neo4j — **retrieval only**, no LLM answer synthesis. Flags include `--scan-id`, `--query` or positional text, `--format`, `--output-dir`, `--top-k`, `--require-stored-embedding-identity`, `--legacy-retrieval`. |
| `cloudrift dashboard` | Serves REST + embedded SPA. Flags: `--port`, `--open`, `--scan-id` (default URL hint), `--output-dir`. |
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
| `/query` | Phase 3 graph query UI (when Neo4j + embeddings are configured) |

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

### Graph-tier behavior (Neo4j off)

The dashboard and API **core** surfaces work from JSON alone. When Neo4j is not configured or unreachable, blast-radius and graph-native endpoints return `graph_available: false` with `graph_unavailable_reason`; JSON-backed pages keep working.

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
GET  /api/scans/{id}/external-entities/blast-radius/summary
GET  /api/scans/{id}/external-entities/blast-radius/explorer
GET  /api/scans/{id}/principals/blast-radius/summary
GET  /api/scans/{id}/principals/blast-radius/explorer
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

See **[CONTRIBUTING.md](CONTRIBUTING.md)** for dev setup, tests, where to add collectors/scorers/dashboard pages, and `graphify update`.

- **Reviewer hub:** [starter-doc.html](starter-doc.html) (hash-routed single HTML file at repo root).
- **Spec anchor:** [tech-spec-v2.md](tech-spec-v2.md) (historical pointer + where deviations are documented).
- **Packages:** AWS I/O in `collectors`, pure logic in `scorers`/`validators`, HTTP in `internal/api`.
- **Tests:** `go test ./...` covers API handlers, scorer golden behavior, and collector fakes.
- **Frontend:** React 18, Vite 5, Tailwind (`darkMode: class`), TanStack Query. `npm run dev` proxies `/api` to `http://127.0.0.1:8080` by default. Override with `VITE_API_PROXY_TARGET` in `dashboard/.env.local`.
