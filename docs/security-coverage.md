# Cloudrift - Security Coverage & Scoring Reference

## What Attacks Are We Protecting Against?

Cloudrift detects two categories of real-world attack surfaces in AWS environments:

---

### 1. Subdomain Takeover / Orphaned Edge Assets

When DNS records point to AWS resources that no longer exist (deleted S3 buckets, removed CloudFront distributions, stale API Gateway endpoints), an attacker can reclaim the underlying resource and serve malicious content under your domain.

**Attack scenarios detected:**

| Scenario | What Happens | Example |
|---|---|---|
| **Subdomain takeover via S3** | DNS CNAME points to an S3 website endpoint but the bucket was deleted. Attacker creates a bucket with the same name in any AWS account and hijacks the domain. | `docs.company.com` → `docs-company.s3-website-us-east-1.amazonaws.com` (bucket deleted) |
| **Dangling AWS endpoint** | DNS resolves to a live AWS-controlled endpoint (CloudFront, API Gateway) but the backing resource is misconfigured or deleted. Returns 403/404 with AWS fingerprints. | `api.company.com` → CloudFront distribution with deleted origin |
| **CDN hostname bypass** | DNS resolves to a CloudFront IP, but the hostname is not in the distribution's alternate domains list. The CDN may reject or misroute the request. | `cdn.company.com` resolves to CloudFront but is absent from distribution's `CNAME` list |
| **Broken DNS** | DNS record returns NXDOMAIN, timeout, or SERVFAIL. No active takeover risk, but indicates stale or misconfigured records. | Orphaned `A` or `CNAME` record with no live target |

---

### 2. Stale / Unapproved External IAM Trust

IAM roles with cross-account trust policies grant external AWS accounts, SAML identity providers, or OIDC providers the ability to assume the role. When these trusts are never rotated, granted to unknown vendors, or carry admin privileges, they become persistent backdoors.

**Attack scenarios detected:**

| Scenario | What Happens | Example |
|---|---|---|
| **Ghost admin access** | An external principal can assume a role with admin-level permissions. Direct privileged access outside your control boundary. | Third-party vendor role with `AdministratorAccess` policy attached, never reviewed |
| **Unknown vendor trust** | Role trusts an external AWS account not in your approved vendor list. Could be a former contractor, acquired company, or misconfiguration. | Role trusts account `123456789012` which is not in `approved_external_accounts` config |
| **Never-used / stale trust** | Role was created with an external trust but has never been used, or hasn't been used in over a year. Latent access with no active justification. | IAM role with cross-account trust, `RoleLastUsed` is null or >365 days ago |
| **Aging trust** | Role was last used 90–365 days ago. Still technically valid; should be reviewed and rotated. | Vendor integration role last assumed 6 months ago |

---

## AWS Services Fetched Per Account

Cloudrift assumes a read-only audit role (`CloudriftAuditRole`) into each account in the AWS Organization and collects the following:

| Service | What Is Fetched | Why |
|---|---|---|
| **AWS Organizations** | Account IDs, names, OU paths, tags (`Team`, `Owner`, `Contact`) | Builds the account inventory; derives ownership context for findings |
| **Route 53** | All hosted zones, all record sets (A, CNAME, Alias); filters out SOA/NS records | Identifies DNS targets pointing to AWS services |
| **S3** | Bucket names, regions, website-hosting config, website endpoint URLs, public access block settings, tags | Validates whether a bucket referenced by DNS still exists and who owns it |
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

| Severity | Condition | Reasoning |
|---|---|---|
| **Critical** | DNS resolves, S3 website endpoint, bucket does not exist in any scanned account | Attacker can create the bucket in any account and immediately hijack the domain |
| **High** | DNS resolves to an AWS-controlled endpoint, but origin/target is deleted or misconfigured (403/4xx with AWS fingerprint) | Endpoint is live but exploitable; attacker may be able to manipulate routing |
| **Medium** | DNS resolves to a CloudFront IP, but the hostname is not listed in the distribution's alternate domains | CDN may reject the hostname; possible origin bypass or misrouting |
| **Low** | DNS returns NXDOMAIN, timeout, or SERVFAIL | Record is broken but no active takeover vector exists |
| **Info** | Insufficient evidence to classify | Probe inconclusive |

**Cost risk multipliers applied on top of severity:**
- Critical (reclaimable): **5×** the estimated monthly resource cost
- High (dangling): **3×** the estimated monthly resource cost
- Others: **1×** (informational only)

---

### External IAM Trust

Severity is a combination of **activity staleness**, **admin privilege**, and **vendor approval status** - whichever produces the highest severity wins:

| Severity | Condition |
|---|---|
| **Critical** | External trust exists AND the role has admin-level permissions (`AdministratorAccess` or `Actions: ["*"]` on `Resources: ["*"]`) |
| **High** | Role has never been used OR last used > 365 days ago (ghost access) |
| **High** | Trusting external account is not in the approved vendor list |
| **Medium** | Role last used 90–365 days ago (aging, should be reviewed) |
| **Low** | Role last used within the last 90 days (active, periodic review sufficient) |

**Permission tiers used to detect admin-level access:**

| Tier | How Detected |
|---|---|
| **Admin** | Policy contains `Actions: ["*"]` + `Resources: ["*"]`, or managed policy `AdministratorAccess` is attached |
| **Privileged** | Role can write IAM policies AND assume other roles AND control CloudFront (privilege escalation chain) |
| **Scoped** | Role has at least one elevated capability: S3 write, CloudFront control, or role chaining |
| **Limited** | Allow statements present, no elevated capabilities detected |
| **Unknown** | Policy could not be parsed, or no policy evidence found (treated conservatively) |

Roles flagged **Privileged** or above that also have external trust are escalated to the highest applicable severity tier.
