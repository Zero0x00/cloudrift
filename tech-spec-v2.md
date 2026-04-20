# cloudrift — Technical Specification v2.0
# Final Architecture & Development Plan

Version: 2.0-final
Date: 2026-04-17
Status: Approved

---

## Implementation status (repository)

Checklist reflects **what exists in this tree today**, not the original phase plan alone.

### Done (implemented)

- [x] **CLI:** `scan`, `report`, `dashboard`, `query`, `version`, **`demo generate`** (`cmd/cloudrift`).
- [x] **Scan artifacts:** `output_dir/<scan_id>/` with `scan-metadata.json`, `findings.json`; optional `relationships.json`, `assets/*.json` for graph export.
- [x] **Dashboard + API:** Chi router; embedded React SPA; read-only scan APIs (`/api/scans`, summary, findings, accounts, diff).
- [x] **External access — API:** `GET /api/scans/{id}/external-entities` (paginated); summary includes entity rollups aligned with that aggregation (`internal/api/handlers/external_entities.go`).
- [x] **Findings filters:** Query params including `external_principal`, `external_account_id`, `principal_type`, `trust_classification`, `trust_stale`, `admin_like`, plus existing module/severity/search filters (`internal/api/handlers/findings.go`).
- [x] **Trust / permission visibility:** Conservative tiering from role policy artifacts (`internal/scorers/permission_visibility.go`); surfaced in finding evidence and dashboard trust UI.
- [x] **Phase 3 graph (optional):** Neo4j schema + `WriteScan` projection; `cloudrift scan --neo4j` and **`cloudrift demo generate --neo4j`**; vector index + retrieval path for `cloudrift query`.
- [x] **Scan Control Center (dashboard):** `/scan-control` UI; backend `GET /api/runtime/status`, `POST /api/runtime/validate-profile`, `POST /api/scan/start`, `GET /api/scan/status`, `GET /api/scan/history`, `GET /api/scan/progress` (WebSocket) — see `internal/api/handlers/scan_control.go`.
- [x] **Demo dataset:** `cloudrift demo generate` writes a deterministic bundle (findings + metadata + relationships + assets) for UI/graph exercises.
- [x] **Response-shape hardening:** list-like API fields now serialize as stable empty arrays (`[]`) instead of `null` in key envelopes (scan list account ids, summary external arrays, diff lists, runtime profiles, run history items).

### Partial / gap

- [ ] **CLI `scan`:** Still creates metadata + **empty** `findings.json` unless other paths populate the directory; full collectors→scorers→write pipeline is not the default `scan` path (library code in `internal/` is tested; orchestration gap called out in `docs/TECHNICAL.md`).
- [ ] **Embeddings `local` provider:** Config allowlist may mention it; **`Embed` is not implemented** (stub errors) — operational path is OpenAI with 384-dim for Neo4j.
- [ ] **RAG answer synthesis:** `cloudrift query` is **retrieval-only**; no LLM-composed answer in-tree.

### Roadmap (not implemented here)

- [ ] Wire default CLI `scan` to full pipeline (or document a single blessed entrypoint).
- [ ] Implement `local` / on-box embedding provider if required without OpenAI.
- [ ] Optional answer synthesis layer for `query` (behind explicit flag and provider config).
- [ ] Hardening: API auth when not bound to loopback; structured logging.

---

## 1. What We Are Building

**cloudrift** is an open-source, single-binary CLI tool that answers four
questions for every edge asset and external trust relationship in a
multi-account AWS Organization:

    What exists | Who owns it | Is it claimable | What is it costing

**Module A — Orphaned Edge Assets**
Discovers dangling/orphaned DNS records, S3 website endpoints, CloudFront
distributions, API Gateway custom domains, and ACM certificates across
multi-account AWS Organizations. Scores each asset for subdomain takeover
claimability and estimates monthly cloud spend waste.

**Module B — External Access Exposure** (Phase 2)
Identifies external entities (vendors, third parties) with cross-account IAM
role access. Detects stale trust relationships using CloudTrail last-used data.
Produces outputs for quarterly access reviews under SOC 2 / ISO 27001.

---

## 2. Final Technology Stack

No alternatives. These are the decisions.

