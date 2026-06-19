# CLI commands

Reference for each `cloudrift` subcommand.

**Global flag:** `--config <path>` — path to `cloudrift.toml` (optional). When unset, the binary searches `CLOUDRIFT_CONFIG`, then `./cloudrift.toml`.

| Command | What it does |
| --- | --- |
| **`cloudrift scan`** | Validates AWS credentials, then runs real discovery + scoring against the org and writes `output_dir/<scan-id>/` (`scan-metadata.json`, `findings.json`, `assets/`, `relationships.json`). |
| **`cloudrift demo generate`** | Writes a **deterministic** populated scan (findings, relationships, assets) for UI and report demos — does **not** call live AWS collectors. |
| **`cloudrift report`** | Reads `findings.json` for a scan and renders it as table, json, csv, markdown, excel, or sarif. |
| **`cloudrift dashboard`** | Serves the embedded SPA + REST API over scans under `output_dir`. |
| **`cloudrift query`** | Vector retrieval over the Neo4j projection, with optional LLM answer synthesis grounded in the retrieved findings. |
| **`cloudrift version`** | Prints the build version. |

---

## `cloudrift scan`

Runs the full pipeline: AWS credential check (`ensureValidSession`, which can auto-trigger `aws sso login` on an expired SSO session) → collect inventory → score → write findings.

| Flag | Default | Description |
| --- | --- | --- |
| `--output-dir` | config `output_dir` (`./cloudrift-output`) | Directory for `<scan-id>/` output. |
| `--no-http` | `false` | Skip HTTP probing during discovery (honored). |
| `--concurrency` | `50` | HTTP probe concurrency (honored — overrides `scan.http_probe_concurrency`). |
| `--neo4j` | `false` | After writing scan files, export the projection to Neo4j (requires `[neo4j]` config + the referenced password env var). |

```bash
cloudrift scan --output-dir ./cloudrift-output
cloudrift scan --no-http --concurrency 100
cloudrift scan --neo4j
```

---

## `cloudrift demo generate`

Subcommand of `cloudrift demo`. Writes a deterministic 5-account demo dataset without any AWS access.

| Flag | Default | Description |
| --- | --- | --- |
| `--output-dir` | config `output_dir` (`./cloudrift-output`) | Output directory. |
| `--scan-id` | `demo-<UTC timestamp>` | Fixed scan directory name (must satisfy safe-scan-id rules). |
| `--neo4j` | `false` | Export the generated demo projection to Neo4j. |
| `--dense` | `false` | Add deterministic dense cross-account trust chains for richer blast-radius paths. |

```bash
cloudrift demo generate --output-dir ./cloudrift-output
cloudrift demo generate --scan-id demo --dense
```

---

## `cloudrift report`

Loads `findings.json` for the resolved scan and renders it. `table` prints to stdout; every other format writes a file (to `--output` if set, else `<scan-dir>/report.<ext>`).

| Flag | Default | Description |
| --- | --- | --- |
| `--scan-id` | `latest` | Scan ID, or `latest` (newest by `scan-metadata.json` timestamp; tie-break directory name ascending). |
| `--format` | `table` | `table` \| `json` \| `csv` \| `markdown` \| `excel` \| `sarif` (`xlsx` is accepted as an alias for `excel`). |
| `--output` | — | Explicit output path; overrides the default `<scan-dir>/report.<ext>`. |
| `--output-dir` | `./cloudrift-output` | Directory containing scan subfolders. |

- `excel` → `.xlsx` workbook.
- `sarif` → SARIF 2.1.0, suitable for GitHub code scanning.

```bash
cloudrift report --scan-id latest --format table
cloudrift report --format markdown --output ./report.md
cloudrift report --format sarif
```

---

## `cloudrift dashboard`

Serves the embedded SPA and REST API. Binds loopback (`127.0.0.1`) by default; pass `--host` to expose it on the network. Set `CLOUDRIFT_API_TOKEN` to gate the whole server (API + UI) behind HTTP Basic auth (the token is the password; username is ignored).

| Flag | Default | Description |
| --- | --- | --- |
| `--port` | `8080` | HTTP listen port (1–65535). |
| `--host` | `127.0.0.1` | Bind address. Set to `0.0.0.0` to expose on the network — prints a warning unless `CLOUDRIFT_API_TOKEN` is set. |
| `--open` | `false` | Open the dashboard in the default browser after startup (loopback URLs only). |
| `--scan-id` | — | Optional default scan id, appended to the browser-open URL as a `scan_id` query param. |
| `--output-dir` | config `output_dir` (`./cloudrift-output`) | Directory containing scan output. |

```bash
cloudrift dashboard --output-dir ./cloudrift-output --port 8080 --open
CLOUDRIFT_API_TOKEN=s3cr3t cloudrift dashboard --host 0.0.0.0
```

---

## `cloudrift query`

Embedding-backed hybrid retrieval against Neo4j for a single scan. Requires scan output on disk (metadata), working Neo4j config, an embeddings provider (OpenAI by default), and an exported graph with vectors.

When a synthesis provider API key is configured (`[synthesis]` in config; `ANTHROPIC_API_KEY` by default), the retrieved findings are passed to an LLM to compose a grounded answer that cites finding IDs. Without a key, output is retrieval-only.

Query text comes from positional args **or** `--query` (not both).

| Flag | Default | Description |
| --- | --- | --- |
| `--scan-id` | `latest` | Scan ID, or `latest` (newest by `scan-metadata.json` timestamp; tie-break directory name ascending). |
| `--output-dir` | `./cloudrift-output` | Directory containing scan subfolders. |
| `--query` | — | Query text (optional if positional `QUERY_TEXT` is given). |
| `--format` | `table` | `table` (human retrieval summary) \| `json`. |
| `--top-k` | `0` (effective default 10, max 100) | Max findings after scan scoping; also scales the vector probe budget. |
| `--require-stored-embedding-identity` | `false` | Reject scans without `embedding_provider`/`dimensions` in `scan-metadata.json`. |
| `--legacy-retrieval` | `false` | Use the legacy retrieval-only query mode. |

```bash
cloudrift query "show high severity external trust" --scan-id latest
cloudrift query --query "dangling DNS records" --format json
```

---

## `cloudrift version`

Prints the build version. No flags.

---

For narrative context (AWS vs demo, Neo4j graph tier), see [starter-doc.html](../starter-doc.html) (sections **CLI commands**, **Kinds of issues**, **Neo4j & graph tier**).
