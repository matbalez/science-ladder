# PostgreSQL restore rehearsal — 2026-09-04

Status: **connection blocked; restore verification incomplete**.

Managed PostgreSQL 17 backup `20260904-220431F` completed. A restore created the separate `science-ladder-restore-drill` cluster, which reached the provider's ready state. Comparison through local Fly WireGuard proxies failed during the restored cluster's TLS handshake, before authentication or SQL. The live cluster's certificate verified successfully through its corresponding proxy. Retrying the restored endpoint with its own credentials and direct endpoint server name did not resolve the TLS failure. Credentials were not added to the repository.

The first restored-copy connection failed before any SQL query executed. Consequently, this run did **not** compare migration inventories, table counts or content hashes, audit-chain continuity, or the restored schema. It did not modify either database, run migrations, seed hosted test data, or delete infrastructure.

The comparison tool is prepared to take read-only, repeatable-read snapshots of both databases; fingerprint each public table and schema constraints; verify the audit hash chain and the backup's exact live-history prefix; and record the restored baseline before optionally applying current migrations to the restored copy. Its intermediate files stay outside the repository in the private task workspace. The temporary restored cluster was scheduled for deletion after the failed rehearsal; the live cluster and original backup were retained. A subsequent drill must resolve restored-endpoint connectivity before making a recovery claim.

Even a successful comparison of this early, mostly empty deployment would not demonstrate restoration of challenge artifacts, private submissions, hidden-suite keys, active runner leases, or competition adjudication. Object-storage recovery, key custody, in-flight job recovery, witness history, and a populated competition restore drill remain separate requirements. This rehearsal does not satisfy the production release gate.
