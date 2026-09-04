# Implementation decisions

The v0.2 documents in `specs/` preserve the source product and architecture. These decisions supersede their older recommendations:

- **MIT:** All first-party platform, protocol, CLI, runner, frontend, prompts, and conformance code is MIT licensed. Dependencies, papers, datasets, and submitted artifacts retain their own declared licenses.
- **Fly.io application hosting:** The public Next.js app, Go API and Go worker are separate processes on Fly.io. PostgreSQL and object storage remain external persistent services. No application filesystem is a source of scientific state.
- **PostgreSQL 17 and Tigris:** A dedicated Fly Managed Postgres cluster holds transactional state and the queue. A private S3-compatible Tigris bucket holds immutable content-addressed bytes. Public visibility is granted by the application after the relevant database transaction, never by making the private origin bucket public.
- **Separate validation infrastructure:** Dedicated Linux amd64 verification hosts run the Firecracker/Jailer contract. The application cannot execute a validator, and a missing trusted runner is a visible unavailable state.
- **Single Go module:** The source shares one Go module because the former split-license boundary no longer exists. The protocol package has no dependency on the platform database or hosted application.
- **Launch posture:** Public browsing, invited writes, capped validation, and no payments. The seeded demonstration is distinct from the externally reviewed invitation pilot. A real agent attempt is not an independent scientist, a completed pilot, or evidence of a new scientific discovery.

ECDSA P-256/SHA-256 DSSE, exact tick arithmetic, immutable locks, all-crossed milestones, receipt ordering, private losing artifacts, clean independent confirmation, and the external launch gates remain unchanged.