| Component            | Choice                        | Reason                                               |
|----------------------|-------------------------------|------------------------------------------------------|
| Language             | Go 1.22+                      | Single binary, goroutine concurrency, security tool ecosystem |
| CLI framework        | Cobra                         | Standard for Go CLI tools; CloudFox baseline uses it |
| AWS SDK              | aws-sdk-go-v2                 | Official, modular, per-service packages, pagination helpers |
| DNS                  | github.com/miekg/dns          | Full resolver control, CNAME chain walking           |
| HTTP probing         | net/http + goroutines         | No extra dependency; semaphore-controlled pool       |
| API server           | Chi                           | Lightweight, 100% stdlib-compatible, readable        |
| WebSocket            | nhooyr.io/websocket           | Modern Go WS, stdlib-compatible                      |
| Frontend framework   | React 18 + Vite               | Component model, fast build, large contributor pool  |
| Styling              | TailwindCSS                   | Utility-first, no CSS file maintenance               |
| Dashboard components | Tremor v3                     | Built for analytics dashboards, pre-built charts     |
| Data tables          | TanStack Table                | Sortable, filterable, virtualized findings table     |
| API client (FE)      | TanStack Query                | Caching, loading states, auto-refetch                |
| Routing (FE)         | React Router v6               | Standard SPA routing                                 |
| Frontend embed       | //go:embed dashboard/dist     | React build baked into binary — zero frontend setup  |
| Config               | BurntSushi/toml               | Standard Go TOML parser                              |
| Excel output         | xuri/excelize/v2              | Go Excel library, three-sheet workbook               |
| Build / release      | GoReleaser                    | Multi-arch binaries, Homebrew tap, GitHub Release    |
| Linting              | golangci-lint                 | Single tool, covers everything                       |
| Testing              | testing + aws-sdk-go-v2 mocks | Built-in framework, SDK mock clients                 |
| Graph DB (Phase 3)   | Neo4j 5+                      | Native HNSW vectors, Cypher, GraphRAG support        |
| Embeddings (Phase 3) | OpenAI text-embedding-3-small   | Default: API `dimensions=384` for Neo4j index; local MiniLM reserved (not bundled) |
| LLM (Phase 3)        | Claude API (claude-sonnet-4-6)| Strong structured data reasoning                     |

---

## 3. Repository Structure

```
cloudrift/
├── cmd/
│   └── cloudrift/
│       └── main.go              # entry point, Cobra root command
│
├── internal/
│   ├── aws/
│   │   └── session.go           # STS AssumeRole, in-memory session cache
│   │
│   ├── models/
│   │   ├── asset.go             # AssetNode struct
│   │   ├── relationship.go      # Relationship struct
│   │   ├── finding.go           # Finding struct
│   │   └── snapshot.go          # ScanSnapshot struct
│   │
│   ├── collectors/
│   │   ├── org.go               # Organizations account enumeration
│   │   ├── dns.go               # Route 53 zones + records
│   │   ├── edge.go              # CloudFront distributions
│   │   ├── storage.go           # S3 buckets + website hosting
│   │   ├── certs.go             # ACM certificates          [Phase 2]
│   │   ├── apigw.go             # API Gateway custom domains [Phase 2]
│   │   ├── trust.go             # IAM cross-account trusts  [Phase 2]
│   │   └── activity.go          # IAM last-used / CloudTrail [Phase 2]
│   │
│   ├── validators/
│   │   └── http.go              # DNS resolve, HTTP HEAD, TLS, fingerprinting
│   │
│   ├── scorers/
│   │   ├── risk.go              # Claimability classification
│   │   ├── cost.go              # Direct + risk-adjusted cost
│   │   └── trust.go             # Trust staleness scoring   [Phase 2]
│   │
│   ├── output/
│   │   ├── table.go             # Rich terminal table (tablewriter)
│   │   ├── json.go              # JSON writer
│   │   ├── csv.go               # CSV writer
│   │   ├── excel.go             # Excel workbook (excelize) [Phase 2]
│   │   └── markdown.go          # Markdown ticket bodies
│   │
│   ├── remediator/
│   │   └── generator.go         # Per-finding AWS CLI snippets
│   │
│   ├── api/                     # Dashboard backend         [Phase 2]
│   │   ├── server.go            # Chi router, mounts /api + static files
│   │   ├── handlers/
│   │   │   ├── scans.go
│   │   │   ├── findings.go
│   │   │   └── ws.go            # WebSocket scan progress stream
│   │   └── schema/
│   │       └── responses.go     # Response structs
│   │
│   └── graph/                   # Neo4j + RAG               [Phase 3]
│       ├── schema.go            # Cypher CREATE CONSTRAINT / INDEX
│       ├── writer.go            # Write nodes + relationships
│       ├── embedder.go          # Text embedding on Finding nodes
│       └── rag.go               # Hybrid retrieval + Claude API
│
├── dashboard/                   # React app
│   ├── package.json
│   ├── vite.config.ts
│   ├── tailwind.config.ts
│   ├── src/
│   │   ├── main.tsx
│   │   ├── pages/
│   │   │   ├── Overview.tsx
│   │   │   ├── Findings.tsx
│   │   │   ├── Accounts.tsx
│   │   │   ├── Diff.tsx
│   │   │   └── TrustReport.tsx
│   │   ├── components/
│   │   └── api/                 # TanStack Query hooks
│   └── dist/                    # go:embed target (gitignored)
│
├── iam/
│   ├── auditing-role-policy.json
│   └── stackset-template.yaml   # CloudFormation StackSet — deploys to all accounts
│
├── docs/
│   ├── architecture.md
│   ├── iam-setup.md
│   └── getting-started.md
│
├── .github/
│   └── workflows/
│       ├── ci.yml               # lint + test on every PR
│       └── release.yml          # GoReleaser on git tag
│
├── .goreleaser.yaml
├── go.mod
├── go.sum
└── README.md
```

---

## 4. Data Models

### 4.1 AssetNode

