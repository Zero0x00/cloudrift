# IAM setup

This file explains **slowly** how Cloudrift reaches AWS accounts and what you must deploy. For API-level scan control, see [technical.md](technical.md). For a visual walkthrough, see [starter-doc.html](../starter-doc.html) (IAM section).

---

## What you are trying to achieve

Cloudrift needs **read access** to many accounts in an AWS Organization. It does that by using **your** identity in the **management account** (or another hub account) to call **`sts:AssumeRole`** into a **member-account role** named like `CloudriftAuditRole`, then calling read-only APIs (Route53, S3 — including **`s3:GetBucketPolicy`** for resource-exposure detection — CloudFront, IAM, Organizations, etc.).

**What this means:** access keys or SSO in *one* account do **not** automatically see every other account. Each member account must **trust** your hub to assume the audit role.

---

## Access keys vs profiles vs roles

| Concept | What it is |
| --- | --- |
| **Access keys** | Long-lived `AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY` for an IAM user or access key on a role — convenient for automation, rotate regularly. |
| **Named profile** | A label in `~/.aws/credentials` and `~/.aws/config` that points at keys, SSO, or a role chain. |
| **Role assumption** | Short-lived credentials obtained by calling STS `AssumeRole` with a **role ARN** and sometimes an **external ID**. |

Cloudrift’s CLI reads **`[aws].management_profile`** from `cloudrift.toml` when set, or the **default credential chain** when it is empty. There is **no** `--profile` CLI flag.

---

## Single account vs multi-account

- **Single account:** you might run with credentials that already have read access in that one account. Org-wide inventory still expects the **same role pattern** if code paths assume `AssumeRole` into members — check [technical.md](technical.md) for how the collectors use Organizations APIs.
- **Multi-account (typical):** deploy **`CloudriftAuditRole`** in every member account via **CloudFormation StackSet** (or equivalent), with trust back to your management account principal and a **secret external ID**.

---

## StackSet deployment

Deploy `CloudriftAuditRole` in all member accounts using the provided template:

```bash
aws cloudformation create-stack-set \
  --stack-set-name cloudrift-audit \
  --template-body file://iam/stackset-template.yaml \
  --parameters ParameterKey=ManagementAccountId,ParameterValue=<id> \
               ParameterKey=ExternalId,ParameterValue=<random>
```

Adjust parameters to match your org’s naming and security standards.

---

## Minimum intent and scope

- The role is **read-oriented** inventory and analysis — not `AdministratorAccess` for Cloudrift.
- The action list covers the read-only APIs the collectors actually call: `organizations:ListAccounts/ListParents/ListTagsForResource/DescribeOrganizationalUnit`, `route53:ListHostedZones/ListResourceRecordSets`, `cloudfront:ListDistributions`, `iam` get/list role + policy reads, `sts:AssumeRole`, and S3 reads — `s3:ListAllMyBuckets`, `s3:GetBucketLocation`, `s3:GetBucketWebsite`, plus **`s3:GetBucketPolicy`** (required for the S3 bucket-policy resource-exposure detection). `ce:GetCostAndUsage` is optional, for FinOps cost enrichment.
- If StackSet is **not** deployed org-wide, findings are only as complete as the accounts your role can reach. Collectors are **resilient** — a per-account failure (role not assumable, an action denied) is skipped rather than fatal — and the scan records which accounts it reached. **Incomplete coverage automatically downgrades absence-based "reclaimable/critical" subdomain-takeover verdicts to High** (the "missing" resource could live in an account you couldn't scan). See [security-coverage.md](security-coverage.md).
- **Least privilege:** grant the actions in the template, not full `*`.

---

## Verify deployment

After rollout, prove you can assume the role from your management profile:

```bash
aws sts assume-role \
  --role-arn arn:aws:iam::<member-account-id>:role/CloudriftAuditRole \
  --role-session-name cloudrift-verify
```

The dashboard **Scan Control** page can validate a profile via `POST /api/runtime/validate-profile` (see [technical.md](technical.md)).

---

## Related files

- `iam/stackset-template.yaml` — trust policy and action list.
- [getting-started.md](getting-started.md) — credentials + first run.
- [security-coverage.md](security-coverage.md) — what findings mean once data exists.
