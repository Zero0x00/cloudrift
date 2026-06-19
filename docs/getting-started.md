# Getting started

This guide is for someone who can open a terminal but may not know AWS Organizations, Neo4j, or embeddings. For a **clickable walkthrough** with diagrams, open [starter-doc.html](../starter-doc.html) in your browser. **Command cheat sheet:** [cli-commands.md](cli-commands.md).

---

## 1. Prerequisites

| Tool | Why | Required? |
| --- | --- | --- |
| **Go 1.24+** | Build `cloudrift` from source | Yes for `go build` / `make build` |
| **Node.js 20+ and npm** | Build the embedded dashboard before `go build` | Yes for full UI in dev builds; release binaries already embed UI |
| **AWS credentials** | **Real scans** call the AWS SDK to discover, probe, and score org assets — production value is AWS-backed | Required for live assessment; **not** needed for `demo generate` alone |
| **AWS CLI** | Helps verify credentials (`aws sts get-caller-identity`); also used to refresh an expired SSO session | Recommended |
| **Neo4j 5+** | **Graph tier** — relationships, blast-radius, embeddings, `cloudrift query` | Only if you use that tier |
| **OpenAI API key** | Default embedding provider for **graph-tier** `query` when Neo4j is used | Optional, only for that path |
| **Anthropic API key** | Optional LLM answer synthesis on top of `cloudrift query` retrieval | Optional, only for synthesized answers |

### Environment variables

| Variable | Used by | Notes |
| --- | --- | --- |
| `CLOUDRIFT_CONFIG` | all commands | Path to `cloudrift.toml` (else `./cloudrift.toml`). Equivalent to `--config`. |
| `CLOUDRIFT_NEO4J_PASSWORD` | `--neo4j` export, `query` | Default env var name referenced by `[neo4j].password_env`. |
| `OPENAI_API_KEY` | `query` embeddings | Default embedding-provider key (override via `[embeddings].openai_api_key_env`). Optional. |
| `ANTHROPIC_API_KEY` | `query` synthesis | Enables LLM answer synthesis (override via `[synthesis].api_key_env`). Optional. |
| `CLOUDRIFT_API_TOKEN` | `dashboard` | When set, gates the dashboard (API + UI) behind HTTP Basic auth. Optional. |

---

## 2. Clone and build from source

```bash
git clone https://github.com/Zero0x00/cloudrift.git
cd cloudrift
go mod download
go test ./...
```

Build the dashboard static assets, then the Go binary:

```bash
cd dashboard
npm ci
npm run build
cd ..
go build -o cloudrift ./cmd/cloudrift
```

Or use the Makefile (same outcome):

```bash
make build
```

Verify:

```bash
./cloudrift version
```

---

## 3. Try locally without AWS (demo UI path)

**Why this exists:** `cloudrift demo generate` writes **deterministic** `findings.json` (and related files) so you can explore the **dashboard and reports** without calling AWS APIs for inventory. It does **not** replace a real org scan — live data comes from a real `cloudrift scan` against your AWS org (see section 5).

```bash
./cloudrift demo generate --output-dir ./cloudrift-output
./cloudrift report --scan-id latest --format table
./cloudrift dashboard --output-dir ./cloudrift-output --port 8080 --open
```

Open `http://127.0.0.1:8080` and pick the newest scan from the list, or open `?scan_id=demo` if you generated with `--scan-id demo`.

The repo may ship sample data under `cloudrift-output/demo/`; you can regenerate with:

```bash
./cloudrift demo generate --output-dir ./cloudrift-output --scan-id demo --timestamp 2026-04-18T18:00:00Z
```

---

## 4. Run with AWS credentials

Cloudrift uses the **AWS SDK default credential chain** unless you pin a profile name in config.

### Environment variables (example)

```bash
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
export AWS_REGION=us-east-1
```

### Named profile via TOML (not a CLI flag)

There is **no** `--profile` flag on `cloudrift` commands. Set the profile in `cloudrift.toml`:

```toml
[aws]
management_profile = "my-readonly-profile"
```

If `management_profile` is **empty**, the SDK uses the default chain (which includes `AWS_PROFILE` when you export it, same as other AWS tools).

### Config file location

Search order: path in `CLOUDRIFT_CONFIG`, else `./cloudrift.toml` next to where you run the binary.

---

## 5. Run a real scan

```bash
./cloudrift scan --output-dir ./cloudrift-output
```

**Credentials:** this command **checks AWS** (`ensureValidSession`) before doing any work. If your SSO session has expired, it attempts `aws sso login` and retries; if the AWS CLI isn't installed, it prints the manual login command.

**What you get:** a timestamped directory with `scan-metadata.json`, `findings.json` (real discovered + scored findings), per-collector files under `assets/`, and `relationships.json`. The pipeline collects org inventory, probes edge assets over HTTP, and scores them.