```go
// internal/models/asset.go

type AssetType string

const (
    AssetDNSRecord        AssetType = "dns_record"
    AssetS3Bucket         AssetType = "s3_bucket"
    AssetCloudFrontDist   AssetType = "cloudfront_dist"
    AssetAPIGatewayDomain AssetType = "apigateway_domain"
    AssetACMCert          AssetType = "acm_cert"
    AssetIAMRole          AssetType = "iam_role"
    AssetExternalPrincipal AssetType = "external_principal"
)

type AssetNode struct {
    ARN        string            `json:"arn"`
    AssetType  AssetType         `json:"asset_type"`
    Name       string            `json:"name"`
    AccountID  string            `json:"account_id"`
    Region     string            `json:"region"`
    Properties map[string]any    `json:"properties"`
    ScanID     string            `json:"scan_id"`
}
```

Key `Properties` per AssetType:

| AssetType           | Required properties                                                      |
|---------------------|--------------------------------------------------------------------------|
| dns_record          | type, value, zone_id, private, target_service, dns_status               |
| s3_bucket           | website_enabled, website_endpoint, public_access_blocked, bucket_region |
| cloudfront_dist     | domain, enabled, origin, alternate_domains, price_class                  |
| apigateway_domain   | cert_arn, endpoint_type, stage                                           |
| acm_cert            | domain, sans, expiry, exportable, status, in_use_by                      |
| iam_role            | trust_policy, last_used, days_since_used, is_admin                       |
| external_principal  | principal_type, external_account_id                                      |

### 4.2 Relationship

```go
// internal/models/relationship.go

type RelType string

const (
    RelPointsTo   RelType = "POINTS_TO"    // DnsRecord → S3|CloudFront|ApiGW
    RelOwnedBy    RelType = "OWNED_BY"     // any asset → AwsAccount
    RelUsesCert   RelType = "USES_CERT"   // CloudFront → AcmCert
    RelFronts     RelType = "FRONTS"       // CloudFront → S3Bucket
    RelTrusts     RelType = "TRUSTS"       // IamRole → ExternalPrincipal
)

type Relationship struct {
    SourceARN  string         `json:"source_arn"`
    TargetARN  string         `json:"target_arn"`
    RelType    RelType        `json:"rel_type"`
    Properties map[string]any `json:"properties"`
    ScanID     string         `json:"scan_id"`
}
```

### 4.3 Finding

```go
// internal/models/finding.go

type Severity    string
type Claimability string
type Module      string

const (
    SeverityCritical Severity = "critical"
    SeverityHigh     Severity = "high"
    SeverityMedium   Severity = "medium"
    SeverityLow      Severity = "low"
    SeverityInfo     Severity = "info"

    ClaimReclaimable  Claimability = "reclaimable"
    ClaimDangling     Claimability = "dangling"
    ClaimBroken       Claimability = "broken"
    ClaimEdgeObscured Claimability = "edge_obscured"
    ClaimUnknown      Claimability = "unknown"

    ModuleOrphanedEdge   Module = "orphaned_edge"
    ModuleExternalAccess Module = "external_access"
)

type Finding struct {
    ID                 string        `json:"id"`                  // sha256(arn+title)[:12]
    Title              string        `json:"title"`
    Severity           Severity      `json:"severity"`
    Module             Module        `json:"module"`
    Claimability       Claimability  `json:"claimability"`
    AffectedARN        string        `json:"affected_arn"`
    AccountID          string        `json:"account_id"`
    AccountName        string        `json:"account_name"`
    OUPath             string        `json:"ou_path"`
    Team               string        `json:"team"`
    Hostname           string        `json:"hostname"`
    MonthlyDirectCost  float64       `json:"monthly_direct_cost_usd"`
    MonthlyRiskCost    float64       `json:"monthly_risk_cost_usd"`
    Impact             string        `json:"impact"`
    Recommendation     string        `json:"recommendation"`
    RemediationCmd     string        `json:"remediation_command"`
    Evidence           map[string]any `json:"evidence"`
    ScanID             string        `json:"scan_id"`
    Embedding          []float32     `json:"-"`                   // Phase 3, never serialized
}
```

### 4.4 ScanSnapshot

```go
// internal/models/snapshot.go

type ScanSnapshot struct {
    ScanID             string    `json:"scan_id"`           // uuid4
    Timestamp          time.Time `json:"timestamp"`
    AccountIDs         []string  `json:"account_ids"`
    ToolVersion        string    `json:"tool_version"`
    FindingCount       int       `json:"finding_count"`
    CriticalCount      int       `json:"critical_count"`
    HighCount          int       `json:"high_count"`
    TotalMonthlyCost   float64   `json:"total_monthly_cost_usd"`
}
```

---

## 5. Module Specifications

### 5.1 collector/org.go

**Purpose:** Enumerate all accounts in the AWS Organization and return an
assumed session per account.

```go
type Account struct {
    ID      string
    Name    string
    OUPath  string
    Team    string
    Contact string
    Session *aws.Config
}

func CollectAccounts(ctx context.Context, cfg *config.Config) ([]Account, error)
```

**Logic:**
1. `organizations.ListAccounts()` — paginated
2. For each account: `sts.AssumeRole(RoleArn = arn:aws:iam::{id}:role/{cfg.OrgRoleName})`
3. Cache sessions in-memory for scan duration
4. Walk `organizations.ListParents()` recursively to build OU path
5. Pull tags: `Team`, `Owner`, `Contact` from account tags
6. Goroutine pool, semaphore cap 10

