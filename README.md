# Cloudrift

Cloudrift is a CLI and embedded dashboard for discovering orphaned AWS edge assets (DNS, S3 websites, CloudFront) and risky cross-account IAM trust relationships. It scans your AWS organization, writes findings as local JSON (no database required), and provides evidence-backed severity scoring.

**Key points:**
- Calls AWS APIs to inventory edge and IAM-trust risk across your org
- Writes findings to local JSON per scan — read via dashboard, CLI reports, or your own tooling
- Dashboard provides interactive exploration and a scan control center
- `cloudrift demo generate` populates the UI with sample data (no AWS needed)
- Neo4j is optional for advanced graph features (blast-radius, vector search, `cloudrift query`)

---

## Documentation index

| Document | Purpose |
| --- | --- |
| [starter-doc.html](starter-doc.html) | **Start here** — 20-minute walkthrough with diagrams and status |
| [docs/cli-commands.md](docs/cli-commands.md) | What each command does and flags available |
| [docs/getting-started.md](docs/getting-started.md) | Quick-start guide (demo or live AWS scan) |
| [docs/architecture.md](docs/architecture.md) | System design and where code lives |
| [docs/technical.md](docs/technical.md) | API reference, debugging, embeddings, security |
| [docs/iam-setup.md](docs/iam-setup.md) | AWS org setup and audit role deployment |
| [docs/security-coverage.md](docs/security-coverage.md) | What is and isn't detected; severity caveats |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Contribution guidelines |

---

## Current implementation status

| Feature | Status | Notes |
| --- | --- | --- |
| `cloudrift scan` | Partial | Writes metadata + empty findings (discovery layer not yet wired) |
| `cloudrift demo generate` | Complete | Deterministic populated scan for learning the UI |
| Dashboard + REST API | Complete | Works from JSON on disk; Neo4j optional |
| `cloudrift report` | Complete | Export findings as table, JSON, CSV, or markdown |
| `cloudrift query` | Complete | Vector search over Neo4j (retrieval-only, no LLM synthesis) |
| Neo4j integration | Optional | Adds blast-radius explorer, graph relationships, vector index |
| Local embeddings | Stub | `provider=local` will error until implemented |

**Try immediately (no AWS):** `cloudrift demo generate && cloudrift dashboard --open`

---

## Installation

### Option 1 - Pre-built binary (recommended)

Download the latest release for your platform:

```bash
# Linux / macOS (example for linux_amd64)
curl -sSL https://github.com/Zero0x00/cloudrift/releases/latest/download/cloudrift_Linux_x86_64.tar.gz | tar -xz
sudo mv cloudrift /usr/local/bin/
cloudrift version
```

Verify the checksum:

```bash
sha256sum -c checksums.txt --ignore-missing
```

### Option 2 - `go install`

Requires Go 1.24+. The dashboard UI is embedded at release time; `go install` builds API-only (no UI).

```bash
go install github.com/Zero0x00/cloudrift/cmd/cloudrift@latest
cloudrift version
```

### Option 3 - Build from source

Requires Go 1.24+ and Node.js 20+:

```bash
git clone https://github.com/Zero0x00/cloudrift.git
cd cloudrift
make build
sudo mv cloudrift /usr/local/bin/
cloudrift version
```

---

## What it solves

- **Orphaned edge:** DNS hostnames that resolve but point to deleted/misconfigured S3 buckets, CloudFront origins, or ELBs — with a verdict (reclaimable, dangling, etc.)
- **External trust:** IAM roles trusting external AWS accounts or principals, scored by last-used date, admin privileges, and risk profile
- **Visibility:** Self-hosted dashboard and CLI reports over the same HTTP server — no cloud dependency

---

## Requirements

### AWS

- **AWS credentials** for the management account (or delegated audit role) that can assume org member roles
- See [docs/iam-setup.md](docs/iam-setup.md) for org setup and StackSet deployment

### Optional

- **Neo4j 5+** — for blast-radius exploration, relationship graphs, and vector search (`cloudrift query`). The dashboard and core workflows degrade cleanly without it (JSON-only mode).

### Build-time only

- Go 1.24+ and Node.js 20+ (to build from source; not needed for pre-built binaries)

---

## AWS credential selection

**Dashboard:** Exposes a profile picker on the Scan Control page.

**CLI:** Does not have a `--profile` flag. Credentials come from (in order):

1. `[aws].management_profile` in `cloudrift.toml` (default: `"default"`)
2. `AWS_PROFILE` environment variable (if `management_profile` is empty)
3. AWS default profile chain (env vars, SSO, instance role, etc.)

---

## Build commands

```bash
make build    # Full build with dashboard (requires npm + go)
make dev      # Binary only, no npm step (API still works, no UI)
make test     # Run all tests
```

The version string comes from the latest git tag. Tagged releases (e.g. `v0.2.0`) are injected; untagged builds show `dev`.

---

## Configuration

Optional TOML file (defaults work without it). Search order: `CLOUDRIFT_CONFIG` env var, then `./cloudrift.toml`.

Key sections:

