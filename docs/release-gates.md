# Release evidence

This checklist separates implemented mechanisms, observed tests, and external launch evidence. Mark a gate complete only with a dated, retained result. A platform-seeded challenge or internal agent test does not count as an external pilot or independent review.

## Required for the public demonstration

- Public MIT repository and reproducible application builds.
- Live HTTPS application, GitHub sign-in, explicit invited writes, capped capacity.
- PostgreSQL durability and private immutable object storage.
- Genuine challenge package, archived sources, adopted scientific review, executable verification suite.
- Genuine attributed solver attempts and displayed receipts; no invented scores.
- Clear deployment mode and visible verification availability.

## Required before official competition

- Independent security assessment of archive parsing, quarantine, Firecracker/Jailer boundary, authorizations, and result acceptance.
- Two inventory-backed physical failure domains; official runner isolation and adversarial probes observed on the exact deployment profile.
- KMS root/online signing separation, revocable per-host identities, signed key-history bootstrap and rotation drill.
- Three independently administered witnesses, a verified 2-of-3 quorum, and tested fork detection/outage recovery.
- Database restoration to an isolated environment with receipt ordering, object digests, and audit checkpoints reconciled.
- Incident/retry/compromise recovery preserving accepted receipt history and restored entitlements.
- A real external invitation pilot with creator, solver, and independent reproduction feedback.

The root-signed `OfficialReleaseAttestation` binds the reviewed source commit and key-history digest to six retained evidence records: `independent-security-review`, `database-restore-drill`, `key-rotation-drill`, `runner-isolation-drill`, `witness-outage-and-fork-drill`, and `external-invitation-pilot`. Each identifies its assessor, HTTPS evidence location, content digest, and completion time. Its expiry is enforced.

The software verifies signatures, bindings, required records, and validity windows. Human operators remain responsible for the substance and independence of the assessments. A configuration flag is not a substitute for evidence.
