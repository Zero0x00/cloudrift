# Cloudrift — Code Review Notes

> Living notes from a code-grounded review. Claims here are verified against source, not docs.
> Format: each section cites the file/lines it was derived from.

---

## 0. Headline context (verified)

- `cloudrift scan` is a **stub**. `cmd/cloudrift/main.go:128` → `scanrun.Run` (`internal/scanrun/scanrun.go`)
  hardcodes `findings := []models.Finding{}`. It writes `scan-metadata.json` (FindingCount: 0) + empty
  `findings.json` and returns. **No collectors/validators/scorers are called on the scan path.**
- `scan` registers `--no-http` and `--concurrency` flags (`main.go:68-69`) that are **never read** (dead flags).
- The scoring pipeline below is **real, tested code in `internal/`, but orphaned** from the `scan` entry point.
  Today it is exercised only by `demo generate` and unit tests.

---

## 1. The two finding modules

There are exactly two `Module` values, each produced by a different scorer:

| Module | Scorer | Entry func | Claimability used? |
| --- | --- | --- | --- |
| `orphaned_edge` | `internal/scorers/risk.go` | `ScoreRisk` | Yes (full enum) |
| `external_access` | `internal/scorers/trust.go` | `ScoreTrust` | No — always `ClaimUnknown` |

---

## 2. `Finding.Evidence` structure (the `map[string]any`)

`Evidence` is an untyped `map[string]any`. The keys differ **per module**.

### 2a. Orphaned-edge evidence — `risk.go:30-35`
```
dns_status   string  — from validator: resolved | nxdomain | timeout | servfail | unknown
http_status  int     — HTTP status from HEAD/GET probe (0 if no HTTP)
fingerprint  string  — error fingerprint (see §3a); "" if none
bucket_name  string  — first label of DNS target value (node.Properties["value"] split on ".")
```

### 2b. External-trust evidence — `trust.go:109-123`
```
role_arn              string
external_principal    string  — principal_value or principal Name
principal_type        string  — e.g. "aws_account" (others not escalated; see §4)
external_account_id   string  — 12-digit, parsed from root ARN or bare id
days_since_used       int     — -1 means never used / missing telemetry
verdict               string  — trustCondition: ghost_admin_access | unknown_vendor | stale_review_now | aging | active
reason                string  — human explanation of verdict
admin_eval_state      string  — true | false | unknown (from role.Properties["is_admin"])
is_admin              bool
unknown_vendor        bool
activity_source       string  — "iam:getrole:role_last_used"
activity_status       string  — observed | missing_join | iam_never_used
permission_visibility models.RolePermissionVisibility  (NESTED OBJECT — see §6 hazard)
```

> NOTE: `Finding.Embedding []float32` is `json:"-"` (never serialized to findings.json). Only used
> in-memory before Neo4j vector export.

---

## 3. Claimability + Severity decision logic

### 3a. Orphaned edge — `risk.go:classifyRisk` (lines 39-68)
Feeds off `validators.ValidationResult` (`validators/http.go`). Evaluated **in order**, first match wins:

| # | Condition | Claimability | Severity |
| --- | --- | --- | --- |
| 1 | resolved + fingerprint=`s3_bucket_deleted` + target=s3_website + bucket NOT in any scanned account | `reclaimable` | **critical** |
| 1b | same but bucket exists in a scanned account | `dangling` | high |
| 2 | resolved + any non-empty fingerprint | `dangling` | high |
| 3 | resolved + CloudFront signal + alias not in distro alt-domains | `edge_obscured` | medium |
| 4 | dns = nxdomain / timeout / servfail | `broken` | low |
| 5 | fallthrough | `unknown` | info |

Validator fingerprints — `validators/http.go:fingerprint` (153-166), matched on 512-byte body snippet:
- `s3_bucket_deleted` ← body `<Code>NoSuchBucket</Code>`
- `s3_bucket_exists_private` ← 403 + Server header contains "s3"
- `cloudfront_origin_error` ← body "The request could not be satisfied"
- `aws_endpoint_controlled` ← body `<Code>InvalidClientTokenId</Code>`

### 3b. External trust — `trust.go:classifyTrust` (136-172)
Severity is computed by escalation (takes the **max** via `severityRank`):
1. **Base** from activity (`activitySeverity`, 174-182):
   - daysSinceUsed == -1 OR > ghostThreshold(365) → **high**, `stale_review_now`
   - between staleThreshold(90) and ghost → **medium**, `aging`
   - below stale → **low**, `active`
