# Getting started

This guide is for someone who can open a terminal but may not know AWS Organizations, Neo4j, or embeddings. For a **clickable walkthrough** with diagrams, open [starter-doc.html](../starter-doc.html) in your browser. **Command cheat sheet:** [cli-commands.md](cli-commands.md).

---

## 1. Prerequisites

| Tool | Why | Required? |
| --- | --- | --- |
| **Go 1.24+** | Build `cloudrift` from source | Yes for `go build` / `make build` |
| **Node.js 20+ and npm** | Build the embedded dashboard before `go build` | Yes for full UI in dev builds; release binaries already embed UI |
| **AWS credentials** | **Real scans and `cloudrift scan` credential checks** call the AWS SDK — production value is AWS-backed | Required for live assessment (once scan populates findings); **not** needed for `demo generate` alone |
| **AWS CLI** | Helps verify credentials (`aws sts get-caller-identity`) | Recommended |
| **Neo4j 5+** | **Graph tier** — relationships, blast-radius, embeddings, `cloudrift query` | Only if you use that tier |
| **OpenAI API key** | Default embedding provider for **graph-tier** `query` when Neo4j is used | Only for that path |

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

**Why this exists:** `cloudrift demo generate` writes **deterministic** `findings.json` (and related files) so you can explore the **dashboard and reports** without calling AWS APIs for inventory. It does **not** replace a real org scan — live data still comes from **AWS APIs** once the default `scan` path is fully wired.

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

**Credentials:** this command **checks AWS** (`ensureValidSession`) before writing artifacts — AWS is part of the real path even when findings are still empty.

**What you get today:** a timestamped directory with `scan-metadata.json` and **`findings.json` with an empty array `[]`**. Collectors/scorers exist in `internal/` but default scan orchestration is not yet populating live findings.

**Graph tier export** (after scan files exist):

```bash
./cloudrift scan --output-dir ./cloudrift-output --neo4j
```

Requires `[neo4j]` in TOML and the password environment variable referenced there.

---

## 6. Open the dashboard

```bash
./cloudrift dashboard --output-dir ./cloudrift-output --port 8080 --open
```

The dashboard **runs fully from JSON** for core pages (listings, findings, diff, trust). **Graph-tier** views (blast explorer, vector query UI) need Neo4j + export; APIs return `graph_available: false` when the graph tier is off.

Overview has three modes (Executive Summary, High-Signal, Operations) via URL state, e.g. `?view=high-signal`. Theme preference is stored in `localStorage` as `cloudrift-dashboard-theme`.

---

## 7. Reports (CLI)

Supported formats: **`table`**, **`json`**, **`csv`**, **`markdown`**.

```bash
./cloudrift report --scan-id latest --format markdown --output ./report.md
```

Excel (`.xlsx`) helpers exist in `internal/output` for programmatic use; they are **not** wired to `cloudrift report` today.

---

## 8. Neo4j (graph tier)

Neo4j is **coupled** to advanced product behavior: **relationship graph**, **blast-radius** exploration, **embeddings**, **`cloudrift query`**, and headroom for **future RAG-style** workflows. **Main** operator flows still work with JSON files only.

1. Run Neo4j 5+ with Bolt reachable (Docker example in [README.md](../README.md#neo4j-graph)).
2. Add `[neo4j]` to `cloudrift.toml` (`uri`, `username`, `password_env`).
3. Run `cloudrift scan --neo4j` or `cloudrift demo generate --neo4j`.

JSON files on disk remain the **source of truth**; Neo4j is a projection.

---

## 9. `cloudrift query` (graph tier)

Hybrid retrieval over embedded finding text in Neo4j. **Retrieval only** — no LLM-generated narrative answers.

- Default embeddings: OpenAI `text-embedding-3-small` — set `OPENAI_API_KEY` (or the env name in `[embeddings].openai_api_key_env`).
- **`provider=local` is stubbed** and returns an error until a local model ships.

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
| Dashboard empty | Empty `findings.json` | Run `demo generate` or pick a scan that has findings |
| `cloudrift scan` “succeeds” but no rows in UI | Expected today | Use demo data until scan wiring lands |
| Neo4j errors | Wrong URI, auth, or firewall | Check `bolt://` host, `password_env`, Docker port 7687 |
| Query / embedding errors | Missing OpenAI key | Set key or skip `query` |
| `provider=local` fails | Stub | Use `openai` or omit query |
| `npm run build` fails | Missing deps | `cd dashboard && npm ci` |

Further detail: [technical.md](technical.md), [architecture.md](architecture.md), [iam-setup.md](iam-setup.md).