---

### 5.2 collector/dns.go

**Purpose:** Collect all Route 53 resource records across all accounts.

```go
type DNSRecord struct {
    Name          string
    Type          string
    Value         string
    ZoneID        string
    ZoneName      string
    Private       bool
    TargetService string    // cloudfront | s3_website | apigateway | alb | third_party
}

func CollectDNS(ctx context.Context, accounts []Account) ([]AssetNode, error)
```

**Target service classification (from CNAME/Alias value):**

| Pattern                                   | TargetService  |
|-------------------------------------------|----------------|
| `*.s3-website-*.amazonaws.com`            | s3_website     |
| `*.cloudfront.net`                        | cloudfront     |
| `*.execute-api.*.amazonaws.com`           | apigateway     |
| `*.elb.amazonaws.com`                     | alb            |
| `*.elasticbeanstalk.com`                  | elasticbeanstalk |
| anything else with `.` (external)         | third_party    |

Skip: SOA, NS records. Flag private zone records but don't score for takeover.

---

### 5.3 collector/storage.go

**Purpose:** Collect S3 buckets with website hosting state.

```go
func CollectStorage(ctx context.Context, accounts []Account) ([]AssetNode, error)
```

**Per bucket:**
- `s3.GetBucketLocation()` → actual region
- `s3.GetBucketWebsite()` → website_enabled + endpoint; skip on `NoSuchWebsiteConfiguration`
- `s3.GetPublicAccessBlock()` → all four BlockPublicXxx flags
- `s3.GetBucketTagging()` → team/owner tags

**Website endpoint formula:**
```
{bucket}.s3-website-{region}.amazonaws.com         (older regions)
{bucket}.s3-website.{region}.amazonaws.com         (newer regions)
```

---

### 5.4 collector/edge.go

**Purpose:** Collect CloudFront distributions, alternate domains, origins.

```go
func CollectEdge(ctx context.Context, accounts []Account) ([]AssetNode, []Relationship, error)
```

**Per distribution:**
- `AlternateDomainNames` → CNAMEs that point to this distribution
- Origin: S3 bucket ARN or custom origin hostname
- `ViewerCertificate.ACMCertificateArn`
- `Enabled` status and `PriceClass`
- Emit `USES_CERT` if cert ARN present
- Emit `FRONTS` if origin is an S3 bucket

**Key insight:** A distribution can be orphaned while `Enabled: true` if its
origin bucket is gone and no DNS record points to it.

---

### 5.5 validator/http.go

**Purpose:** Live DNS + HTTP + TLS validation for each DNS record.

```go
type ValidationResult struct {
    DNSStatus        string     // resolved | nxdomain | timeout | servfail
    HTTPStatus       int
    TLSValid         bool
    CDNDetected      bool
    CDNVendor        string     // cloudfront | akamai | fastly | unknown
    ErrorFingerprint string     // NoSuchBucket | CloudFrontError | ...
    BodySnippet      string     // first 512 bytes
}

func ValidateAssets(ctx context.Context, nodes []AssetNode, concurrency int) map[string]ValidationResult
```

**Concurrency:** goroutine pool with semaphore cap (default 50).
**Probing order:** DNS resolve → HTTP HEAD → fallback GET (512 bytes only) → TLS check.
**Rate limiting:** `--no-http` flag skips all probes (DNS-only mode).

**Known error fingerprints:**

| Pattern in body / headers                    | Fingerprint          |
|----------------------------------------------|----------------------|
| `<Code>NoSuchBucket</Code>`                  | s3_bucket_deleted    |
| `403` + `S3` server header                   | s3_bucket_exists_private |
| CloudFront `The request could not be satisfied` | cloudfront_origin_error |
| `<Code>InvalidClientTokenId</Code>`          | aws_endpoint_controlled |
| NXDOMAIN                                     | dns_nxdomain         |

---

### 5.6 scorer/risk.go

**Purpose:** Classify each DNS record + asset pair with a claimability verdict.

```go
func ScoreRisk(node AssetNode, validation ValidationResult, bucketNames map[string]bool) Finding
```

**Classification logic:**

```
RECLAIMABLE (critical):
  dns_status = resolved
  AND http error_fingerprint = s3_bucket_deleted
  AND bucket name NOT in any scanned account's bucket list
  → Attacker can create bucket in any AWS account and claim the hostname

  OR: CloudFront distribution deleted AND its CNAME still resolves to *.cloudfront.net

DANGLING (high):
  dns_status = resolved
  AND AWS-controlled endpoint (HTTP 4xx/5xx with AWS error body)
  AND not immediately reclaimable

EDGE_OBSCURED (medium):
  Hostname resolves to a CloudFront IP
  AND hostname NOT in distribution's AlternateDomainNames list
  → CDN may drop the hostname; attacker creates origin after

BROKEN (low):
  dns_status = nxdomain OR timeout
  AND no AWS-recognizable error
  → Record points nowhere; no immediate takeover risk
```

**Critical implementation note:** The reclaimable verdict MUST cross-reference
the full set of bucket names collected across ALL scanned accounts. A bucket
is only reclaimable if its exact name does not exist in any scanned account.
False positives here destroy credibility.

---