2. **unknown_vendor** bump: principal_type=`aws_account` + account not in `cfg.Trust.ApprovedExternalAccounts`
   → escalate to **high**, verdict `unknown_vendor` (only if higher than base)
3. **ghost_admin** bump: `is_admin=true` → escalate to **critical**, verdict `ghost_admin_access`
   (only if higher than current)

Thresholds default to 90 / 365 if config ≤ 0 (`trust.go:146-151`).

---

## 4. Permission-tier classification — `scorers/permission_visibility.go`

`DeriveRolePermissionVisibility` reads `role.Properties["attached_policy_names"]` and
`["inline_policy_documents"]`. Output struct = `models.RolePermissionVisibility`.

Tier decision (157-178), first match:
- `admin` (high conf) — wildcard `Action:* + Resource:*`, OR attached `AdministratorAccess`
- `privileged` (med) — adminLike, OR (iam-write AND can-assume-role)
- `scoped` (med) — any single elevated cap (iam-write | s3-write | cloudfront-control | assume-role)
- `limited` (med) — has allow statements, no elevated flags
- `unknown` (low) — no evidence

Capability flags from action prefix matching (84-107): `iam:create/update/put/delete/attach/detach/passrole/set/*`
→ iamWrite; `s3:put/delete/*` → s3Write; `cloudfront:create/update/delete/...` → cloudFrontControl;
`sts:assumerole|sts:*` → canAssumeRole. adminLike = wildcardAll OR (assume+iamWrite+cloudFront).

Explicit honesty in the model: `AnalysisMode = "attached_names_plus_inline_docs"`. It does **NOT**
fetch/inspect managed-policy *documents* — only matches a hardcoded list of 4 managed policy *names*
(`AdministratorAccess`, `IAMFullAccess`, `AmazonCloudFrontFullAccess`, `AmazonS3FullAccess`, lines 116-133).
Name-heuristic use without doc inspection downgrades confidence to low (191-196). Parse failure forces
`unknown` unless already admin (180-184). `NotAction` statements set `ComplexPolicyDetected` and are skipped.

---

## 5. Cost model — `scorers/cost.go`

### Static baseline `ScoreCost` (39-64)
directCost by asset type: DNS $0.50, S3 $0.23, CloudFront $35, ACM $7, else $0
(→ IAM roles / external principals get **$0**). riskCost = directCost × multiplier
(reclaimable ×5, dangling ×3, else ×1).

### CE enrichment `EnrichCostFromCE` (66-135) — only if `cfg.Cost.UseCUR`
- Pulls last-30d UnblendedCost grouped by LINKED_ACCOUNT + SERVICE.
- Maps finding→service by ARN substring (`serviceForFinding`, 176-199); **external_access excluded** (returns 0).
- Distributes each account+service monthly cost **evenly across matching findings** (avoids additive
  double-counting) then re-applies multiplier (`multiplierForFinding`).
- All failures (cfg load, CE call, empty map) degrade gracefully → keep static pricing, warn to stderr.

---

## 6. Priority ranking — `scorers/priority.go`

`PriorityScore = severityPoints + claimabilityPoints + riskCostPoints + externalExposurePoints`
(NOT normalized; pure relative rank). Tie-break: severity rank → risk cost → ID (`PriorityLess`).
- severity: crit 100 / high 72 / med 48 / low 28 / info 12
- claimability: reclaimable 38 / dangling 30 / broken 24 / edge_obscured 18 / unknown 6
- riskCost: min(42, monthlyUSD × 0.12)
- externalExposure (external_access only): base 10 +adminLike 22 +stale 14 +privileged 12
  +unknown_vendor 9 +unknown principal 5, capped at 52

---

## 7. Observations / potential issues (to validate further)

1. **[Consistency hazard] Nested struct vs map in evidence.** `trust.go:122` stores
   `permission_visibility` as a typed `models.RolePermissionVisibility` **struct**, but priority.go
   consumers (`evidenceAdminLike`, `evidenceTrustClassification`, priority.go:175/183) type-assert it to
   `map[string]any`. On an **in-memory** finding straight from `ScoreTrust`, those assertions FAIL →
   external-exposure points for admin-like/privileged are silently 0. It only works after a JSON
   round-trip (disk → reload), which is the normal API/report path. Fragile coupling; demo/in-process
   paths could under-score. WORTH CONFIRMING with a test.

