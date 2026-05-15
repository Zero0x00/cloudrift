# CLI commands

Short reference for each `cloudrift` subcommand. Global flag: **`--config`** path to `cloudrift.toml` (optional).

| Command | What it does | Main flags |
| --- | --- | --- |
| **`cloudrift scan`** | Validates AWS credentials, creates `output_dir/<scan-id>/` with `scan-metadata.json` and (today) **empty** `findings.json`. Intended path calls AWS for full inventory once orchestration is complete. | `--output-dir`, `--neo4j` (export graph projection after write). `--no-http`, `--concurrency` are registered but **not wired**. |
| **`cloudrift demo generate`** | Writes a **deterministic** populated scan (findings, relationships, assets) for **UI and report demos** — does **not** call live AWS collectors. | `--output-dir`, `--scan-id`, `--neo4j`, `--dense` (richer trust graph for blast demos). |
| **`cloudrift report`** | Reads `findings.json` for a scan; outputs **table, json, csv, or markdown**. | `--scan-id` (default `latest`), `--output-dir`, `--format`, `--output`. |
| **`cloudrift dashboard`** | Serves embedded SPA + REST API reading scans under `output_dir`. | `--port`, `--open`, `--scan-id`, `--output-dir`. |
| **`cloudrift query`** | **Graph tier:** hybrid retrieval over Neo4j (needs projection + embeddings). **Retrieval only** — no synthesized answers. | `--scan-id`, `--output-dir`, `--query` or positional text, `--format table|json`, `--top-k`, `--require-stored-embedding-identity`, `--legacy-retrieval`. |
| **`cloudrift version`** | Prints build version. | — |

For narrative context (AWS vs demo, Neo4j graph tier), see [starter-doc.html](../starter-doc.html) (sections **CLI commands**, **Kinds of issues**, **Neo4j & graph tier**).
