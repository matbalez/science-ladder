# Deploying Science Ladder

## Web, API, and persistence

The reference Fly.io applications are `science-ladder` and `science-ladder-api`. The API application contains separate API and worker process groups. Both use managed PostgreSQL for authoritative state; only the API/worker have private artifact-store credentials. The frontend proxies `/v1` and the public signing-key endpoint to the API over Fly's private network.

Create or select a dedicated managed PostgreSQL database and attach it as `DATABASE_URL`. Create a private Tigris bucket, then configure `S3_BUCKET`, `S3_REGION=auto`, `S3_ENDPOINT=https://t3.storage.dev` and its access credentials. Enable object versioning, verify a restoration to an isolated database, and document the result before official intake.

Configure the exact production origin and immutable operator GitHub ID. Do not give the web process any database, object-store, signing, or OpenAI credentials. Source snapshots and all authoritative artifacts survive application restarts; application filesystems are disposable.

Run from the repository root:

```sh
fly deploy . --config deploy/fly.api.toml
fly deploy . --config deploy/fly.web.toml
```

The API release command applies transactional migrations before a rollout. A failed migration stops deployment. Health checks distinguish a live process from database readiness. Use a clean, tested source snapshot for deployment so concurrent development cannot enter a build partway through a change.

## Initial submission capacity

Migrations create the logical queue limit with `maximum_units = 0`, so submission
acceptance returns `503 capacity_unavailable` until an operator configures intake.
Each accepted submission reserves **two units** for its primary run and fresh
confirmation. A limit of **six units** permits three outstanding submissions on a
serial single-host verifier; it does not promise six simultaneous microVMs or
independent hosts.

After enrolling an enabled, approved verifier with current trust evidence, an
operator with infrastructure database access can initialize a new deployment:

```sql
BEGIN;
SELECT maximum_units, reserved_units FROM capacity WHERE id = 1 FOR UPDATE;
UPDATE capacity
SET maximum_units = 6
WHERE id = 1 AND maximum_units = 0 AND reserved_units = 0
RETURNING maximum_units, reserved_units;
COMMIT;
```

The guarded update must return `(6, 0)`. If it returns no row, inspect the existing
configuration instead of resetting it. Record the previous limit, new limit,
operator, time, and reason in the deployment audit. Later limit changes must
preserve outstanding reservations and remain at least `reserved_units`. **Never
edit `reserved_units` or `capacity_reservations` manually**; acceptance and
resolution transactions maintain them.

This is operator infrastructure configuration. It does not approve a scientific
review, publish a challenge, accept a candidate, or change a result. Continue those
operations through the authenticated application workflow and its signed evidence.

## GitHub and OpenAI

Science Ladder uses a GitHub App with read-only repository contents/metadata. Install it only on repositories intended for challenge or submission ingestion. The public app callback is `/v1/auth/github/callback`; the webhook is `/v1/webhooks/github`.

`scripts/register-github-app.py` performs GitHub's manifest exchange through a short-lived loopback callback and writes credentials directly to a private local environment file. It validates state and does not print credentials. Review the script's arguments before using it for a different deployment.

Set a dedicated `OPENAI_API_KEY` and `OPENAI_REVIEW_MODEL` for scientific reviews. Review output is structured, versioned evidence; it does not replace checker conformance or human adoption of a challenge.

Preserve actual line breaks when storing private-key PEM values. Some dotenv import paths retain literal backslash-n sequences. Use the secret manager's raw-value input, never paste keys into source, shell history, logs, issues, or chat. Empty/malformed signing credentials cause initialization to fail.

## Verification hosts

The public API never runs a challenge checker. Configure the separate mTLS runner listener and enroll each physical host in the control plane with its certificate fingerprint, signing public key, and administrator-assigned failure-domain group. Two virtual servers do not establish physical independence by themselves.

Run host inspection before installation. Official verification requires Linux amd64 with KVM, approved pinned Firecracker/Jailer/kernel/rootfs artifacts, dedicated CPU isolation, disabled SMT/KSM/swap, no guest NIC, bounded outputs, and private scratch storage. Enrollment and physical-host evidence must agree. Refer to the runner's configuration and diagnostics for the enforced profile.

Quarantine receives exact immutable source inputs and job-scoped upload grants. It rebuilds deterministic read-only disks twice, runs fixtures/probes, and reports the immutable outputs. Validation workers receive exact disk digests and a one-use result capability; they never receive the root signing key or broad bucket credentials.

## Signing and witnesses

`DEPLOYMENT_MODE=controlled-demo` permits an explicitly labelled online PEM for demonstrations. `production` refuses raw application keys and requires `RECEIPT_KMS_KEY_ID`, `RECEIPT_KMS_REGION`, and root delegation. `SIGNING_AWS_*` credentials are separate from S3-compatible storage credentials. Prefer short-lived workload identity to long-lived credentials. The adapter pins the resolved immutable KMS key ARN and verifies the returned signature locally.

Configure `ROOT_PUBLIC_KEY_FILE`, `ROOT_KEY_ID`, `KEY_HISTORY_FILE`, and `OFFICIAL_RELEASE_ATTESTATION_FILE` as deployment-mounted public trust records. Keep the root private key outside the web/API/worker environment. The `SOURCE_COMMIT` container build argument stamps the reviewed commit into the binary; a development build cannot satisfy an official attestation. The signed attestation must authorize that exact source and key history.

Recruit three independently administered witnesses. Running three copies under one account is a useful test but does not establish independent witnessing. The service immediately reports missing quorum, and production acceptance stops after one hour without a current verified quorum. Existing accepted work can continue to resolution.