2. **[Unused signal] `s3_bucket_exists_private` (403) fingerprint** is produced by the validator but
   never distinguished in `classifyRisk` — it just falls into the generic "resolved + fingerprint != ''"
   → dangling/high (risk.go:53). A private-but-existing bucket is treated identically to a controlled
   AWS error body.

3. **[Scalability] Validator goroutine fan-out.** `validators/http.go:60-71` launches one goroutine
   **per node up front**; the semaphore only caps in-flight HTTP, not goroutine count. Fine for hundreds,
   wasteful for very large inputs.

4. **[Redundant connection] TLS probe** (`http.go:128-134`) opens a *second* TCP/TLS connection separate
   from the HTTP probe just to set `TLSValid`. Doubles connections for https targets.

5. **[Diff stability] Finding IDs embed the verdict.** Edge ID = `sha256(ARN|title)[:12]` where title
   contains claimability (risk.go:16-18); trust ID = `sha256(roleARN|targetARN|verdict)[:12]`
   (trust.go:96-98). If a finding's classification changes between scans, its **ID changes** → the Diff
   feature will show it as remove+add rather than a changed severity. Confirm against `Diff` logic.

6. **[Scope] unknown_vendor only for `aws_account` principals** (trust.go:155). Service/federated/SAML
   external principals never get the unknown-vendor escalation, even if untrusted.

7. **[Dup logic] Two multiplier code paths** — inline switch in `ScoreCost` (53-62) vs
   `multiplierForFinding` (205-217). They currently agree; risk of drift if one is edited.

---

# PART II — Full-system wiring review

> Synthesized from a parallel subsystem sweep. Each subsystem section (A–H) is below;
> this top block is the cross-cutting picture that only emerges when you connect them.

## ★ The single most important finding: the entire detection engine is orphaned

Trace the production data flow and it collapses to almost nothing real:

```
cloudrift scan ───────┐
dashboard "Start Scan"─┴──► scanrun.Run() ──► writes empty findings.json. THE END.

internal/collectors/*  ──► called by NOTHING in production (only their own tests)
internal/scorers/*     ──► ScoreRisk/ScoreTrust called by NOTHING in production (only tests)
internal/validators/*  ──► called by NOTHING in production (only tests)
internal/remediator/*  ──► imported by NOTHING (dead package)
internal/output/*      ──► imported by NOTHING (dead package; report cmd reimplements inline)

demo generate ─────────► FABRICATES findings as hardcoded literals (demo.go),
                         also does NOT call scorers/collectors.
```

So **every populated finding a user ever sees is hand-written demo data.** No AWS is ever
scanned, no risk is ever scored, on any code path. The ~10k LOC of collectors+scorers+validators
is a tested-but-disconnected library. (Verified independently by the collectors, scorers, and
demo agents via repo-wide grep for callers.)

## ★ Even if the pipeline were wired, it would not work end-to-end

The collector→scorer contract is broken by **property-key mismatches** (collectors agent §C.4):

| Scorer reads (`Properties[...]`) | Collector actually writes | Effect |
| --- | --- | --- |
| `is_admin` (trust.go:185) | never written | "ghost admin → critical" path is **dead** under real data |
| `in_alternate_domains` (risk.go:99) | writes `alternate_domains` | edge_obscured check never sees data |
| `dns_status` (risk.go via validator) | hardcoded `"unknown"` (dns.go:83) | classifier needs the validator, which the stub never runs |
| `external_account_id` (trust.go:215) | never written | recovered by regex only for `aws_account` principals |

These would surface immediately if anyone wired collectors→scorers — strong signal the chain
has **never been run end-to-end**, only unit-tested in fragments.

## ★ The graph/embeddings/RAG tier is also non-functional end-to-end

(graph agent §D.3) `AttachFindingsEmbeddings` is **never called** on any export path, and
`Finding.Embedding` is `json:"-"` so it never round-trips disk. Therefore: no `:Finding` ever
gets an embedding → the vector index is always empty → **every RAG/`query` vector search returns
zero candidates** (`RAGEmptyHintNoVectorCandidates`). The default `cloudrift query` (v2) sidesteps
this by composing answers from JSON findings with `fmt.Sprintf` templates — **no LLM synthesis
anywhere** in the codebase. So "vector search / RAG" is built, unit-tested, and dead in practice.

## ★ Cross-cutting consistency bug: evidence is `map[string]any` with mixed value types

