# Cloudrift

Cloudrift is a **Go CLI** plus an embedded **React dashboard** for discovering and reporting on **orphaned AWS edge assets** (DNS, S3 websites, CloudFront, etc.) and **cross-account IAM trust** relationships. It produces evidence-backed **findings**, **severity**, **claimability**, and **estimated monthly cost / risk** signals, stored as **JSON under a scan directory** (no database).

**Full system documentation:** [docs/TECHNICAL.md](docs/TECHNICAL.md) (API reference, diagrams, debugging, security).

---

## What it solves

- **Orphaned edge:** Hostnames that still resolve but point at deleted buckets, broken origins, or ambiguous CloudFront mappings—with a structured verdict (e.g. reclaimable vs dangling).
- **External trust:** IAM roles that trust external principals, scored using **role last used**, **admin posture**, and **approved vendor accounts**.
- **Visibility:** CLI reports, Excel export, and a local **dashboard** over the same HTTP server.

---

## Requirements

- **Go** 1.24+ (see `go.mod`)
- **Node.js 18+** (only if you build the dashboard assets yourself)
- **AWS credentials** with permissions appropriate for collectors (when the full scan pipeline is wired—see note below)

---

## Build

```bash
go mod tidy
go test ./...
go build -o cloudrift ./cmd/cloudrift
```

### Dashboard assets (embedded in binary)

The `dashboard` command serves a production build from `embed.FS`. To rebuild UI:

```bash
cd dashboard
npm ci
npm run build
cd ..
go build -o cloudrift ./cmd/cloudrift
```

---

## Configuration

Optional TOML (defaults apply if missing). Search order includes `CLOUDRIFT_CONFIG` and `./cloudrift.toml` (see `internal/config/config.go`).

Notable sections:

- **`[aws]`** — Org role name, management profile, regions  
- **`[scan]`** — HTTP concurrency, role assumption concurrency, timeouts  
- **`[cost]`** — `use_cur` for optional Cost Explorer enrichment  
- **`[trust]`** — `approved_external_accounts`, stale/ghost day thresholds  
- **`[output]`** — `output_dir` (default `./cloudrift-output`)  
- **`[embeddings]`** (Phase 3) — **Default `provider` is `openai`** (set in `internal/config/config.go` `Default()`). That is the **only operational** embedding path today (OpenAI `text-embedding-3-small` with `dimensions=384` for Neo4j). **`provider = "local"` is planned only** (future on-box MiniLM); it is **not supported** yet and will error if embeddings are invoked. Set `OPENAI_API_KEY` (or the env name in `openai_api_key_env`) when using graph embedding features.

---

## Commands

