# Cloudrift - Security Coverage & Scoring Reference

> **Implementation note:** This document describes the **intended detection and scoring model** used by collectors/scorers in `internal/`. The default **`cloudrift scan`** path today still writes an **empty** `findings.json` (orchestration gap). Use **`cloudrift demo generate`** or tests to exercise populated findings until the CLI wires the full pipeline.

## What Cloudrift does **not** detect (limits)

- **Application-layer bugs** — XSS, SQLi, auth bypass in your apps are not inferred from DNS alone.
- **Insider threat or runtime exfiltration** — no host-based or network IDS story here.
- **Misconfigurations inside VPC-only private APIs** unless collectors and IAM permissions reach them.
- **Historical CloudTrail proof of abuse** — trust scoring uses IAM last-used metadata, not full log analytics (see “Not yet collected” below).
- **Effective IAM permissions** — admin/privilege analysis reads attached managed-policy **names** plus inline policy documents only; it does **not** fetch and evaluate managed-policy documents, condition keys, SCPs, or permission boundaries. See the permission-tier caveats below.
- **Confirmed third-party CNAME pairing** — third-party takeover signatures (GitHub Pages, Heroku, Shopify) match on **response-body fingerprints** and can false-positive. A planned refinement pairs each match with the record's CNAME-target suffix (`github.io`, `herokuapp.com`, `myshopify.com`) before confirming.
- **Guaranteed completeness** — DNS/HTTP probes can miss transient failures; false positives and false negatives are possible; human review of `evidence` fields is expected. Per-account scan failures further limit coverage (see **Scan coverage** below).

---

## What Attacks Are We Protecting Against?

Cloudrift detects two categories of real-world attack surfaces in AWS environments:

---

### 1. Subdomain Takeover / Orphaned Edge Assets

When DNS records point to resources that no longer exist (deleted S3 buckets, removed CloudFront distributions, stale API Gateway endpoints, or unclaimed third-party SaaS sites), an attacker can reclaim the underlying resource and serve malicious content under your domain.

Detection is driven by an **extensible fingerprint catalog** (`takeoverSignatures` in `internal/validators/http.go`). Each entry matches on HTTP status, `Server` header, and/or response-body substrings, and is flagged `claimable` when the backing name is genuinely takeover-able (versus a misconfigured-but-still-controlled endpoint). Coverage scales by adding signatures to that table.

**Attack scenarios detected:**

| Scenario | What Happens | Example |
|---|---|---|
| **Subdomain takeover via S3** | DNS resolves to an S3 website endpoint but the bucket was deleted (`NoSuchBucket`). Cloudrift confirms the bucket is absent from **all** scanned org accounts (cross-account check) before flagging reclaimable. Attacker creates a bucket with the same name in any AWS account and hijacks the domain. | `docs.company.com` → `docs-company.s3-website-us-east-1.amazonaws.com` (bucket deleted) |
| **Claimable third-party CNAME** | DNS resolves to an unclaimed third-party SaaS endpoint that returns a known "site not found" body. Attacker registers the app/site name and serves content under your domain. Detected for **GitHub Pages**, **Heroku**, and **Shopify**. *(Body-fingerprint only — can false-positive; see limits above.)* | `blog.company.com` → `company.github.io` ("There isn't a GitHub Pages site here") |
| **Dangling AWS endpoint** | DNS resolves to a live AWS-controlled endpoint but the backing resource is misconfigured or deleted. Covers CloudFront origin errors and **API Gateway** missing custom-domain mappings (403 `{"message":"Forbidden"}`). Endpoint is AWS-controlled, so not freely reclaimable. | `api.company.com` → API Gateway with no matching custom-domain mapping |
| **CDN hostname bypass** | DNS resolves to a CloudFront target, but the hostname is not in the distribution's alternate-domains (alias) allowlist. The CDN may reject or misroute the request. Verified via a real alias-allowlist join in the pipeline. | `cdn.company.com` resolves to CloudFront but is absent from the distribution's alias list |
| **Broken DNS** | DNS record returns NXDOMAIN, timeout, or SERVFAIL. No active takeover risk, but indicates stale or misconfigured records. | Orphaned `A` or `CNAME` record with no live target |

---

### 2. External Access (IAM Trust + Resource-Based Policy Exposure)

External access is now detected from **two independent sources**, both reported under the `external_access` module:

1. **IAM role trust** (`internal/scorers/trust.go`) — roles whose `AssumeRole` trust policy grants an external AWS account, SAML, or OIDC provider the ability to assume the role.
2. **S3 bucket-policy resource exposure** (`internal/scorers/resource_exposure.go`) — buckets whose **resource-based policy** grants read/write to an external account or to the public (`Principal: "*"`). This catches exposure that role-trust scanning misses entirely.

#### 2a. IAM role trust

When these trusts are never rotated, granted to unknown vendors, or carry admin privileges, they become persistent backdoors.

**Attack scenarios detected:**