Corroborated from two directions:
- Part I §7.1: trust.go stores `permission_visibility` as a **struct**, priority.go reads it as
  `map[string]any` → fails on in-memory findings, only works post-JSON-roundtrip.
- Blastradius agent §E.2: the handler's `strEvidence` returns `""` for non-string values, but
  blastradius's `strEv` does `fmt.Sprint(v)`. A numeric `external_account_id` (JSON → `float64`)
  keys differently on each side → **external-entity → graph match silently fails**
  (`ReasonNoGraphProjection`).

Root cause is the same: an untyped evidence bag consumed by multiple packages with divergent
type-coercion rules. This is the highest-value *real* bug class (affects the paths that DO run).

## ★ Security posture (server is network-reachable)

- **No auth on any route** (api agent §A.4). Server binds `0.0.0.0` (dashboard.go banner + bind),
  including `POST /api/scan/start` and `POST /api/runtime/sso-login` (can shell out `aws sso login`).
  Anyone who can reach the port can drive AWS credential flows.
- **SSRF in Slack webhooks** (alerting agent §F.8): only validation is `HasPrefix("https://")`.
  No host allowlist, no `CheckRedirect` block, no private-IP guard → `https://169.254.169.254/...`
  is POSTed verbatim. Webhooks are user-editable via the API.
- **CSP allows `'unsafe-inline'` in `default-src`** (api §A.4) — weakens the otherwise-present
  security headers. XSS surface is small in practice (frontend §H.5: `SafeExternalLink` is solid,
  no `dangerouslySetInnerHTML`), but the CSP undercuts it.
- Mitigations present: WebSocket origin is localhost-restricted; browser auto-open is loopback-only;
  path traversal is genuinely blocked (`IsSafeScanID`, scans agent §G.2); CSV-injection guard exists
  (but only in the *orphaned* output package — the shipped report writer has none, §G.3).

## ★ Build is broken on a fresh clone

(frontend agent §H.5) `.gitignore` ignores `dashboard/dist/assets/` but `dashboard/dist/index.html`
**is tracked** and references hashed bundles that aren't committed (and don't currently exist on
disk). `embed.go` does `//go:embed dist`. So a fresh clone either fails `go build` or embeds an
index.html pointing at 404'd assets → blank dashboard. Only works after a local `npm run build`.

## ★ Two divergent implementations of the same operation

`exportScanToNeo4j` exists twice (graph §D.5): the CLI copy (`main.go`, injectable connector,
tested, exports assets+rels+findings) vs the API copy (`scan_control.go`, inline driver, untestable,
**always passes empty asset/rel slices**). API-triggered Neo4j export produces a degraded graph
(no `:Asset` nodes, no relationships, no `AFFECTS`/`OWNED_BY`).

## ★ Diff is lossy (affects a path that runs)

(api agent §B.3) `DiffScans` matches findings by `(lower(Title), lower(AffectedARN))` — **not by ID**.
There is **no "changed" category** (schema only has new/resolved/unchanged). Since edge titles embed
claimability and trust titles embed verdict (Part I §7.5): a severity-only change → reported
`unchanged` (silently dropped); a reclassification → reported as remove+add. Same-scan (Title,ARN)
duplicates collide in the index map and under-count.

---

## A. HTTP API server wiring (internal/api)

- **Bootstrap**: `dashboard` cmd → `api.StartServer` → `NewRouter` (chi: RequestID, RealIP,
  Recoverer, custom `securityHeaders`) → `apiRouter` instantiates all deps **eagerly, once**.
  `config.Load` error is **ignored** (`cfg, _ :=`, server.go:36) → silent degraded wiring.
- **Neo4j**: `blastradius.TryConnect` with 4s timeout; nil driver = normal degraded mode. The driver
  is **never closed** (no graceful shutdown; `http.ListenAndServe` with default unbounded timeouts).
- **Scan-control runtime** (`handlers/scan_control.go`): all state **in-memory**, lost on restart;
  history capped at 10. `StartScan` → 202 + `go runScanAsync` which does a **real** STS
  `GetCallerIdentity` check, then calls the **stub** `scanrun.Run` (the `module`/`no_http`/`provider`
  request fields are validated then **dropped**). Single-flight guard via 409 if running.
- **WebSocket `/api/scan/progress`** is a **fake**: 500ms ticker re-sending `CurrentProgressEvent`,
  which always reports `CompletedAccounts:0, TotalAccounts:0`. Not event-driven; no real progress.