| Command | Description |
|---------|-------------|
| `cloudrift scan` | Creates a new scan directory under `--output-dir` / config with `scan-metadata.json` and `findings.json`. **Current implementation writes an empty findings list**; the scoring/collection stack lives in `internal/` and is used heavily in tests—see [docs/TECHNICAL.md §2](docs/TECHNICAL.md#2-codebase-structure). Optional **`--neo4j`** exports that scan’s artifacts to Neo4j (Phase 3 projection; JSON remains source of truth). |
| `cloudrift demo generate` | Writes a **deterministic** demo scan under `./cloudrift-output/demo-<UTC-timestamp>/`: `findings.json` (mixed severities, orphaned-edge + `external_access`), `scan-metadata.json` (counts aligned with findings), `relationships.json` (including `TRUSTS`), and `assets/*.json`. Use **`--neo4j`** to run the same Neo4j export as `scan --neo4j` after generation. Output dir: **`--output-dir`** (defaults to config / `./cloudrift-output`). |
| `cloudrift report` | Reads `findings.json` for `--scan-id` (default `latest`) and emits `table` (stdout), `json`, `csv`, or `markdown`. Explicit scan IDs must be **path-safe** (same rules as the API); see [docs/TECHNICAL.md — Scan IDs](docs/TECHNICAL.md#scan-id-resolution-and-path-safety). |
| `cloudrift query` | Phase 3 **retrieval-only**: embeds query text, runs hybrid vector retrieval against Neo4j for `--scan-id`, prints grounded hits plus operator notes / empty hints (**no LLM answer synthesis**). Requires Neo4j URI + credentials in config, **OpenAI** embedding key by default, vector index applied, and findings embedded in the graph. Flags: `--output-dir`, `--scan-id` (default `latest`), `--query` or positional text, `--format table|json`, `--top-k`, `--require-stored-embedding-identity`. Details: [docs/TECHNICAL.md — CLI query](docs/TECHNICAL.md#cli-cloudrift-query-phase-3). |
| `cloudrift dashboard` | Serves REST API + embedded SPA on `--port` (default `8080`), reading scans from `--output-dir` or config. Flags: `--open`, optional `--scan-id` for the browser URL. |
| `cloudrift version` | Prints version string. |

**API diff (not a separate CLI subcommand):** `GET /api/diff?old=…&new=…` — see technical doc.

---

## Dashboard

```bash
./cloudrift dashboard --output-dir ./cloudrift-output --port 8080 --open
```

Open `http://127.0.0.1:8080` (optional `?scan_id=<id>`). Routes:

| Path | Purpose |
|------|---------|
| `/overview` | Product-style dashboard with in-page modes: **Executive Summary**, **High-Signal**, **Operations** (`?view=...`) and drilldowns into findings/trust/entities |
| `/scan-control` | Start scans from the UI (profile/module/`no_http`/`neo4j`), validate AWS profile, runtime capability badges, run status + WebSocket progress (socket failure is non-fatal; polling remains active) |
| `/findings` | Paginated findings table, filters (including **`external_principal`**, **`external_account_id`** for `external_access` drill-down) |
| `/triage` | Findings view in triage mode (same data, alternate layout) |
| `/accounts` | Per-account rollups |
| `/diff` | Compare two scans |
| `/trust-report` | Trust-focused view for `external_access` findings |
| `/external-entities` | Entity-centric table: rollups matching `GET /api/scans/{id}/external-entities` |

**Theme:** The SPA supports **light** and **dark** themes. Use the header control to toggle; preference is stored in the browser as `localStorage` key `cloudrift-dashboard-theme` (default **dark**). Rebuild UI assets after UI changes (`cd dashboard && npm run build`).

**Navigation/state behavior:** The left sidebar is primary navigation. Dashboard mode (`view`) is preserved when navigating within `/overview`; entering Dashboard from non-dashboard routes defaults to Executive Summary. `scan_id` is preserved across app navigation.

### Neo4j (optional graph)

1. Run Neo4j 5+ with Bolt reachable from your machine; create a database user.
2. Set **`[neo4j]`** in TOML: `uri`, `username`, `password_env` (env var name whose value is the password). Same config keys the CLI export uses (`internal/config`).
3. Export: `cloudrift scan --neo4j` after a scan directory exists, or **`cloudrift demo generate --neo4j`** for a sample graph.
4. **Viewing data:** open **Neo4j Browser** (or Neo4j Aura workspace) → connect with the same Bolt URI and credentials → run Cypher (examples):

```cypher
MATCH (s:ScanSnapshot) RETURN s.scan_id, s.finding_count LIMIT 25;
MATCH (f:Finding) WHERE f.scan_id = $scan RETURN f.id, f.title, f.severity LIMIT 50;
```

Vectors: index name `finding_embeddings` (384 dimensions, cosine) per `internal/graph/schema.go`. If `query` returns index-missing hints, apply `graph.SchemaStatements()` DDL and re-export with embeddings populated.

---

## API overview

All under `/api` — JSON in/out. Examples:

```http
GET /api/scans
GET /api/scans/{id}/summary
GET /api/scans/{id}/external-entities?page=1&page_size=50
GET /api/scans/{id}/findings?page=1&page_size=50&module=external_access
GET /api/scans/{id}/findings/{fid}
GET /api/scans/{id}/accounts
GET /api/diff?old=older-scan&new=newer-scan
GET /api/runtime/status
POST /api/runtime/validate-profile
POST /api/scan/start
GET /api/scan/status
GET /api/scan/history
```

WebSocket: `GET /api/scan/progress` — scan-control progress events (`stage`, `message`, optional `scan_id`); loopback origins only.

**Response shape guarantee:** List-like response fields are emitted as stable empty arrays (`[]`) rather than `null` where practical (for example: `items`, `new_findings`, `resolved_findings`, `aws_profiles`, summary external-entity arrays, scan history items). Filter/meta objects remain present where modeled in envelopes.

Details, examples, and error format: [docs/TECHNICAL.md — API](docs/TECHNICAL.md#3-api-documentation).

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

The dashboard and API **only read** these artifacts.

**`latest`:** Resolved by newest `scan-metadata.json` **timestamp** (descending), with **directory name ascending** as a tie-break; malformed directories are skipped (`internal/scans`).

---

## Excel export

Library: `internal/output.WriteExcel` — workbook with **Findings**, **Cost Summary**, and **Trust Report** sheets. Wire into your workflow or extend the CLI as needed.

---

## IAM / organization

See [docs/iam-setup.md](docs/iam-setup.md) and `iam/stackset-template.yaml` for org-wide audit role deployment patterns.

---

## Scope limitation

The **`reclaimable`** verdict is only valid within the **set of accounts scanned**. If an account is excluded, bucket existence cross-checks may be incomplete—treat edge findings accordingly.

---

## Contributing / dev notes

- **Packages:** Prefer keeping AWS I/O in `collectors`, pure logic in `scorers` / `validators`, HTTP in `internal/api`.
- **Tests:** `go test ./...` includes API handler tests, scorer golden behavior, and collector fakes.
- **Frontend:** `dashboard/` — React 18, Vite5, Tailwind (`darkMode: class`), TanStack Query; `fetch('/api/...')` assumes same origin as the Go server.

---

## License / version

Version constant: `cmd/cloudrift/main.go` (`0.1.0` at time of writing). Add SPDX / license file if open-sourcing.

--------------------------------
Commands to reproduce later
Terminal 1 — backend

cd /path/to/Defcon_clouddrift
go run ./cmd/cloudrift dashboard --output-dir ./cloudrift-output --port 9090
Terminal 2 — frontend

cd dashboard && npm run dev
Use the URL Vite prints (often http://localhost:5173/ or the next free port).

If you want the proxy to use 8080 again, free that port and set target back to http://127.0.0.1:8080, then run the dashboard with --port 8080.