| Scenario | What Happens | Example |
|---|---|---|
| **Ghost admin access** | An external principal can assume a role with admin-level permissions. Direct privileged access outside your control boundary. The admin signal is bridged from the role's permission-tier analysis into severity (see caveats). | Third-party vendor role with `AdministratorAccess` policy attached, never reviewed |
| **Unknown vendor trust** | Role trusts an external AWS account not in your approved vendor list. Could be a former contractor, acquired company, or misconfiguration. | Role trusts account `123456789012` which is not in `approved_external_accounts` config |
| **Never-used / stale trust** | Role was created with an external trust but has never been used, or hasn't been used in over a year. Latent access with no active justification. | IAM role with cross-account trust, `RoleLastUsed` is null or >365 days ago |
| **Aging trust** | Role was last used 90–365 days ago. Still technically valid; should be reviewed and rotated. | Vendor integration role last assumed 6 months ago |

#### 2b. S3 bucket-policy resource exposure

A bucket policy can grant access to anyone or to outside accounts without any IAM role being involved. Cloudrift reads each bucket's policy (`s3:GetBucketPolicy`) and scores every external/public Allow grant.

**Attack scenarios detected:**

| Scenario | What Happens | Example |
|---|---|---|
| **Public write** | Bucket policy grants write to `Principal: "*"`. Anyone can tamper with objects or host malware under your bucket. | `Allow s3:PutObject` to `*` |
| **Public read** | Bucket policy grants read to `Principal: "*"`. Potential data exposure to the internet. | `Allow s3:GetObject` to `*` |
| **External write** | Bucket policy grants write to an external (non-owning) AWS account. Cross-account data tampering risk. | `Allow s3:PutObject` to `arn:aws:iam::123456789012:root` |
| **External read** | Bucket policy grants read to an external AWS account not on the approved list. Cross-account data exposure. | `Allow s3:GetObject` to an unapproved external account |
| **Approved-vendor grant** | Bucket policy grants access to an account that **is** in `approved_external_accounts`. Recorded for visibility at low severity. | Read grant to a vetted partner account |

---

## AWS Services Fetched Per Account

Cloudrift assumes a read-only audit role (`CloudriftAuditRole`) into each account in the AWS Organization and collects the following:

| Service | What Is Fetched | Why |
|---|---|---|
| **AWS Organizations** | Account IDs, names, OU paths, tags (`Team`, `Owner`, `Contact`) | Builds the account inventory; derives ownership context for findings |
| **Route 53** | All hosted zones, all record sets (A, CNAME, Alias); filters out SOA/NS records | Identifies DNS targets pointing to AWS services |
| **S3** | Bucket names, regions, website-hosting config, website endpoint URLs, public access block settings, tags | Validates whether a bucket referenced by DNS still exists and who owns it |
| **S3 bucket policy** | `s3:GetBucketPolicy` per bucket; parses Allow statements for external-account / public (`Principal: "*"`) grants | Detects resource-based exposure (read/write to outside accounts or the internet) that IAM role-trust scanning misses |
| **CloudFront** | Distribution domains, alternate CNAMEs, origins (S3 / custom), ACM certificate ARNs, enabled status | Cross-checks DNS targets against active distributions and their hostname lists |
| **IAM** | All roles, trust policies, attached managed policies, inline policy documents | Detects external trust relationships and scores permission exposure |
| **IAM Activity** | `RoleLastUsed.LastUsedDate` per role via `iam:GetRole` | Determines how stale a trust relationship is |
| **STS** | `GetCallerIdentity` | Confirms assumed-role identity during cross-account scanning |
| **Cost Explorer** *(optional)* | `GetCostAndUsage` (last 30 days, grouped by account + service) | Enriches findings with actual monthly spend for FinOps prioritization |

> **Not yet collected (planned):** ACM certificate details, API Gateway custom domains, CloudTrail `AssumeRole` events.

---

## What Is Mapped and How Resources Relate

Cloudrift builds a directed graph of relationships between resources. This graph powers blast-radius analysis - given a compromised resource, what else is reachable?

| Relationship | From | To | Meaning |
|---|---|---|---|
| `POINTS_TO` | DNS record | S3 / CloudFront / API Gateway | Hostname resolution target |
| `OWNED_BY` | Any asset | AWS Account | Which account the resource belongs to |
| `FRONTS` | CloudFront distribution | S3 bucket | S3 bucket backing the distribution as origin |
| `USES_CERT` | CloudFront distribution | ACM certificate | TLS certificate bound to the distribution |
| `TRUSTS` | IAM role | External principal | Cross-account or federated identity allowed to assume the role |

When exported to Neo4j, findings are attached to assets via `:AFFECTS` edges, and scan snapshots link to all findings via `:CAPTURED` edges. This allows queries like:
- *"Which accounts are reachable from this external principal?"*
- *"What is the blast radius if this IAM role is compromised?"*
- *"Which CloudFront distributions use a certificate that is about to expire?"*

---

## How Criticality Is Determined

### Orphaned Edge / Subdomain Takeover

Severity is assigned based on **claimability** - whether an attacker can actively take over the resource:

