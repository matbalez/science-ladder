# Isolated runner

`runnerd` is a separate executable and trust zone. The web/API never launch checker
code. `runnerd serve` uses a dedicated mTLS listener, verifies signed jobs, consumes
exact-object grants, uploads deterministic disks with bounded one-job PUT grants,
and returns signed typed results. A failed trust or resource check fails closed.

The Linux guest uses `/sbin/sl-init` (the `runnerd` executable installed under that
name). Rootfs composition is a first-party fixed recipe: `runnerd build-rootfs`
accepts digest-pinned runtime and filesystem-tools images and a Linux amd64 binary.
It never runs a challenge. Build the filesystem-tools image with the supplied
Dockerfile, an approved immutable Debian base and a reviewed Debian snapshot;
record its resulting OCI digest. Pin the produced ext4 digest and a reviewed kernel
with KVM/virtio-block, SquashFS, ext4, cgroup v2, PID/memory controllers and seccomp.

Official host configuration requires:

- Linux amd64 KVM, exclusive physical tenancy, disabled SMT/KSM/swap, pinned CPUs;
- expiring platform-signed inventory/egress attestation and separate host identity;
- immutable kernel/rootfs/Firecracker/Jailer/mksquashfs digests;
- private tmpfs work storage, an empty host network namespace and no guest NIC;
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
profile. SBOM/vulnerability review and external runner security review remain
separate release gates; these structural checks are not a security certification.

`local-preflight --unsafe-local` and `sl validate --local --unsafe-local` are developer
tools using a pinned Docker runtime with no network. Their output explicitly says
`official: false` and cannot establish a competitive outcome.

Do not advertise production readiness from unit tests or successful local runs.
Required deployment evidence includes actual KVM boot, cross-host determinism,
anti-affinity and failure recovery, credential/metadata/hidden-output probes,
external security review, key rotation, database restore and witness quorum drills.