- `[aws]` — org role name, management profile, regions to scan
- `[scan]` — HTTP concurrency, role-assumption concurrency, timeouts
- `[cost]` — `use_cur` flag for Cost Explorer enrichment
- `[trust]` — approved external accounts, thresholds for stale/ghost roles
- `[output]` — scan output directory (default: `./cloudrift-output`)
- `[neo4j]` — `uri`, `username`, `password_env` (optional)
- `[embeddings]` — embedding provider (default: `openai`, local stub)

Environment:

- `CLOUDRIFT_APP_BASE_URL` — base URL for alert links in Slack (e.g. `https://your-host:8080`); defaults to `http://127.0.0.1:8080`

---

## Commands

Global option: `--config <path>` to specify a TOML config file.

| Command | Purpose | Key flags |
| --- | --- | --- |
| `scan` | Scan AWS org for edge + IAM findings | `--output-dir`, `--neo4j` |
| `demo generate` | Create deterministic sample scan | `--output-dir`, `--neo4j`, `--dense` |
| `report` | Export findings to table/JSON/CSV/markdown | `--scan-id`, `--format`, `--output` |
| `query` | Search findings via vector retrieval (Neo4j required) | `--scan-id`, `--query`, `--format` |
| `dashboard` | Start the web UI and REST API | `--port`, `--open`, `--output-dir` |
| `version` | Print version string | — |

---

## Dashboard UI

Start the dashboard:

```bash
cloudrift dashboard --port 8080 --open
```

Open `http://127.0.0.1:8080` in your browser (or use `?scan_id=<id>` to load a specific scan).

| Page | Purpose |
| --- | --- |
| Overview | Summary, high-signal findings, operations view |
| Scan Control | Start scans, validate AWS profile, runtime status |
| Findings | Paginated findings table with filters and sorting |
| Triage | Findings in triage/review mode |
| Accounts | Per-account risk breakdown |
| Diff | Compare two scans side-by-side |
| Trust Report | IAM trust findings, external principals |
| External Entities | Entity-centric view with blast actions |
| Blast Explorer | 3D graph visualization of risk paths (Neo4j required) |
| Alerting | Slack webhooks, alert rules, event history |

Light and dark themes available; preference stored in browser localStorage.

---

## Neo4j (optional graph tier)

### Setup

1. Run Neo4j 5+ with Bolt accessible from your machine
2. Create a database user and add to `cloudrift.toml`:

```toml
[neo4j]
uri = "bolt://127.0.0.1:7687"
username = "neo4j"
password_env = "CLOUDRIFT_NEO4J_PASSWORD"
```

3. Export a scan: `cloudrift scan --neo4j` or `cloudrift demo generate --neo4j --dense`

### Local Docker setup

```bash
export CLOUDRIFT_NEO4J_PASSWORD='dev-password-only'
docker run --name cloudrift-neo4j -p 7474:7474 -p 7687:7687 \
  -e NEO4J_AUTH=neo4j/${CLOUDRIFT_NEO4J_PASSWORD} \
  -d neo4j:5
```

### Example queries

```cypher
MATCH (s:ScanSnapshot) RETURN s.scan_id, s.finding_count LIMIT 25;
MATCH (f:Finding) WHERE f.scan_id = $scan RETURN f.id, f.title, f.severity LIMIT 50;
```

### Graceful degradation

Neo4j is optional. When unconfigured or unreachable, the dashboard and API still work with JSON-only findings (no blast-radius explorer or vector search).

---

## API overview

All endpoints under `/api`, request/response as JSON:

```
GET  /api/scans
GET  /api/scans/{id}/summary
GET  /api/scans/{id}/findings
GET  /api/scans/{id}/external-entities
GET  /api/scans/{id}/accounts
GET  /api/diff?old=<scan>&new=<scan>

POST /api/runtime/validate-profile
POST /api/scan/start
GET  /api/scan/status
GET  /api/scan/history
GET  /api/scan/progress (WebSocket)

GET  /api/alerts/catalog
GET  /api/alerts/rules
POST /api/alerts/rules/{ruleID}/test
```

Full reference: [docs/technical.md](docs/technical.md#api-documentation)

---

## Scan output structure

```
cloudrift-output/
└── <scan-id>/
    ├── scan-metadata.json
    ├── findings.json
    ├── relationships.json    (optional)
    └── assets/               (optional)
        └── *.json
```

Latest scan resolves by timestamp in `scan-metadata.json` (newest first); directory name used as tiebreak.

---

## Org setup

For multi-account scanning, set up an audit role via CloudFormation StackSet:

See [docs/iam-setup.md](docs/iam-setup.md) and `iam/stackset-template.yaml`

---

## Contributing

Code contributions welcome. Guidelines:

- Keep AWS I/O in `collectors/`, business logic in `scorers/`/`validators/`, HTTP handlers in `internal/api/`
- Tests: `go test ./...` covers handlers, scorers, and fakes
- Frontend: React 18, Vite 5, Tailwind CSS, TanStack Query. `npm run dev` in `dashboard/` proxies `/api` to `http://127.0.0.1:8080` by default

See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

---

## Version and license

Current version: injected from git tag (`git describe`). See [tech-spec-v2.md](tech-spec-v2.md) for historical context and design notes.
