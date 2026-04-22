# Cloudrift Phase 1 Implementation Plan (Historical)

## Scope (original plan)

- Commands: `scan`, `report`, `diff`, `remediate`, `version`
- Collectors: org, dns, storage, edge
- Validator: DNS + HTTP/TLS probes with concurrency control and `--no-http`
- Scorers: risk + cost
- Outputs: table/json/csv/markdown
- IAM and docs baseline for onboarding

## Critical Correctness Risks

1. Prevent reclaimable false positives via cross-account bucket checks.
2. Preserve scope transparency for account coverage limitations.
3. Support S3 website endpoint regional variations.
4. Keep fingerprint matching deterministic and test-backed.
5. Enforce concurrency safety and timeout controls.

## Delivery Order

1. Tree/module scaffolding
2. Models/contracts
3. Session + org collection
4. DNS/storage/edge collectors
5. Validator engine
6. Risk + cost scoring
7. Remediation + outputs
8. CLI wiring
9. IAM/docs/CI validation

---

## Current command reality (repository)

This file is preserved as a historical planning artifact. The active CLI command tree today is:

- `scan`
- `report`
- `dashboard`
- `query` (Phase 3 retrieval-only)
- `demo generate`
- `version`

Planned Phase 1 commands `diff` and `remediate` are not currently exposed as CLI subcommands.
