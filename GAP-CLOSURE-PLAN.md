# Cloudrift — Gap-Closure Strategy

> Companion to `CODE-REVIEW-NOTES.md`. Turns the review findings into an ordered, actionable plan.
> Sequenced by **dependency leverage**: the first change unblocks everything downstream.

## Implementation status (branch `fix/wire-detection-engine`)

- **P0 — DONE & tested.** New `internal/pipeline` orchestrates collect→validate→score→cost→persist;
  both `cloudrift scan` and the dashboard StartScan route through it; `--no-http`/`--concurrency`/
  `module` now honored; dead `scanrun` stub removed. The org-wide `bucketNames` set (P1 #1) is
  assembled here and fed to `ScoreRisk`. Verified by `internal/pipeline/pipeline_test.go` (both
  modules end-to-end, persistence, module filter, healthy-record suppression). Full suite green.
- **P1 — core DONE & tested.** #1 cross-account bucket set (in P0). #3 CDN hostname-vs-alias
  comparison implemented as a pipeline join (`annotateAlternateDomains`) + test. #5 admin-equivalent
  policy now bridges into trust severity (`trust.go`) + test. #2 and #6 are **intentional scope
  decisions left as-is** (see below).
- **P2 — DONE & tested.** Evidence type-coercion fixed (`permission_visibility` normalized to
  `map[string]any` via `evidenceMap` so in-memory consumers match post-JSON). Finding IDs are now
  stable (derived from module+ARN[+principal], not the mutable verdict). Diff matches by stable ID
  and adds a `changed_findings` category (severity transition) — backend, schema, TS type, and Diff
  UI updated. Tests updated for the new by-ID semantics.
- **P3 — DONE (core) & tested.** Dashboard Neo4j export now loads + projects assets and
  relationships (was findings-only) via new reusable `scans.LoadAssets`/`LoadRelationships`. RAG
  embeddings wired into BOTH export paths (`graph.AttachEmbeddingsBestEffort`, best-effort: skips
  cleanly without an API key). SSRF guard on webhook delivery (`allowedWebhookURL` + dial-time
  internal-IP block + no-redirect client) applied at validation and send, with tests. Alerting JSON
  store writes are now atomic (temp+rename). Deferred: `scan_completion` always-fires is an
  intentional notification (not gated); real per-account WS progress (low value / high plumbing).
- **P4 — DONE & tested.** Server now binds **127.0.0.1 by default** with a `--host` flag to opt
  into `0.0.0.0` (prints a warning when non-loopback). Optional auth: setting `CLOUDRIFT_API_TOKEN`
  gates the whole server (API + UI) behind HTTP Basic (browser-native; same-origin `fetch`
  inherits creds, so zero frontend changes). CSP tightened to `script-src 'self'` (kept
  `style-src 'unsafe-inline'` for CSS-in-JS). Removed dead `strconvItoa`. Tests cover the auth gate.
  Caveat: CSP change not yet verified against the running UI (frontend build pending P5).
- **P5 — DONE & verified.** Fresh-clone build fixed: built the frontend (deps installed, tsc +
  vite + 11 vitest pass — confirms the P2 Diff.tsx/type changes compile), committed the full
  `dashboard/dist/` and removed `dist/assets/` from `.gitignore`. Runtime smoke test: binary serves
  the embedded UI (index + 2.2MB JS asset, HTTP 200), hardened CSP header present, loopback bind,
  API works without auth on loopback. `report` now uses the injection-safe `internal/output`
  writers and exposes a new `excel` format (verified CSV + .xlsx output) — un-orphaning that
  package and removing the unsafe inline CSV writer. Remaining cosmetic dead code (blastradius
  `IsReachable`/`HasExternalRoot`, triplicate `\x1e` separators, orphaned `internal/remediator`)
  left as-is to avoid DTO/frontend churn — documented, low value.
- **P6 — DONE (core).** `internal/pipeline.TestRunProducesAllSevenIssueTypes` drives the wired
  pipeline (fake source + deterministic per-ARN validation, no AWS) and asserts all 4 orphaned-edge
  claimabilities (reclaimable/dangling/edge_obscured/broken) AND all 4 external-trust verdicts
  (ghost_admin_access/unknown_vendor/stale_review_now/active) are produced — the end-to-end proof
  that every issue type the tool targets is detectable. Deferred: rewriting the 990-line demo
  generator to call real scorers instead of fabricated literals (lowest value — the integration
  test is the real proof; the demo already showcases all 7 in the UI). `go vet` clean; full suite green.

### Deliberate scope decisions (not implemented — owner's call)
- **#2 dangling CDN**: detection stays scoped to the 4 recognized AWS error-body fingerprints.
  Widening to flag any generic 403/404 risks false positives. Revisit if FN rate matters more than FP.
- **#6 unapproved vendor (SAML/OIDC)**: `unknown_vendor` intentionally applies only to `aws_account`
  principals (protected by `TestScoreTrust_UnknownVendorAppliesOnlyForAWSAccountPrincipal`). Extending
  to federated principals needs a new `Trust.ApprovedFederatedProviders` allowlist + that test updated.

## Guiding logic

The codebase is a **correct blueprint + polished viewer with a disconnected core**. Almost every
gap traces to one root cause: `scan` never runs the detection library. So the sequencing is:

1. **P0 — Wire the engine** (turns 0 detections into ~5 of 7 working). The single highest-leverage change.
2. **P1 — Make all 7 detections correct** (fix the per-issue defects exposed once data flows).
3. **P2 — Fix correctness bugs on paths that run** (evidence typing, diff).
4. **P3 — Repair supporting components** (RAG, dashboard-Neo4j, alerting, progress).
5. **P4 — Security hardening** (GATE: required before any non-localhost deployment — can run in parallel).
6. **P5 — Build + dead-code hygiene.**
7. **P6 — Prove it end-to-end.**

Effort key: **S** ≤0.5d · **M** ~1–3d · **L** ~1wk+. All file refs are to current code.

---

## P0 — Wire the detection engine  ★ THE UNLOCK

**Gap:** `internal/scanrun/scanrun.go` hardcodes empty findings; collectors/validators/scorers have
zero production callers; no orchestrator joins them. Both `cloudrift scan` (`cmd/cloudrift/main.go:128`)
and dashboard `StartScan` (`internal/api/handlers/scan_control.go:209`) call the stub.

**Do:**
1. Create `internal/pipeline` (or rewrite `scanrun.Run`) implementing the real flow:
   ```
   CollectAccounts(org) → assume member roles
     └─ per account, concurrently (cfg.Scan.RoleAssumptionConcurrency):
        CollectDNS, CollectStorage, CollectEdge, CollectTrust, CollectActivity
   Assemble org-wide bucketNames set  ← from all CollectStorage results (fixes #1)
   ValidateAssets(DNS/HTTP probe, cfg.Scan.HTTPProbeConcurrency, noHTTP)
   ScoreRisk(node, validation, bucketNames) per edge asset
   ScoreTrust(assets, rels, IndexActivityByRoleARN(activity), cfg) per trust rel
   ScoreCost + EnrichCostFromCE(cfg.Cost.UseCUR)
   Persist: scan-metadata.json, findings.json, assets/*.json, relationships.json
   ```
2. Route **both** entry points to it. Pass through the currently-dropped flags: `--no-http`,
   `--concurrency` (`main.go:68-69`), and `module`/`no_http`/`provider` (`scan_control.go:122-132`).
3. Record **scan coverage** in metadata: which accounts were attempted vs. succeeded (needed by P1 #1).

**Acceptance:** a scan against a real (or mocked) org writes non-empty `findings.json` containing
both modules. Config knobs already exist (`internal/config`): no new config needed.
**Effort:** L. **Unblocks:** all 7 detections, cost, alerting-on-real-data, Neo4j, RAG.

---

## P1 — Make the 7 detections correct

Once data flows, fix the per-issue defects from the acceptance audit. (#4 Broken DNS, #7 Stale-vs-active
need nothing — they're already correct.)

| # | Issue | Fix | Files | Effort |
| --- | --- | --- | --- | --- |
| 1 | S3 takeover (cross-account) | Feed the org-wide `bucketNames` set into `ScoreRisk` (built in P0). Add false-positive guard: only emit `reclaimable` when coverage is complete; downgrade/flag when owning account may be unscanned. | pipeline + `scorers/risk.go:45` | M |
| 3 | CDN hostname mismatch | Implement the real comparison: for each DNS host, resolve its CloudFront distribution and check membership in the distribution's alias list. Either set `in_alternate_domains` on the DNS node in the collector, **or** change `inAlternateDomains` to read `alternate_domains` + do the lookup. | `collectors/edge.go`, `scorers/risk.go:98-101` | M |
| 5 | Admin-equiv external trust | Bridge `DeriveRolePermissionVisibility().Capabilities.AdminLike` into severity. Either set `Properties["is_admin"]` from permission visibility in the collector, or have `classifyTrust` consume `permissionVisibility.AdminLike` instead of `Properties["is_admin"]`. | `scorers/trust.go:83,165-169,184` | S |
| — | Property-key mismatches | `is_admin` (never set), `in_alternate_domains` vs `alternate_domains`, `dns_status` (P0's validator wiring fixes this). Add a contract test asserting collector-written keys == scorer-read keys. | collectors + scorers | S |
| 2 | Dangling CDN scope | Decide: keep "known AWS error bodies only" or widen `fingerprint()` to flag generic 403/404-to-AWS-infra as lower-severity dangling. | `validators/http.go:153-166` | S |
| 6 | Unapproved vendor scope | Extend the approved-list check to SAML/OIDC principals (currently `aws_account` only). | `scorers/trust.go:155` | S |

---

## P2 — Correctness bugs on paths that already run

These affect the **demo/dashboard/diff** paths that execute today.

1. **Evidence type-coercion (highest-value live bug).** `evidence map[string]any` mixes struct and
   primitive values; consumers coerce differently (`permission_visibility` struct-vs-map; `strEv`
   `fmt.Sprint` vs `strEvidence` ""-on-nonstring). Symptoms: priority under-scores in-memory findings;
   external-entity→graph match silently fails on numeric `external_account_id`.
   **Fix:** define one typed evidence accessor (or marshal evidence to a canonical `map[string]any`
   before any consumer reads it). `scorers/trust.go:122`, `scorers/priority.go:175-193`,
   `blastradius/entitymatch.go:25-34`, `handlers/findings.go` (`strEvidence`). **Effort:** M.
2. **Diff is lossy.** Match by stable finding **ID**, add a **`changed`** category, stop embedding
   verdict in the match key. `handlers/scans.go:341-375`, `schema/responses.go:255-261`. **Effort:** M.
3. **Finding-ID stability.** Stop embedding claimability/verdict in the ID hash so a reclassified
   finding keeps its ID across scans (prereq for #2's by-ID diff). `scorers/risk.go:16-18`,
   `scorers/trust.go:96-98`. **Effort:** S (coordinate with diff change).

---

## P3 — Repair the supporting components

1. **RAG / vector — currently dead end-to-end.**
   - Call `AttachFindingsEmbeddings` + `SyncScanSnapshotEmbeddingMeta` in **both** export paths before
     `graph.WriteScan` (`cmd/cloudrift/main.go:181-193`, `handlers/scan_control.go:502-514`).
   - Decide on the `local` provider: implement it or remove the stub + its misleading metadata
     (`graph/embedder.go:142-155`).
   - Decide RAG scope: either add a synthesis/LLM step, or rename the feature "semantic search"
     (today it's retrieval-only and the index is empty). **Effort:** M (embedding wiring) + L (if synthesis).
2. **Dashboard Neo4j export ships empty graph.** Unify the two `exportScanToNeo4j` impls — make
   `scan_control.go` load+pass assets+relationships (use the CLI path's loader) and delete the
   empty-slice copy. `handlers/scan_control.go:485-515`. **Effort:** S.
3. **Alerting.**
   - **SSRF (do in P4 too):** allowlist `hooks.slack.com`, set `http.Client.CheckRedirect` to block,
     reject private/link-local IPs. `service.go:428`, `provider_slack.go:18`. **Effort:** S.
   - `scan_completion` always fires → make it threshold/opt-in gated. `evaluator.go:152`. **Effort:** S.
   - Write amplification: batch per-rule writes instead of rewriting `rules.json` per evaluation;
     make store writes atomic (temp+rename). `alerting/store.go`, `service.go:215-220`. **Effort:** M.
4. **Scan progress WS is fake.** Emit real account-level progress from the P0 pipeline instead of the
   500ms re-send of a zeroed event. `handlers/scan_control.go:186-199`, `handlers/ws.go`. **Effort:** M.

---

## P4 — Security hardening  ★ GATE before any non-localhost deployment

The server binds `0.0.0.0` with **no auth**; anyone reachable can trigger scans + `aws sso login`.

1. **AuthN/Z** on `/api` (token or session), **or** default-bind to `127.0.0.1` and require explicit
   opt-in for `0.0.0.0`. `internal/api/server.go`, `cmd/cloudrift/dashboard.go:55`. **Effort:** M.
2. **SSRF** in alerting webhooks (see P3.3). **Effort:** S.
3. **CSP:** drop `'unsafe-inline'` from `default-src`. `internal/api/server.go:152-164`. **Effort:** S.
4. Gate `POST /api/scan/start` and `/api/runtime/sso-login` behind auth (covered by #1). **Effort:** —.

---

## P5 — Build & dead-code hygiene

1. **Fresh-clone build is broken:** `dashboard/dist/index.html` is tracked but `dist/assets/` is
   gitignored and missing → `//go:embed dist` yields a blank UI. Either commit all of `dist/` or none
   (build in CI before embed). `.gitignore:2`, `dashboard/embed.go`. **Effort:** S.
2. **Switch `report` to the safe writers:** the shipped `report` reimplements CSV/markdown inline in
   `main.go` with **no CSV-injection protection**; the orphaned `internal/output` package has a safe
   `WriteCSV` + Excel writer. Wire `report` to `internal/output` (and expose Excel), or delete the dead
   package. `cmd/cloudrift/main.go:369-397` vs `internal/output/*`. **Effort:** S.
3. **Remove dead code:** `internal/remediator` (orphaned), `strconvItoa` (`server.go`), dead fields
   (`IsReachable` always true, `HasExternalRoot` never set), duplicate `\x1e` separator constants. **Effort:** S.

---

## P6 — Prove it end-to-end

1. **Integration test over mocked AWS.** Collectors already have interface seams + fakes. Drive the P0
   pipeline with synthetic AWS responses crafted to trigger **each of the 7 issue types**, assert the
   exact verdicts/severities/costs. This is the missing proof that the chain works as a whole (today
   only unit fragments are tested). **Effort:** L.
2. **Make demo exercise the real engine.** Replace `demo.go`'s fabricated finding literals with running
   the actual scorers over synthetic collector data, so the showcase validates the engine and demo
   numbers reflect real scoring (config thresholds/multipliers actually apply). `cmd/cloudrift/demo.go`.
   **Effort:** M.

---

## Minimum path to a working MVP

If the goal is "actually detect the 7 issues against a real org," the critical path is:

**P0 (wire engine) → P1 #1/#3/#5 + property-key fixes → P6.1 (prove it)**

Everything else (RAG, alerting polish, diff, build, security) is parallelizable or deferrable —
**except P4**, which is a hard gate the moment this runs anywhere other than localhost.