| Severity | Claimability | Condition | Reasoning |
|---|---|---|---|
| **Critical** | reclaimable | DNS resolves, S3 website endpoint with `NoSuchBucket`, and the bucket does not exist in **any scanned account** | Attacker can create the bucket in any account and immediately hijack the domain. *(Auto-downgraded to High when scan coverage is incomplete — see below.)* |
| **High** | reclaimable | DNS resolves and the backing name is takeover-able but not the cross-account-verified S3 case — e.g. a deleted S3 REST target, or an unclaimed third-party site (GitHub Pages, Heroku, Shopify) | Backing name is claimable, but with lower confidence than the org-wide-verified S3 critical |
| **High** | dangling | DNS resolves to an AWS-controlled endpoint, but origin/target is deleted or misconfigured (e.g. CloudFront origin error, API Gateway missing mapping) | Endpoint is AWS-controlled (not freely reclaimable) but exploitable; attacker may manipulate routing |
| **Medium** | edge_obscured | DNS resolves to CloudFront, but the hostname is not in the distribution's alias allowlist | CDN may reject the hostname; possible origin bypass or misrouting |
| **Low** | broken | DNS returns NXDOMAIN, timeout, or SERVFAIL | Record is broken but no active takeover vector exists |
| **Info** | unknown | Insufficient evidence to classify | Probe inconclusive |

**Cost risk multipliers applied on top of severity:**
- Critical (reclaimable): **5×** the estimated monthly resource cost
- High (dangling): **3×** the estimated monthly resource cost
- Others: **1×** (informational only)

> **Scan coverage safeguard (and caveat).** The reclaimable/critical S3 verdict is **absence-based** — it asserts the bucket exists in *no* scanned account. Collectors are resilient: a per-account failure (e.g. an account whose role can't be assumed) is **skipped, not fatal**, and `scan-metadata.json` records `coverage_complete` plus `failed_account_ids`. When coverage is **incomplete**, a "missing" bucket could in fact be owned by an account that failed to scan, so the pipeline **automatically downgrades reclaimable/critical to High** and stamps a `coverage_note` on the finding. This is a correctness safeguard, but it is also a caveat: re-run with full coverage before treating a downgraded finding as merely High.

---

### External IAM Trust

Severity is a combination of **activity staleness**, **admin privilege**, and **vendor approval status** — whichever produces the highest severity wins:

| Severity | Condition |
|---|---|
| **Critical** | External trust exists AND the role is admin-equivalent (`AdministratorAccess` managed policy attached, or an Allow with `Action: ["*"]` + `Resource: ["*"]`, or the equivalent capability bundle). Only escalates above the activity-derived base. |
| **High** | Role has never been used OR last used > 365 days ago (ghost/stale access) |
| **High** | Trusting external account is not in the approved vendor list (escalates the base only if it would raise severity) |
| **Medium** | Role last used 90–365 days ago (aging, should be reviewed) |
| **Low** | Role last used within the last 90 days (active, periodic review sufficient) |

The admin-equivalence used for the Critical escalation is **bridged from the role's permission-tier analysis** (`Capabilities.AdminLike`), not just an explicit `is_admin` hint — so a role classified admin-like from real policy data triggers ghost-admin even when collectors did not set an `is_admin` flag.

### S3 Bucket-Policy Resource Exposure

Severity for resource-based bucket-policy grants (`internal/scorers/resource_exposure.go`):

| Severity | Condition |
|---|---|
| **Critical** | Public (`Principal: "*"`) grant that includes write actions (`public_write`) |
| **High** | Public read-only grant (`public_read`) |
| **High** | Write grant to an external account (`external_write`) |
| **Medium** | Read-only grant to an external account (`external_read`) |
| **Low** | Grant to an account on the approved-vendor list (`approved_vendor_grant`) |

### Permission tiers used to detect admin-level access

| Tier | How Detected |
|---|---|
| **Admin** | An inline policy Allow with `Action: ["*"]` + `Resource: ["*"]`, or attached managed policy `AdministratorAccess` (by **name**) |
| **Privileged** | Role can write IAM policies AND assume other roles AND control CloudFront (privilege escalation chain) |
| **Scoped** | Role has at least one elevated capability: IAM write, S3 write, CloudFront control, or role chaining |
| **Limited** | Allow statements present, no elevated capabilities detected |
| **Unknown** | Policy could not be parsed, or no policy evidence found (treated conservatively as caution, not safety) |

> **Permission-analysis caveat (heuristic).** The tier analysis (`internal/scorers/permission_visibility.go`) is **heuristic** and intentionally scoped: its `analysis_mode` is `attached_names_plus_inline_docs`. It inspects attached managed-policy **names** (matching `AdministratorAccess`, `IAMFullAccess`, `AmazonS3FullAccess`, `AmazonCloudFrontFullAccess`) and parses **inline** policy documents — but it does **not** fetch and evaluate managed-policy *documents* (`managed_policy_documents_inspected = false`), condition keys, permission boundaries, or SCPs. Name-only matches are flagged non-authoritative and confidence is reduced accordingly. "Unknown" means insufficient evidence, not "safe."