### 5.7 scorer/cost.go

**Purpose:** Estimate monthly billing waste per resource.

```go
func ScoreCost(node AssetNode, finding *Finding) (directCost, riskCost float64)
```

**Pricing rules (AWS list price):**

| Resource            | Direct cost formula                                     |
|---------------------|---------------------------------------------------------|
| Route 53 hosted zone| $0.50/month (first 25), $0.10/month thereafter          |
| Route 53 queries    | $0.40/million standard, $0.60/million latency-based     |
| S3 bucket           | $0.023/GB-month (Standard) + $0.0004/1k GETs            |
| CloudFront dist     | $0 free tier → $35+/month paid plan                     |
| ACM exportable cert | $7/FQDN or $79/wildcard (issuance + renewal)            |

**Risk multipliers:**
```
risk_cost = direct_cost * multiplier
  reclaimable → 5x
  dangling    → 3x
  broken      → 1x
```

Phase 2: Pull actual spend from `ce.GetCostAndUsage()` when `cost.use_cur = true`.

---

### 5.8 collector/trust.go (Phase 2)

**Purpose:** Find IAM roles with external trust relationships.

**External principal types:**
- `AWS: arn:aws:iam::<external_account_id>:*` → aws_account
- `Federated: arn:aws:iam::*:saml-provider/*` → saml
- `Federated: accounts.google.com` → oidc
- `Service: *.amazonaws.com` (internal) → skip

---

### 5.9 collector/activity.go (Phase 2)

**Purpose:** Get last-used timestamps for IAM roles.

- `iam.GetRole()` → `RoleLastUsed.LastUsedDate` (free, no CloudTrail)
- Phase 2 enhancement: `cloudtrail.LookupEvents(AssumeRole)` for per-principal activity
- `days_since_used = today - last_used_date`
- Never used: `days_since_used = -1`

---

### 5.10 scorer/trust.go (Phase 2)

| Condition                                   | Severity | Verdict              |
|---------------------------------------------|----------|----------------------|
| Never used OR days > 365                    | high     | stale — review now   |
| days 90–365                                 | medium   | aging                |
| days < 90                                   | low      | active               |
| is_admin = true AND external trust          | critical | ghost admin access   |
| external account not in approved list       | high     | unknown vendor       |

---

### 5.11 remediator/generator.go

Generates per-finding output:
1. `RemediationCmd` — one-line AWS CLI command, stored on Finding
2. Markdown ticket body — title, evidence, impact, steps, owner contact
3. JSON payload — Jira/ServiceNow-ready

---

## 6. CLI Design

```
cloudrift [flags] <command>

Commands:
  scan        Run collectors + validators + scorers
  report      Generate output from scan results
  diff        Compare two scan snapshots
  remediate   Show remediation for a finding
  dashboard   Start web dashboard server        [Phase 2]
  query       Natural language query            [Phase 3]
  version     Print version info

scan flags:
  --profile TEXT          AWS CLI profile
  --role-arn TEXT         Entry-point role ARN
  --org-role TEXT         Role name in each member account [default: CloudriftAuditRole]
  --accounts TEXT         Comma-separated account IDs (skip org enumeration)
  --module TEXT           orphaned-edge | external-access | all [default: orphaned-edge]
  --output-dir PATH       [default: ./cloudrift-output]
  --no-http               Skip HTTP probing (DNS-only mode)
  --concurrency INT       HTTP probe concurrency [default: 50]
  --neo4j                 Write to Neo4j (Phase 3)

report flags:
  --scan-id TEXT          Target scan [default: latest]
  --format TEXT           table | json | csv | excel | markdown [default: table]
  --severity TEXT         Minimum severity filter
  --module TEXT           orphaned-edge | external-access | all
  --output PATH

diff flags:
  --old TEXT              scan-id of baseline
  --new TEXT              scan-id of current [default: latest]
  --format TEXT           table | json [default: table]

remediate flags:
  --finding-id TEXT
  --format TEXT           cli | markdown | json [default: cli]

dashboard flags:
  --port INT              [default: 8000]
  --open                  Auto-open browser [default: true]
  --scan-id TEXT          Scan to display [default: latest]
```

---

## 7. Configuration File

Precedence: `$CLOUDRIFT_CONFIG` env var → `./cloudrift.toml` → `~/.config/cloudrift/config.toml`

```toml
[aws]
org_role_name = "CloudriftAuditRole"
management_profile = "default"
regions = []                               # empty = all regions

[scan]
http_probe_concurrency = 50
role_assumption_concurrency = 10
http_timeout_seconds = 10
user_agent = "cloudrift/0.1"

[cost]
currency = "USD"
risk_multiplier_reclaimable = 5.0
risk_multiplier_dangling = 3.0
use_cur = false                            # Phase 2: enable CUR enrichment

[trust]
approved_external_accounts = []            # known-good vendor accounts
stale_threshold_days = 90
ghost_threshold_days = 365

[output]
default_format = "table"
output_dir = "./cloudrift-output"

[neo4j]                                    # Phase 3
uri = "bolt://localhost:7687"
username = "neo4j"
password_env = "CLOUDRIFT_NEO4J_PASSWORD"

# --- Embeddings (Phase 3) -----------------------------------------------
# DEFAULT (no file / empty override): provider is "openai" — see config.Default() in code.
# Only OpenAI is operational today (text-embedding-3-small, API dimensions=384 for Neo4j).
# "local" is PLANNED ONLY (MiniLM/ONNX): not supported; Embed always errors until implemented.
[embeddings]
provider = "openai"                        # MUST stay explicit: openai = operational; local = planned stub only
local_model = "all-MiniLM-L6-v2"           # reserved for future local provider (not wired)
openai_api_key_env = "OPENAI_API_KEY"      # required for OpenAI path; never commit keys
```

