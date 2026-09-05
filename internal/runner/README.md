# Isolated runner

`runnerd` is a separate executable and trust zone. The web/API never launch checker
code. `runnerd serve` uses a dedicated mTLS listener, verifies signed jobs, consumes
exact-object grants, uploads deterministic disks with bounded one-job PUT grants,
and returns signed typed results. A failed trust or resource check fails closed.
Before every claim, signed host authorization must have more than twenty minutes
remaining. The daemon automatically renews its exact operator-approved enrollment
on startup and with six hours remaining. Temporary renewal failures retain the
valid lease and retry; expiry pauses claims until automatic recovery. Stale
advisory evidence blocks new checker preflight only, not existing challenge
verification. Actual host controls remain mandatory before every execution. See the
[renewal runbook](../../docs/runner-renewal.md).

The Linux guest uses `/sbin/sl-init` (the `runnerd` executable installed under that
name). Rootfs composition is a first-party fixed recipe: `runnerd build-rootfs`
accepts digest-pinned runtime and filesystem-tools images and a Linux amd64 binary.
It never runs a challenge. Build the filesystem-tools image with the supplied
Dockerfile, an approved immutable Debian base and a reviewed Debian snapshot;
record its resulting OCI digest. The platform-tools base must already include
trusted HTTPS CA certificates. Pin the produced ext4 digest and a reviewed kernel
with KVM/virtio-block, SquashFS, ext4, cgroup v2, PID/memory controllers and seccomp.

Official host configuration requires:

- Linux amd64 KVM, exclusive physical tenancy, disabled SMT/KSM/swap, pinned CPUs;
- expiring platform-signed inventory/egress attestation and separate host identity;
- immutable kernel/rootfs/Firecracker/Jailer/mksquashfs/custom CPU-template digests;
- private tmpfs work storage that allows the Jailer device nodes and VMM executable,
  an empty host network namespace and no guest NIC;
- per-host signing through a private socket-backed key agent, never a platform root key;
- an encrypted, private result spool for retry/reconciliation, separate from guest storage.

The Firecracker Jailer owns the VMM namespace, UID and cgroup boundary. The guest
mounts challenge, validator, suite, submission and job configuration read-only,
runs a direct Python argv as UID 65534 with no capabilities, applies PID/memory/file
limits, captures arbitrary logs privately, and emits one bounded typed result.
Writable guest state is RAM-backed and destroyed with its one-job microVM.

Quarantine preflight accepts exact GitHub snapshots. It rejects unsafe paths,
credential indicators, source shell builds and arbitrary Dockerfiles; constructs
the validator disk twice offline; requires matching digests; and runs baseline,
valid/invalid/malformed fixtures twice plus platform isolation/resource probes.
Dependencies are vendored, SHA256-locked `py3-none-any` wheels. Native wheels,
install hooks and network dependency resolution are not supported by this initial
profile. Every preflight generates a CycloneDX SBOM from the platform runtime and
all transitive locked wheel metadata. A platform-controlled, signed, digest-pinned
advisory snapshot must cover each exact normalized package version; unknown or
stale coverage and high/critical findings fail closed. The snapshot expires within
seven days, cites archived primary-source bytes and is bound to the signed job.
Source/package inventory and advisory keys are also pinned in the host attestation.
No creator can supply a clean scan. External security review remains a separate
release gate; successful mechanical checks are not a security certification.

`runnerd runtime-inventory --image IMAGE@sha256:DIGEST --out NEW_FILE` derives the
base inventory using a fixed command against the pinned image. The distroless
runtime records the complete retained-file list and original Debian binary/source
versions under `/usr/share/science-ladder/runtime-components.json`. It retains the
working installed Python standard library and its actual ELF library closure,
upstream copyright files, CA certificates and timezone data. It omits package
installers and unrelated system programs. Optional native extensions whose shared
libraries were already missing from the upstream slim image are listed explicitly
as unavailable; this does not assert that the retained libraries are CVE-free.
`runnerd dependency-inventory --runtime-inventory FILE --snapshot FILE --out NEW_FILE`
adds all vendored challenge wheels for advisory collection. The runner never
downloads advisory or dependency data while preparing a challenge.

Hidden suite source and disks remain AES-256-GCM encrypted in object storage.
Per-job X25519 capsules bind the random suite key to a named host and job; only
private tmpfs holds decrypted suite data. Deterministic disk encryption uses a
keyed nonce derived from the disk plaintext so independent byte-identical builds
agree without reusing a nonce for different disk contents. Public locks expose a
salted commitment and encrypted disk digest. Only an authorized post-season
reveal may publish key material; ordinary receipts never expose it.

`local-preflight --unsafe-local` and `sl validate --local --unsafe-local` are developer
tools using a pinned Docker runtime with no network. Their output explicitly says
`official: false` and cannot establish a competitive outcome.
Local hidden-suite creators can add `--private-suite DIRECTORY` to `sl validate`
or `sl challenge test`; this previews their own files without downloading hosted
secrets or verifying the official commitment. `sl suite upload` uploads a bounded
inert source document over the authenticated API for platform encryption.

`runnerd hardware-probe` runs only embedded, first-party synthetic isolation and
resource probes on an enrolled host. Use the same `--config`, `--keys`,
`--host-keys`, `--key-id`, and `--signer-socket` options as `run`, plus `--out` for a
new receipt file. It produces a distinct `HostConformanceReceipt`, always labels
official acceptance false, and explicitly does not establish advisory clearance
or cross-host determinism. It cannot accept scientific artifacts or hidden data.
Bounded boot diagnostics are visible only for this fixed platform corpus.

Do not advertise production readiness from unit tests or successful local runs.
Required deployment evidence includes actual KVM boot, fresh-VM repeatability,
failure recovery, credential/metadata/hidden-output probes, external security
review, key rotation, database restore and witness quorum drills. New manifests
default to immutable `verificationPolicy: platform`; successful repeated runs on
one enrolled host establish `platform_verified`. Explicit `independent` policies
add physical-host anti-affinity and cross-host determinism before establishing
`independently_replicated`. Locks predating this field retain independent-policy
semantics. Same-host repetitions never establish independent replication.
