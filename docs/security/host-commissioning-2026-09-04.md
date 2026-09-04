# Runner commissioning — 4 September 2026

The enrolled Science Ladder host completed all five fixed platform checks in fresh
Firecracker microVMs under Jailer. The signed report records **passed: true**,
cleanup completed, and a total corpus duration of **10.480 seconds**. Its signature
was verified with the included host public key.

This is **single-host platform evidence** in `controlled-demo` mode. The receipt
explicitly records `crossHostVerified: false` and `officialAcceptance: false`.
It is not independent replication or an official production launch certification.

## Evidence

- [Unmodified signed host conformance receipt](host-conformance-2026-09-04.json)
- [Host public verification key](host-conformance-public-keys.json)
- [Fixed platform corpus source](../../internal/runner/conformance.go)
- [Guest isolation and teardown source](../../internal/runner/guest_linux.go)
- [Patched runtime package and advisory evidence](runtime-patched-2026-09-04/evidence-index.json)

The receipt contains an opaque host identity/group, execution-profile digest,
timing and outcomes. It contains no host address, private key, server credential,
scientific candidate or private suite. Its signed bytes were not edited for publication.

| Fixed check | Observed result |
| --- | --- |
| Network/metadata access, credential paths, read-only inputs, unprivileged identity/capabilities, process and file bounds | All assertions passed |
| Deliberate CPU exhaustion | `resource_limit` |
| Deliberate memory exhaustion | `resource_limit` |
| Malformed checker result | `invalid_output` |
| Large checker-log output with one valid result | Valid result; arbitrary checker logs remained bounded |

The guest destroys the entire validator cgroup and checks that no descendant
remains before reading the result. It then resets the microVM through the
Firecracker-supported reset path. An outer host/lease deadline is classified as
an infrastructure failure; the guest reports actual checker resource exhaustion.

The delivery service uses a dedicated mode-0700 child directory,
`/var/lib/science-ladder/spool/results`, on the encrypted result volume. Diagnostic
probe receipts and filesystem housekeeping entries stay outside that delivery
directory. This preserves strict replay parsing; the signed host configuration
binds the delivery path separately from the unchanged execution profile.

## Commissioned artifact identities

| Artifact | SHA-256 |
| --- | --- |
| Execution profile | `1577c6ea734b0ddbed15ace603c37f9fe4ebf3e56c77b38cbf67517661013ca0` |
| Public runtime OCI | `a8136bf6f5082a72776f2565f279a6c336b462da483aa54cfa81586ad20705fa` |
| Complete retained runtime component inventory | `188492919acca87e0b11fde5b24464a2511e886a551d987a9f8583e879745226` |
| Runtime package inventory | `0dbe6e677aecaf5ee204bd49d4f167f6496bcbb37c6d8348daf23dd2657bcd2a` |
| Guest root filesystem | `1646f4a0089ffaa94d77e65df6d3a3044de0e016eea911de8dbb8a61fed5fbea` |
| Linux amd64 runner / guest init binary | `a7a1676400d0aabf2d5bf5a933d74ad7725a98981230a9810b8bc8ee6c1ec153` |
| Guest Linux kernel 6.1.182 | `9b7e715caab6629caa881a481a091dc33a65ec901ff1486904b5ac905a5f8578` |
| Firecracker 1.16.1 | `2fd0171309af7e24cf8dafc8a6f921c1434c49b5f9349bb996b7ed0a4deb8aa7` |
| Jailer 1.16.1 | `1f3a0c1fe86212d0001819bfe0819071c01208b3ccc9398c3b3bc1b84cf21edd` |
| C3 CPU template | `23312035388df2efbdee7d4ae7854e57e05a5d7d90e53d1306b5316bcdb0c250` |
| Host SquashFS builder | `47d5c1af3da11864e64c9dc6bb4e568719dcc315e6a744e79381ce3374fb7393` |
| Signed receipt file | `c512ef0b3d193c60608af9edd7910d4e1e3954895963548bc6ca03ad126804c7` |

The public runtime is
`ghcr.io/matbalez/science-ladder-python@sha256:a8136bf6f5082a72776f2565f279a6c336b462da483aa54cfa81586ad20705fa`.
It was published by [the runtime build](https://github.com/matbalez/science-ladder/actions/runs/33926498541)
using the [fixed patched recipe](../../internal/runner/assets/validator-runtime-patched.Dockerfile).
Its retained component inventory matched the reviewed build exactly. Fresh local
checker-contract tests also passed against this public OCI digest.

Firecracker, Jailer and the CPU template came from the
[upstream 1.16.1 release](https://github.com/firecracker-microvm/firecracker/releases/tag/v1.16.1).
The kernel came from the
[Firecracker CI kernel artifact](https://s3.amazonaws.com/spec.ccfc.min/firecracker-ci/20260902-a6146c8bb213-0/x86_64/vmlinux-6.1.182).
The [rootfs build procedure](../../internal/runner/README.md) constructs immutable
guest inputs from pinned platform components. First-party code is MIT; retained
upstream runtime components keep their own licenses.

The probe intentionally records `advisoryGateSatisfied: false`: this hardware
corpus does not perform dependency review. A separate platform-controlled signed
advisory snapshot covers the exact runtime packages. The accompanying public
scanner report labels its own unsigned status and retains the remaining
medium/moderate/low findings. Neither report establishes absence of all vulnerabilities.

## Verify the published report

From the repository root:

```sh
go run ./cmd/sl receipt verify \
  --receipt docs/security/host-conformance-2026-09-04.json \
  --keys docs/security/host-conformance-public-keys.json
```

The expected result is `signatureValid: true`. This verifies integrity under the
published host key; authority, key delegation and audit history are separate checks.
