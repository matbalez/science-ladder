# Deployment status

The public source is MIT-licensed at [matbalez/science-ladder](https://github.com/matbalez/science-ladder). The hosted invitation preview runs at [science-ladder.fly.dev](https://science-ladder.fly.dev).

The application uses Fly Managed PostgreSQL 17 and a private Tigris content-addressed artifact bucket. Tigris snapshot support is enabled. Database backups and a separate restore rehearsal are being exercised; provisioning a backup is not a completed integrity/recovery drill.

This is a controlled demonstration deployment. A single dedicated physical verification host is being commissioned. The approved MVP policy permits platform verification on this host. Independent replication remains a separate status requiring a different physical host group; two processes, virtual machines, or labels on the same server do not establish that status.

New challenge locks explicitly select `platform` or `independent` verification. The MVP defaults to `platform`, allowing preflight, immutable challenge publication, accepted scores, milestone claims, and public-frontier advances on one host once the remaining execution and integrity checks pass. Fresh-VM repeatability checks remain in place. Existing locks retain their original policy. The host's actual isolation and runtime-advisory checks are still being completed; the policy change does not claim those checks have passed.

Current demonstration receipt and host signing keys use explicitly labelled software-key custody. Managed nonexportable signing, root-delegated key history, independently operated checkpoint witnesses, release evidence, and the external security/pilot gates are required before production acceptance. See [release gates](release-gates.md) and [persistence](persistence.md).