---

## 8. IAM Setup

### Auditing Role Policy (deploys to every member account)

File: `iam/auditing-role-policy.json`

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "OrganizationsRead",
      "Effect": "Allow",
      "Action": [
        "organizations:ListAccounts",
        "organizations:ListAccountsForParent",
        "organizations:ListOrganizationalUnitsForParent",
        "organizations:ListParents",
        "organizations:ListTagsForResource",
        "organizations:DescribeOrganization",
        "organizations:DescribeAccount"
      ],
      "Resource": "*"
    },
    {
      "Sid": "Route53Read",
      "Effect": "Allow",
      "Action": [
        "route53:ListHostedZones",
        "route53:ListResourceRecordSets",
        "route53:GetHostedZone",
        "route53:ListTagsForResource"
      ],
      "Resource": "*"
    },
    {
      "Sid": "CloudFrontRead",
      "Effect": "Allow",
      "Action": [
        "cloudfront:ListDistributions",
        "cloudfront:GetDistribution",
        "cloudfront:GetDistributionConfig",
        "cloudfront:ListTagsForResource"
      ],
      "Resource": "*"
    },
    {
      "Sid": "S3Read",
      "Effect": "Allow",
      "Action": [
        "s3:ListAllMyBuckets",
        "s3:GetBucketLocation",
        "s3:GetBucketWebsite",
        "s3:GetBucketPolicy",
        "s3:GetBucketPolicyStatus",
        "s3:GetPublicAccessBlock",
        "s3:GetBucketTagging",
        "s3:GetBucketAcl"
      ],
      "Resource": "*"
    },
    {
      "Sid": "ACMRead",
      "Effect": "Allow",
      "Action": [
        "acm:ListCertificates",
        "acm:DescribeCertificate",
        "acm:ListTagsForCertificate"
      ],
      "Resource": "*"
    },
    {
      "Sid": "APIGatewayRead",
      "Effect": "Allow",
      "Action": ["apigateway:GET"],
      "Resource": "arn:aws:apigateway:*::/domainnames"
    },
    {
      "Sid": "IAMRead",
      "Effect": "Allow",
      "Action": [
        "iam:ListRoles",
        "iam:GetRole",
        "iam:ListAttachedRolePolicies",
        "iam:ListRolePolicies",
        "iam:GetRolePolicy",
        "iam:ListRoleTags"
      ],
      "Resource": "*"
    },
    {
      "Sid": "CloudTrailRead",
      "Effect": "Allow",
      "Action": ["cloudtrail:LookupEvents"],
      "Resource": "*"
    },
    {
      "Sid": "CostRead",
      "Effect": "Allow",
      "Action": ["ce:GetCostAndUsage"],
      "Resource": "*"
    },
    {
      "Sid": "STSAssumeAuditRole",
      "Effect": "Allow",
      "Action": "sts:AssumeRole",
      "Resource": "arn:aws:iam::*:role/CloudriftAuditRole"
    }
  ]
}
```

### Trust Policy (on each member account role)

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": {
      "AWS": "arn:aws:iam::<MANAGEMENT_ACCOUNT_ID>:role/CloudriftExecutionRole"
    },
    "Action": "sts:AssumeRole",
    "Condition": {
      "StringEquals": { "sts:ExternalId": "<RANDOM_EXTERNAL_ID>" }
    }
  }]
}
```

### Deployment

A CloudFormation StackSet (`iam/stackset-template.yaml`) deploys
`CloudriftAuditRole` to all member accounts from the management account.
Users run:
```bash
aws cloudformation create-stack-set \
  --stack-set-name cloudrift-audit \
  --template-body file://iam/stackset-template.yaml \
  --parameters ParameterKey=ManagementAccountId,ParameterValue=<id> \
               ParameterKey=ExternalId,ParameterValue=<random>
```

---

## 9. Output

### JSON Directory Structure

```
cloudrift-output/
  <scan-id>/
    scan-metadata.json
    assets/
      dns-records.json
      s3-buckets.json
      cloudfront-dists.json
      acm-certs.json         [Phase 2]
      iam-roles.json         [Phase 2]
    relationships.json
    findings.json
    summary.json
```

### Terminal Table