**Useful flags:**

```bash
./cloudrift scan --no-http              # skip HTTP probing
./cloudrift scan --concurrency 100      # HTTP probe concurrency (default 50)
```

**Graph tier export** (writes the scan, then projects it into Neo4j):

```bash
./cloudrift scan --output-dir ./cloudrift-output --neo4j
```

Requires `[neo4j]` in TOML and the password env var it references (default `CLOUDRIFT_NEO4J_PASSWORD`).

---

## 6. Open the dashboard

```bash
./cloudrift dashboard --output-dir ./cloudrift-output --port 8080 --open
```

The dashboard binds **`127.0.0.1` by default**. To expose it on the network, pass `--host 0.0.0.0`; the command warns you to set auth first. Setting `CLOUDRIFT_API_TOKEN` gates the whole server (API + UI) behind HTTP Basic auth — the token is the password (username is ignored):

```bash
CLOUDRIFT_API_TOKEN=s3cr3t ./cloudrift dashboard --host 0.0.0.0
```

The dashboard **runs fully from JSON** for core pages (listings, findings, diff, trust). **Graph-tier** views (blast explorer, vector query UI) need Neo4j + export; APIs return `graph_available: false` when the graph tier is off.

Overview has three modes (Executive Summary, High-Signal, Operations) via URL state, e.g. `?view=high-signal`. Theme preference is stored in `localStorage` as `cloudrift-dashboard-theme`.

---

## 7. Reports (CLI)

Supported formats: **`table`**, **`json`**, **`csv`**, **`markdown`**, **`excel`** (`.xlsx`), **`sarif`** (SARIF 2.1.0, for GitHub code scanning). `table` prints to stdout; the others write a file — to `--output` if given, otherwise `<scan-dir>/report.<ext>`.

```bash
./cloudrift report --scan-id latest --format markdown --output ./report.md
./cloudrift report --format excel       # writes <scan-dir>/report.xlsx
./cloudrift report --format sarif        # writes <scan-dir>/report.sarif
```

---

## 8. Neo4j (graph tier)

Neo4j is **coupled** to advanced product behavior: **relationship graph**, **blast-radius** exploration, **embeddings**, **`cloudrift query`**, and headroom for **future RAG-style** workflows. **Main** operator flows still work with JSON files only.

1. Run Neo4j 5+ with Bolt reachable (Docker example in [README.md](../README.md#neo4j-graph)).
2. Add `[neo4j]` to `cloudrift.toml` (`uri`, `username`, `password_env`).
3. Run `cloudrift scan --neo4j` or `cloudrift demo generate --neo4j`.

JSON files on disk remain the **source of truth**; Neo4j is a projection.

---

## 9. `cloudrift query` (graph tier)

Hybrid retrieval over embedded finding text in Neo4j, with **optional** LLM answer synthesis grounded in the retrieved findings.

- Default embeddings: OpenAI `text-embedding-3-small` — set `OPENAI_API_KEY` (or the env name in `[embeddings].openai_api_key_env`).
- **`provider=local` is stubbed** and returns an error until a local model ships.
- **Answer synthesis** is enabled when a synthesis provider API key is configured (`[synthesis]` in TOML; default provider Anthropic, key from `ANTHROPIC_API_KEY`). The retrieved findings are passed to the LLM to compose a grounded answer that cites finding IDs. Without a key, output is **retrieval-only**.
- Output `--format` is `table` (human summary) or `json`. `--top-k` defaults to 10 (max 100).

Example:

```bash
./cloudrift query "show high severity external trust" --scan-id latest --output-dir ./cloudrift-output
```

---

## 10. Troubleshooting

| Symptom | Likely cause | What to try |
| --- | --- | --- |
| `NoCredentialProviders` / auth errors | No credentials in chain | Run `aws sts get-caller-identity` with same profile/env |
| `AccessDenied` on `AssumeRole` | Trust policy, external ID, or role name | Compare member account role to `iam/stackset-template.yaml` |
| Dashboard empty | Empty `findings.json` | Pick a scan that has findings, or run `demo generate` |
| Dashboard returns 401 | `CLOUDRIFT_API_TOKEN` is set | Provide the token as the Basic-auth password, or unset the env var |
| Neo4j errors | Wrong URI, auth, or firewall | Check `bolt://` host, `password_env`, Docker port 7687 |
| Query / embedding errors | Missing OpenAI key | Set `OPENAI_API_KEY` or skip `query` |
| Query returns hits but no narrative answer | No synthesis key | Set `ANTHROPIC_API_KEY` (or `[synthesis].api_key_env`) to enable LLM answers |
| `provider=local` fails | Stub | Use `openai` or omit query |
| `npm run build` fails | Missing deps | `cd dashboard && npm ci` |

Further detail: [technical.md](technical.md), [architecture.md](architecture.md), [iam-setup.md](iam-setup.md).