- **No CORS, no auth.** `securityHeaders` sets nosniff/DENY/no-referrer + a CSP with `'unsafe-inline'`.
- Static SPA serving is correct (path-cleaned, immutable asset caching, index.html fallback).
- Smell: `strconvItoa` hand-rolls `strconv.Itoa`. `runScanAsync` uses `context.Background()` (ignores
  shutdown/cancel); stale goroutines from a superseded run keep running, no-op their state writes.

## B. API handlers & response schema (internal/api/handlers, schema)

- **All reads are filesystem-backed** via `scans.LoadScanArtifacts` (findings sorted by (ARN,ID) on
  load). Neo4j used only by blast-radius routes. No central response normalizer — stable-array
  behavior is per-handler `make(...)`, and **inconsistent** (query response + the blast finding-summary
  error path can still emit `null`).
- **Endpoint compute notes**: scan-list `TotalMonthlyCostUSD` = sum of **risk** cost only (name
  doesn't say so); summary `LowCount` is a **catch-all** bucket (info+unknown+empty all land in "low");
  external-entities aggregation is reused by the summary so counts match (tested invariant).
- **BUG (§B.5)**: blast-radius **finding** summary returns a **zero-value struct with HTTP 200** for a
  non-existent finding id (other blast roots route through a populated fallback) — inconsistent, and
  `top_resource_types` etc. come back `null` in that path.
- **Perf**: `ListScans` loads every scan's full findings just to count; `latest` resolution loads all
  scans again; blast handlers double-load artifacts (existence gate + service reload, TOCTOU gap).
- Dead-ish: `PrincipalBlastByID` decodes `pType` then `_ = pType`; `ExplorerFromEntity` focus depends
  on Go map iteration order.

## C. AWS collectors (internal/collectors, internal/aws)  — ORPHANED (see ★ above)

- Pattern: each `CollectX` fans out goroutine-per-account, semaphore =
  `RoleAssumptionConcurrency`, mutex accumulate, **first-error-wins (later errors dropped)**.
- Real AWS calls map cleanly: Route53 (dns), S3 (storage), CloudFront (edge), IAM roles+policies
  (trust), IAM GetRole/RoleLastUsed (activity, N+1), Organizations+STS assume (org).
- **Only live part of `internal/aws` is the SSO helper set** (`sso.go`); `session.go`
  (`AssumeAccount`) and all collectors have **no non-test callers**.
- `AssumeAccount` returns `error` but its body never returns non-nil (dead error path); caches creds
  before validating assumability. Org OU/tag lookups hammer a single shared Organizations client from
  N goroutines. `bucketFromOrigin` naive (breaks on dotted bucket names).

## D. Graph tier, embeddings & RAG (internal/graph, cmd neo4j)

- **Projection** (`writer.go`): 4 node labels (`AwsAccount`, `ScanSnapshot`, `Asset`, `Finding`);
  single `:Asset` label with `asset_type` discriminator; Properties/Evidence stored as JSON strings.
  Rel types: `OWNED_BY`/`CAPTURED`/`AFFECTS` + allowlisted `POINTS_TO`/`USES_CERT`/`FRONTS`/`TRUSTS`.
  Deterministic (sorted) statement emission. **Non-atomic**: one session+tx per statement, no rollback.
- **Embeddings**: only `openai` works (`text-embedding-3-small`, forced to 384 dims to fit an index
  sized for a *local* model that's a **hard-failing stub**). Doc comments claim "all-MiniLM-L6-v2" —
  aspirational, not reality. **Never invoked in production** (see ★).
- **RAG** (`rag.go`): retrieval-only; the `OWNED_BY` join is computed but its columns are sourced from
  the Finding node instead (dead join); missing-index detection is brittle error-string matching.
- **Cypher injection: low risk** — all values parameterized; only rel-type (closed allowlist) and the
  constant index name are interpolated. Good pattern.
- Two `exportScanToNeo4j` impls (see ★); `dbName` always `""`.

## E. Blast-radius & external-entity matching (internal/blastradius)

- **Algorithm**: JSON-first to resolve root → **Neo4j variable-length traversal** (undirected
  `*1..4` for blast, directed for attack-path, rel-type allowlisted) → in-memory working graph →
  DTO. Clean JSON-only fallback when no driver (`GraphAvailable:false` + stable reason).
- **External entity** = aggregate of `external_access` findings sharing
  `(external_principal, principal_type, external_account_id)`, id = base64(`\x1e`-joined). The `\x1e`
  separator constant is **duplicated 3×** across 2 packages.
- **BUG**: producer/consumer evidence type mismatch (see ★ cross-cutting); `ExplorerFromEntity` focus
  is non-deterministic (map iteration); `escalationPossibleFromSignals` returns true for **any** trust
  edge, making the more specific branches dead → escalation over-reported.
- Smells: `IsReachable` hardcoded `true` (meaningless field); several dead fields/`_ =` no-ops; two
  near-identical Cypher templates differing only by arrow direction. Perf: undirected `*1..4` over a
  dense IAM graph can enumerate many paths before LIMIT; O(N·E) per-node edge scans (bounded by 80/120
  caps).

## F. Alerting subsystem (internal/alerting)

- Clean layering: types → evaluators (4 rule types) → routing → cooldown → Slack → JSON store.
  Triggered **both** automatically post-scan (`scan_control.go:229`, fire-and-forget) **and** via the
  `/test` API (bypasses cooldown).
- Rule types: `scan_completion` (**always fires**, info), `new_critical` (needs baseline; diff by
  title|ARN), `reclaimable_threshold` (OR of count/risk thresholds), `stale_external_privileged`
  (ignores its advertised `RiskCostUSDMin`).
- **Cooldown** is per-`rule_id` only (no per-finding fingerprint); pure decision fn takes injected
  `now` (testable), but call sites read wall clock. Anchor lookup bounded by 500-event history → a
  busy deploy can evict the anchor and silently end suppression early.
- **SSRF** (see ★). Slack non-2xx error bodies discarded (only status). Each evaluation **mutates and
  rewrites the whole rule** → O(rules²) write amplification + lost-update risk. `previousScanID` does
  O(N) artifact loads per rule. JSON store writes non-atomic (no temp+rename). Webhooks plaintext.

## G. Query, persistence, config & demo (see ★ for demo/orphan findings)

- **Two query paths**: default **v2** (JSON composition + brittle substring intent classifier, no LLM,
  best-effort semantic retrieve silently swallowed) vs `--legacy-retrieval` (real vector RAG,
  retrieval-only). CLI help documents the **legacy** flags but defaults to v2 which ignores them.
- **v2 CLI leaks the Neo4j driver** (`driver,_,_ :=`, no `defer close`) and swallows connect errors;
  doesn't handle empty `latest` → confusing "invalid argument" when no scans exist.
- **Scan safety** (`internal/scans`): `IsSafeScanID` genuinely blocks `..`/`/` traversal (tested).
  **Asymmetry**: missing `scan-metadata.json` tolerated (synthetic fallback) but missing
  `findings.json` is a **hard error**.
- **Config**: stores env-var *names* not secrets (good); missing file non-fatal (returns Default);
  `local` embeddings explicitly documented as not implemented.
- **Demo** (`demo.go`, ~990 LOC): the only data producer; **fabricates 26 findings as literals**
  (8 edge + 18 external), risk costs from a severity lookup table, bypassing scorers entirely.
  `--dense` only adds trust-chain *relationships* (graph paths), not findings. `bundled_demo.go`
  embeds an 18-row JSON used **only** for scan id exactly `"demo"`; `buildDemoArtifacts` redundantly
  re-derives findings the same branch already handles.
- **Orphaned packages**: `internal/output` (Excel + safe CSV) and `internal/remediator` (stub command
  generator) imported by nothing; `report` cmd reimplements weaker writers inline (no CSV-injection
  protection).

## ★ Supporting-component audit (Neo4j / RAG / Alerting / Scan module)

| Component | Intent | Built to | Works today | The catch |
| --- | --- | --- | --- | --- |
| Neo4j graph | attack paths + visualize connectivity | High (projection + var-length traversal + 3D explorer + path variants) | ✅ on demo data via CLI export | dashboard export is a divergent copy shipping **empty asset/rel slices** → no edges to traverse; real data never arrives (stub) |
| Vector/RAG | embed scan data, query via RAG | High (embedder + vector index + hybrid retrieval) | ❌ dead end-to-end | `AttachFindingsEmbeddings` **never called**; `Finding.Embedding` is `json:"-"` → index always empty → zero results. **No LLM synthesis anywhere** (retrieval-only; default query = `fmt.Sprintf` + substring intent) |
| Alerting | alert people | High (rules + routing + cooldown + Slack) | ✅ mechanically sends Slack | evaluates the empty scan → only `scan_completion` reliably fires (noise); **SSRF** (webhook validated as `https://` only) |
| AWS scan module | initiate scan from dashboard | Medium (full UX + real cred/SSO check) | ❌ empty scans | 202 → real STS check → **stub** `scanrun.Run`; `module`/`no_http`/`provider` dropped; progress WS is a **fake** 500ms re-send (`CompletedAccounts:0`) |

**Two are broken independently of the data-starvation root cause:**
1. **RAG** — embeddings never attached on any export path → empty vector index even with real data;
   plus no generation step exists (legacy prints "Answer synthesis: not implemented").
2. **Dashboard Neo4j export** — divergent `exportScanToNeo4j` (scan_control.go) passes empty
   asset/rel slices → graph has Findings+Accounts+CAPTURED but no Assets/relationships/AFFECTS/OWNED_BY.

Closeness-to-working rank: Alerting > Neo4j-viz(demo) > Scan-module > RAG(non-functional by construction).

## ★ Per-issue acceptance audit (owner's required detections)

Universal gap (applies to ALL 7): `scan` is a stub → none fire against real AWS. Table below =
issue-SPECIFIC logic status beyond that. Demo (fabricated) displays all 7.

**Orphaned edge**
| # | Issue | Path → verdict | Logic | Issue-specific gap |
| --- | --- | --- | --- | --- |
| 1 | S3 website takeover | risk.go:41-47 → reclaimable/critical | ✅ | **`bucketNames` org-wide set never assembled** — the cross-account "not in your org" test can't run |
| 2 | Dangling CloudFront/CDN | risk.go:53-55 → dangling/high (http.go:153-166 fingerprints) | ✅ recognized bodies only | catches only 4 hardcoded AWS error-body signatures; generic 403/404 → not flagged |
| 3 | CDN hostname mismatch | risk.go:58-60 → edge_obscured/medium | ⚠️ broken | reads `in_alternate_domains` (never set; collector writes `alternate_domains`); no host-vs-alias-allowlist computation exists |
| 4 | Broken DNS (NXDOMAIN/timeout) | risk.go:63-65 → broken/low (http.go:93-103) | ✅ complete | none specific |

**External IAM trust**
| # | Issue | Path → verdict | Logic | Issue-specific gap |
| --- | --- | --- | --- | --- |
| 5 | Ghost / admin-equiv external trust | trust.go:165-169 → ghost_admin/critical; never-used → stale | ⚠️ split-brain | "ghost/never-used" works; **admin-equiv never escalates severity** (`is_admin` never set). Real admin analysis (`DeriveRolePermissionVisibility.admin_like`) IS computed → affects priority (+22) but NOT severity |
| 6 | Unapproved vendor account | trust.go:155-163 → unknown_vendor/high | ✅ AWS accts only | SAML/OIDC principals never get the allowlist check |
| 7 | Stale vs active trust | trust.go:174-182 activitySeverity (90/365) + perm heuristics | ✅ strongest logic in tool | none specific |

Summary: design-complete for all 7; fully correct for #4 & #7; per-issue defects for #1 (no
cross-account assembler), #3 (comparison not implemented), #5 (admin not bridged to severity);
scope limits for #2 (known bodies only) & #6 (AWS-acct only).

## ★ The intended two-part product model (owner-confirmed taxonomy)

| | **Part 1 — Orphaned / Reclaimable assets** | **Part 2 — External Trust** |
| --- | --- | --- |
| Goal | (a) cross-account takeover risk, (b) reclaim idle assets to save money | who outside the org has access, how long, last used when |
| Module | `orphaned_edge` (`risk.go` + `cost.go`) | `external_access` (`trust.go` + `activity.go`) |
| Issue types | #6 reclaimable (+dangling/edge_obscured/broken/unknown) | #1-5 ghost_admin / unknown_vendor / stale_review_now / aging / active |

### Part 1 sub-goal maturity (verified)
- **1a security takeover** — designed in `risk.go`, **unwired** (see thesis #1 below). 0 detected today.
- **1b cost / reclaim idle resources — barely built**:
  - **No idle/usage detection for any non-IAM resource.** The only `LastUsedAt`/`DaysSinceUsed`/
    `RoleLastUsed` signals are in `activity.go`, **IAM-roles only** (which belong to Part 2). No
    CloudWatch/access-log/idle detection for S3, CloudFront, DNS, EIP, EBS, etc.
  - **Cost is NOT usage-aware** (`cost.go`): static per-type estimate × **claimability** multiplier
    (reclaimable ×5, dangling ×3) — driven by *takeover risk*, not idleness. CE enrichment pulls
    actual *spend* (`UnblendedCost`), but spend ≠ idle.
  - ⇒ Part 1 has a cost-**labeling** layer (every orphaned finding gets a `$/mo` "Monthly Waste"
    figure), but **no idle-detection layer**: it can estimate the waste of a *broken* edge asset, not
    find a *healthy-but-unused* resource to reclaim.
- **"Reclaim" means two different things**: code's `ClaimReclaimable` = *attacker* can reclaim the
  dangling DNS name (security); owner's "reclaim to save money" = *org* can delete an unused resource
  (cost). The `claimability` enum encodes only takeover risk, nothing about idle-for-savings.

### Cross-account detection catalog (what the logic can classify)
- **6 issue types** over **3 external principal classes** (`aws_account`, `saml`, `oidc`; AWS service
  principals deliberately excluded). External-trust findings are cross-account by construction
  (`normalizeExternalAWSPrincipal` drops same-account principals, trust.go:344).
- **Identifiable right now from a real scan: 0** (stub + unwired). Demo *displays* 9 (`demo`) / 58
  (`demo-dense`) fabricated cross-account findings.
- Even if wired: `ghost_admin_access` (the critical one) is **dead** — needs `Properties["is_admin"]`
  the collector never sets; `unknown_vendor` only applies to `aws_account` principals.

## ★ Thesis vs. reality #1 — cross-account orphaned assets (S3 subdomain takeover)

**Intended product thesis:** linked, cross-account AWS services where deleting one component
creates a vuln; canonical case = dangling DNS → deleted S3 website bucket → reclaimable by any
AWS account globally → subdomain takeover.

**Design correctly models it** — `classifyRisk` (risk.go:40-50): RECLAIMABLE/critical fires on
`resolved + s3_bucket_deleted fingerprint + s3_website target + !bucketNames[bucket]`. The
`bucketNames` set is meant to be the **org-wide union** of bucket names across all scanned accounts;
`!bucketNames[bucket]` = "no account in the org owns this name" = reclaimable by an outsider.
Data model supports it (`AssetNode.AccountID` distinguishes the DNS-record account from the
bucket account).

**But the cross-account join is never built** (verified by grep):
- `ScoreRisk` has **only test callers** (`risk_test.go`).
- `bucketNames` is **only ever a test literal** — no production code unions buckets across accounts.
- **No file orchestrates collectors+validators+scorers** — `storage.go`/`http.go`/`risk.go` are
  three islands; nothing runs `collect-all-accounts → union bucket names → validate DNS → score`.
- Demo ships a fabricated reclaimable/critical finding so the UI *shows* the scenario, but it's
  hand-written JSON, not detector output.

**⇒ The flagship capability is the one thing the tool structurally cannot do today.**

Two correctness caveats for whoever wires it:
1. **Completeness = correctness**: reclaimable = "missing in *scanned* accounts"; incomplete org
   coverage → false-positive criticals on buckets owned in unscanned accounts. No scan-coverage
   signal exists today.
2. **Name correlation is naive**: `bucketNameFromTarget` (risk.go:70-80) splits DNS target on `.`
   and takes `[0]`; fragile for dotted/virtual-hosted names.

## H. React dashboard frontend (dashboard/src)

- Vite + React 18 + TS + TanStack Query + React Router v6 + react-three-fiber (3D blast explorer,
  lazy-loaded). `API_BASE="/api"`, same-origin, **no auth/CSRF**. Generally high quality (strict types,
  centralized query keys, error boundary, hardened `SafeExternalLink`, URL-state hooks, tests).
- `scan_id` flows via URL → `useScanContext` (defaults to newest scan). BlastExplorer inconsistently
  uses `scan`/`finding`/`entity`/`principal` params instead of `scan_id`.
- **Build-wiring bug** (see ★): tracked `dist/index.html` + gitignored/missing `dist/assets/`.
- Smells: `Query.tsx` is a prototype outlier (calls `apiClient` directly, raw Tailwind, `error as
  Error`); `ExternalEntities`/`Diff` hand-roll `useSearchParams` (ExternalEntities fires a query +
  history write **per keystroke**, no debounce); Findings hides `external_principal`/`external_account_id`
  filter chips; `Alerting` has metadata type-drift (two delivery-shape formats); `Accounts` light-mode
  contrast bug; no mobile sidebar toggle; no 404 route.