```
cloudrift v0.1.0  |  scan-id: 2026-04-17-abc123  |  accounts: 12  |  duration: 47s

┌──────────────────────┬──────────────┬──────────────┬───────────────┬──────────────┐
│ Hostname             │ Account      │ Service      │ Verdict       │ Monthly Waste│
├──────────────────────┼──────────────┼──────────────┼───────────────┼──────────────┤
│ app.example.com      │ prod-web     │ S3 Website   │ RECLAIMABLE ● │ $0.50        │
│ cdn.example.com      │ prod-cdn     │ CloudFront   │ DANGLING ●    │ $35.00       │
│ api.example.com      │ prod-api     │ API Gateway  │ BROKEN ●      │ $0.00        │
└──────────────────────┴──────────────┴──────────────┴───────────────┴──────────────┘
Summary: 3 findings  |  1 critical  |  1 high  |  $35.50/month waste
Run `cloudrift remediate --finding-id <id>` to see cleanup commands.
```

### Excel (Phase 2) — Three sheets

- **Findings** — color-coded by severity (red=critical, orange=high, yellow=medium)
- **Cost Summary** — waste per service per account
- **Trust Report** — external access findings with last-used dates

---

## 10. Dashboard (Phase 2)

### Backend embed

```go
//go:embed dashboard/dist
var dashboardFS embed.FS

func StartServer(port int, outputDir string) error {
    r := chi.NewRouter()
    r.Mount("/api", apiRouter(outputDir))
    r.Handle("/*", http.FileServer(http.FS(dashboardFS)))
    return http.ListenAndServe(fmt.Sprintf(":%d", port), r)
}
```

### API Endpoints

```
GET  /api/scans                          list all scan snapshots
GET  /api/scans/:id/summary              KPI counts + cost totals
GET  /api/scans/:id/findings             paginated, filterable findings
GET  /api/scans/:id/findings/:fid        single finding detail
GET  /api/scans/:id/accounts             per-account breakdown
GET  /api/diff?old=:id&new=:id           new + resolved findings
WS   /api/scan/progress                  live scan progress stream
```

### Dashboard Pages

**Overview** — KPI cards + charts
```
[Critical: N]  [High: N]  [Reclaimable: N]  [Monthly Waste: $X]

Findings by Severity (donut) | Waste by Service (bar)
Findings by Account (bar)    | Claimability Breakdown (donut)
```

**Findings** — TanStack Table, sortable + filterable
Columns: Severity | Hostname | Account | Team | Service | Verdict | Cost
Expandable row: evidence JSON, remediation command, ticket markdown

**Accounts** — Per-account card grid
Each card: finding count, waste, OU path, team, top finding

**Diff** — Scan comparison
New findings (red) | Resolved (green) | Unchanged (grey)

**Trust Report** (Phase 2)
External principals table: account ID, role, last used, days stale, verdict

---

## 11. Graph + RAG (Phase 3)

### Neo4j Schema

```cypher
CREATE CONSTRAINT account_id IF NOT EXISTS
  FOR (a:AwsAccount) REQUIRE a.account_id IS UNIQUE;

CREATE CONSTRAINT finding_id IF NOT EXISTS
  FOR (f:Finding) REQUIRE f.id IS UNIQUE;

CREATE VECTOR INDEX finding_embeddings IF NOT EXISTS
  FOR (f:Finding) ON (f.embedding)
  OPTIONS {indexConfig: {
    `vector.dimensions`: 384,
    `vector.similarity_function`: 'cosine'
  }};
```

### Hybrid RAG Query

```cypher
CALL db.index.vector.queryNodes('finding_embeddings', 5, $query_vector)
YIELD node AS f, score
MATCH (f)-[:AFFECTS]->(asset)
MATCH (asset)-[:OWNED_BY]->(account:AwsAccount)
OPTIONAL MATCH (asset)-[:POINTS_TO]->(target)
RETURN f.title, f.severity, f.claimability, f.monthly_direct_cost_usd,
       f.recommendation, account.name, account.ou_path, account.team, score
ORDER BY score DESC
```

### Temporal Diff (Cypher)

```cypher
MATCH (s2:ScanSnapshot {scan_id: $current})-[:CAPTURED]->(f:Finding)
WHERE NOT EXISTS {
  MATCH (s1:ScanSnapshot {scan_id: $previous})-[:CAPTURED]->(f2:Finding)
  WHERE f2.title = f.title AND f2.affected_arn = f.affected_arn
}
RETURN f AS new_findings
```

---

## 12. Testing Strategy

```go
// All collectors: aws-sdk-go-v2 mock client
func TestCollectDNS_FindsDanglingCNAME(t *testing.T) {
    mockR53 := mockRoute53Client(t, withZone("example.com"), withRecord("app", "CNAME", "deleted.s3-website-us-east-1.amazonaws.com"))
    nodes, err := collectDNS(context.Background(), mockSession(mockR53))
    require.NoError(t, err)
    assert.Equal(t, "s3_website", nodes[0].Properties["target_service"])
}

// All scorers: pure functions, no mocking
func TestScoreRisk_DeletedS3IsReclaimable(t *testing.T) {
    node := assetNode(AssetDNSRecord, map[string]any{"target_service": "s3_website"})
    result := ValidationResult{DNSStatus: "resolved", ErrorFingerprint: "s3_bucket_deleted"}
    buckets := map[string]bool{}   // bucket not found in any account
    finding := ScoreRisk(node, result, buckets)
    assert.Equal(t, ClaimReclaimable, finding.Claimability)
    assert.Equal(t, SeverityCritical, finding.Severity)
}

// Validator: httptest.Server for HTTP mocks
// Integration tests: tagged @integration, skipped unless CLOUDRIFT_INTEGRATION=1
```

