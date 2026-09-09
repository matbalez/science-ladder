# Proof runtime review — 8 September 2026

The proof profile is a separately pinned Linux guest on the existing Scaleway server. It preserves the legacy Quiet Echoes and numerical Load Paths profiles. It is not a second-server or independent-security-review claim.

## Execution and proof authority

Candidate Lean elaboration and export run as the candidate identity in a private filesystem, PID, mount, network, IPC and UTS namespace. A fixed setup program creates a read-only PID-only proc view before dropping groups and identity and entering the existing seccomp launcher. This provides Lean its own executable path without exposing the checker process tree. Checker tools and hidden assets are absent from the candidate view.

Build products are exact declared regular files with individual limits. The broker kills build descendants, seals the candidate tree, copies only those files, and exposes them read-only and non-executable to the checker. Serialized proof text remains untrusted input. Lean Comparator checks the target statement and allowed axioms and replays the certificate through Lean; DRAT-trim checks a certificate against the frozen CNF. The adapter does not import candidate-created binary Lean object files into the checker.

The proof/performance composition test establishes that proof checking gates the paired timing pipeline. Its tiny CNF is an interface fixture, not a proof of the timed C program's semantics. A real challenge must bind its semantic proof to the measured candidate.

## Third-party provenance and advisory coverage

The runtime inventory covers 190 components: 182 in the pinned split native runtime and eight proof components. Executable assets bind their full file inventories and package coordinates. A composite inventory binds the base OCI inventory plus the first-party guest, launcher and final root filesystem overlay.

| Component | Exact source or release |
| --- | --- |
| Lean | 4.33.1, commit `819816b2e0a3bf405af45ae5c7af2491d8f5bee6` |
| Comparator | `c0c5a52d2aff92b457c3e5ed4a68c1ebc5795809` |
| lean4export | `15f6055e299ad5b89345e533cc2192f4cc00f659` |
| DRAT-trim | `2e3b2dc0ecf938addbd779d42877b6ed69d9a985` |
| Linked support components | GMP 6.3.0, libuv 1.48.0, mimalloc 2.2.3, LLVM C++ runtime 22.1.4 |

The Lean archive is checked against SHA-256 `890afd185370f85666025b883914ab4f4b339136f8c96167b69cfb62aecaf235`. Upstream licenses and notices are retained in both proof asset bundles. The platform adapter and build recipe are MIT; upstream licenses remain applicable.

The signed advisory review uses the actual fetched Debian security tracker, CVE records and GitHub public advisory responses. GMP and libuv versions are beyond the identified affected ranges. The LLVM advisories inspected concern IR parsing or compiler features, not the retained libc++, libc++abi and libunwind components. The candidate Lean bundle excludes its unused compiler, static archives and utility libraries; the checker binary has no OpenSSL dependency. Empty published-advisory responses mean no entries in those sources at review time, not an assurance that vulnerabilities do not exist.

The native compiler inventory retains upstream findings with an explicit candidate-only disposition. This is allowed only under the attested split execution policy. There are no high/critical checker-domain exceptions; missing coverage fails admission. The final first-party root overlay changes no third-party package versions.

## Conformance and limits

The fixed corpus includes valid and changed Lean statements, added axioms, `sorry`, malformed/truncated certificates, valid and invalid DRAT certificates, statement mismatch, and missing, linked, oversized, large and late-writer build products. A separate native corpus checks isolation, C++, Rust, numerical libraries, paired timing and proof-gated paired timing.

Both signed hardware receipts passed on the exact final profile: 14 proof/product cases and six native cases. The [evidence index](proof-runtime-2026-09-08/evidence-index.json) links clean builds and contains the profile digest; the signed receipts and bound inventories are adjacent. Clean build success alone does not enable a runtime. Certificate text is streamed into the adapter, but Lean retains its parsed proof graph in memory; this is not constant-memory verification. Larger workloads need an explicitly provisioned resource envelope, reviewed assets and challenge-specific adversarial fixtures.

The first advisory bundle used the earlier base review generation time even though new proof sources had been fetched later. Service admission rejected this temporal inconsistency. The corrected signed bundle retains each real source fetch time and records the actual new generation time. The corrected profile repeated all 20 hardware cases, and the superseded enrollment is disabled. Prior receipts are preserved for their original profile; the evidence index identifies the active receipt pair.

The [public runtime release](https://github.com/matbalez/science-ladder/releases/tag/verification-952a8185ca40) includes the exact Lean and checker SquashFS bundles as well as the guest and runner. The public/hidden data assets there are fixed conformance canaries, not live private challenge data. Filesystem timestamps can make a fresh asset build differ in bytes; the commissioned bundles have exact recorded hashes.
