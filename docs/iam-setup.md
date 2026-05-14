# IAM Setup

Deploy `CloudriftAuditRole` in all member accounts using the provided StackSet template:

```bash
aws cloudformation create-stack-set \
  --stack-set-name cloudrift-audit \
  --template-body file://iam/stackset-template.yaml \
  --parameters ParameterKey=ManagementAccountId,ParameterValue=<id> \
               ParameterKey=ExternalId,ParameterValue=<random>
```

This is required for multi-account collection.

## Minimum intent and scope

- The role is intended for read-oriented inventory and analysis across org accounts.
- Cloudrift scan and dashboard flows rely on this role setup for account-wide visibility.
- If the StackSet is not deployed org-wide, reclaimability and trust findings are only valid within the scanned subset.

## Verify deployment

After StackSet rollout, verify role assumption from your management profile:

```bash
aws sts assume-role \
  --role-arn arn:aws:iam::<member-account-id>:role/CloudriftAuditRole \
  --role-session-name cloudrift-verify
```

For dashboard runtime checks, the Scan Control page also supports profile validation through `POST /api/runtime/validate-profile`.

## Related docs

- `docs/getting-started.md` for end-to-end local run instructions
- `docs/technical.md` for API and scan-control runtime behavior