---

## 13. Critical Risks

**Risk 1: Reclaimable false positives destroy credibility.**
The bucket-name cross-reference check across all scanned accounts is
non-negotiable. A bucket named in a dangling CNAME might exist in a different
account that wasn't scanned. Document the limitation clearly:
"reclaimable verdict is only valid within the scope of scanned accounts."

**Risk 2: IAM role deployment is the #1 adoption blocker.**
The StackSet template is not optional. Every user who has to manually deploy
to 47 accounts will give up. Ship the StackSet in v0.1.0, reference it on the
first line of getting-started.md.

**Risk 3: HTTP probing triggers WAF / IDS alerts.**
Document this clearly. Default user-agent is identifiable (`cloudrift/0.1`).
Provide `--no-http` for environments where probing is not allowed.
Use HEAD-only probes; GET only when fingerprinting requires body content.

**Risk 4: Multi-region S3 endpoint format variation.**
AWS added the `{bucket}.s3-website.{region}.amazonaws.com` format for newer
regions alongside the older `{bucket}.s3-website-{region}.amazonaws.com`.
The validator must check both patterns or misses records in newer regions.

**Risk 5: Phase 3 scope creep.**
Neo4j and RAG are a research project layered on a finished product. They must
never influence Phase 1 or Phase 2 architecture decisions. The JSON flat file
storage model has no Neo4j dependency and this must stay true through v0.2.0.

---

## 15. Improvised Scan Control Center (Safe Runtime-Orchestrated Scans)

### Why this exists

Operators need to launch scans from the dashboard while maintaining strict secret hygiene:

- UI can choose profile/module/provider flags.
- Backend resolves credentials from AWS shared config, environment, role-based providers, and configured secret env vars.
- UI and persisted app config **must never carry raw AWS/OpenAI secret values**.

### Safe control APIs (no secret payloads)

- `GET /api/runtime/status`
  - Returns safe runtime state only:
    - available AWS profile names
    - default profile
    - booleans for OpenAI, Neo4j, Slack alerting, email alerting configuration
  - Explicitly excludes raw secret values.
  - Note: empty profile discovery does **not** imply auth is impossible; ambient AWS credential sources
    (instance role, task role, env chain) may still be valid.

- `POST /api/runtime/validate-profile`
  - Input: `{ "profile": "name" }`
  - Backend-only profile resolution and caller-identity validation.
  - Returns safe operator message (`ok`, `profile`, `message`) without leaking internal auth details.

- `POST /api/scan/start`
  - Input:
    - `profile`
    - `module` (`all|orphaned_edge|external_access`)
    - `no_http`
    - `neo4j`
    - optional `provider` (bounded allowlist: `openai|local`)
  - Starts scan asynchronously; returns accepted run envelope (`run_id`, status, message).
  - Preserves existing scan engine behavior (scan artifact writer remains canonical).

- `GET /api/scan/status`
  - Current run status snapshot (safe operator fields only).

- `GET /api/scan/progress` (WebSocket)
  - Emits safe progress event using current control-center state.
  - No credentials or secrets in stream.

### Frontend Scan Control Center

New dashboard page `/scan-control`:

- profile selector
- module selector
- flags (`no_http`, `neo4j`)
- optional provider selector (only if already supported)
- configured/not-configured badges
- profile validation button
- start scan button
- live run status/progress panel (poll + websocket message)
- explicit note on current limitation: single active run model (latest run state shared across tabs/users)

Current UI behavior hardening:

- explicit render states: `loading`, `error`, `empty/unconfigured`, `ready`
- API-derived nullable lists normalized before `.length` / `.map` use
- WebSocket progress connection is non-fatal; API polling status/history remains authoritative

### Security invariants

1. No raw AWS/OpenAI credentials in UI forms.
2. No plaintext secret persistence in frontend state, URL params, or API responses.
3. Backend emits capability/status booleans only.
4. Profile errors are sanitized to operator-safe messages.
5. Existing CLI behavior remains intact.

### Test coverage focus

- runtime status excludes secret values
- invalid/missing profiles handled clearly
- run-status endpoint returns safe state (idle/running/failed/completed)
- websocket still serves progress event
- list-like JSON fields in representative endpoints serialize as arrays (`[]`) when empty
- no regression to existing scans/findings/dashboard APIs

---

## 16. Open Source Launch

**Before v0.1.0:**
- Write companion blog post: real findings from a real org (anonymized).
  Concrete numbers: accounts scanned, reclaimable assets found, waste estimated.
  This drives GitHub stars more than any feature.
- Submit to Black Hat Arsenal 2027 (submission window typically opens ~8 months before).
- `CONTRIBUTING.md` with labeled "good first issue" types: new CDN fingerprints,
  new error page fingerprints, new cost pricing rules. These are self-contained,
  easy first contributions that don't require knowing the full codebase.

**Each collector is a standalone package.**
External contributors add `collectors/elasticbeanstalk.go` without touching
anything else. This is intentional — make contribution surface area obvious.

**License: Apache 2.0.**
Standard for security tools. Allows commercial use, which is important for
enterprise adoption and does not scare off contributors.

---

*End of tech-spec-v2.0-final*
