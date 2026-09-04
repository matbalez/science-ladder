# Persistence and recovery

## What is authoritative

PostgreSQL is authoritative for identity, invitations, immutable challenge locks, quotas, submission receipt order, durable jobs, attempt leases, milestone claims, public visibility, and the append-only audit history. Use row locks and database constraints for competition invariants. Network calls and scientific execution take place outside transactions.

Tigris stores immutable bytes under content hashes: archived GitHub snapshots, canonical submissions, validator/suite disks, receipts, and logs. A database record binds each digest to its size, ownership, and publication state. Keep the bucket private. A solver can access their private submission; public downloads require committed publication authorization.

The GitHub repository hosts source code and reviewable challenge packages. A force-push or repository deletion cannot change a previously archived challenge lock. Container images are addressed by digest; tags are only discoverability conveniences.

## Hosted deployment

The reference application uses a dedicated Fly Managed Postgres 17 cluster in `sjc`, with the web/API/worker in the same region. Tigris is S3-compatible and billed through Fly.io. The application keeps no authoritative data on a Fly Machine's ephemeral filesystem.

Configure the managed database connection through a Fly secret. Database credentials are never present in the web app. Only the control API and worker have broad object-store credentials; verification hosts receive short-lived reads for the exact job inputs.

## Recovery procedure

1. Pause competitive intake before recovery. Preserve committed receipt order and the last independently witnessed checkpoint.
2. Restore PostgreSQL to a separate database using managed recovery. Do not overwrite the live cluster during a drill.
3. Run schema migrations against the restored copy and verify challenges, receipt sequences, adjudication watermarks, grant reservations, and uniqueness constraints.
4. Sample every artifact class and verify its stored size and SHA-256 digest. Reproduce a public frontier from exported artifacts on a clean verifier.
5. Reconcile durable job leases and result uploads. Supersede stale fencing tokens; never fabricate terminal scores to clear a queue.
6. Compare the restored audit chain with independently held checkpoints. A gap or fork keeps intake paused.
7. Rotate compromised credentials separately from scientific keys; preserve historical public key delegations and revocations so old receipts remain verifiable.
8. Record drill evidence, recovery point, elapsed recovery time, and all unresolved discrepancies before reopening intake.

Automated backups are not evidence that restoration works. An observed restore drill and independently replicated public audit objects remain release gates until recorded in the release checklist. Managed-service availability does not replace those application-level checks.

Official provider documentation: [Fly Managed Postgres](https://fly.io/docs/mpg/), [Tigris on Fly](https://fly.io/docs/tigris/).
