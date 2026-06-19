# Contributing to Cloudrift

Thank you for helping. This project values **accurate docs**, **small focused changes**, and **honesty about gaps**. The detection engine is now wired: a scan runs the full collectors → validators → scorers → persistence pipeline (`internal/pipeline`) and writes real `findings.json`.

---

## Philosophy

- **JSON on disk is truth** for findings; APIs serve that data; **Neo4j** is the **graph-tier** projection for advanced analysis.
- **No surprise permissions** — read-oriented audit role, not admin-by-default.
- **Prefer tests** for scorers and API response shapes (stable `[]` not `null` for lists).

---

## Local development

```bash
go mod download
go test ./...
go vet ./...
```

Dashboard (when you touch `dashboard/`):

```bash
cd dashboard && npm ci && npm run build && cd ..
go build -o cloudrift ./cmd/cloudrift
```

Or `make build` / `make test` from the repo root.

---

## Documentation

- **Beginner / narrative:** [starter-doc.html](starter-doc.html) (single HTML; edit directly).
- **Setup steps:** [docs/getting-started.md](docs/getting-started.md).
- **Architecture (plain language):** [docs/architecture.md](docs/architecture.md).
- **Deep implementation:** [docs/technical.md](docs/technical.md).
- **CLI reference:** [docs/cli-commands.md](docs/cli-commands.md).
- **Historical spec:** [tech-spec-v2.md](tech-spec-v2.md) — note intentional deviations in `docs/technical.md` when behavior changes.

After changing **Go or TS/JS** source, refresh the AST graph (if you use graphify in this repo):

```bash
.venv-graphify/bin/graphify update .
```

---

## Where to add things

| Change | Location |
| --- | --- |
| New AWS read / inventory | `internal/collectors/` |
| Risk or trust scoring | `internal/scorers/` |
| DNS/HTTP validation | `internal/validators/` |
| Scan orchestration (collect → validate → score → persist) | `internal/pipeline/` (the single entry point both CLI `scan` and the dashboard scan-control handler call; AWS collection sits behind the `Source` interface in `source_aws.go` so the rest is testable without AWS) |
| LLM answer synthesis for `cloudrift query` | `internal/synth/` (pluggable `Synthesizer`; `anthropic` is the operational provider, no-op fallback when no API key) |
| REST handler or DTO | `internal/api/handlers/`, `internal/api/server.go` |
| Scan directory layout rules | `internal/scans/` |
| CLI subcommand / flags | `cmd/cloudrift/` (ensure flags are either wired or documented as stub) |
| Dashboard page | `dashboard/src/pages/`, route in `dashboard/src/App.tsx` |
| New `report` format | Wire in `cmd/cloudrift` report path + `internal/output/` |
| Tests | `_test.go` next to package; dashboard: `npm run test:run` |

> Note: `internal/scanrun` was removed; its orchestration role is now `internal/pipeline`.

---

## How to write tests

- **Go:** table-driven tests, golden files where output is stable, fakes for AWS in collectors.
- **API:** assert JSON keys and types; list fields should be arrays, not `null`.
- **Pipeline:** `internal/pipeline/pipeline_test.go` drives the full pipeline through a fake `Source` (no AWS, no network — validation is stubbed via the package seam), asserting persisted `findings.json`/`scan-metadata.json` across both detection modules. Extend it when you change orchestration or persistence.
- **Synthesis:** `internal/synth/synth_test.go` uses `httptest` to stand in for the provider API — no API key required — covering request shape, refusal/degrade, and the no-op-without-key path.
- **Avoid** giant integration tests unless necessary; unit-test pure functions first.

---

## Scope creep

Match the issue or PR description. Do not refactor unrelated packages or “clean up” without agreement. A 20-line fix beats a 200-line cosmetic diff.

---

## Good first issues (ideas)

- New DNS or HTTP error **fingerprint** in validators (with tests).
- New **pricing** or cost-estimate rule for an asset type (document assumptions).
- Dashboard **empty state** copy or accessibility for a page that assumes data.
- **Glossary** or starter-doc clarification (still no marketing fluff).
- **Unit test** for a scorer edge case (null evidence, boundary severity).

---

## Security

Do not commit secrets. If you find a vulnerability, report responsibly per project security policy (or open a private advisory on GitHub if enabled).
