# Deployment status

Status recorded **8 September 2026**.
Public site: [scienceladder.org](https://scienceladder.org).
Public MIT source: [matbalez/science-ladder](https://github.com/matbalez/science-ladder).

| Component or evidence | Current state |
| --- | --- |
| Web and API | Fly.io; typed measurements, scientific-metric review, runtime capability routing, public GitHub source import and versioned creator/solver guidance |
| Load Paths | Published fixed-material structural-design challenge with three geometries, nine load evaluations, 19 measurements and three hard gates |
| Load Paths verification | Real hosted preflight passed eight fresh-VM fixture runs; bound automated scientific review passed before lock/publication |
| Load Paths seeds | Three disclosed platform-agent entries finalized with fresh confirmation: 1.002570, 1.002844 and 1.014935 |
| Quiet Echoes | Original source, lock, three results and researcher context preserved |
| Dedicated verifier | One Scaleway physical server; separate immutable guest profiles, candidate/checker isolation and automatic unchanged-host authorization renewal |
| Proof execution | Additional enrolled profile; 14 signed proof/product cases and six native cases passed, including proof-gated timing. [Evidence](security/proof-runtime-review-2026-09-08.md) |
| Native execution | Linux C++/Rust and Python numerical programs, paired baseline timing with uncertainty, pinned read-only assets and profile-bound longer sessions |
| Apple/Metal | Represented by the execution contract; backend and MLX workload deliberately deferred |

[Load Paths](https://science-ladder.fly.dev/challenges/load-paths) challenges the
community to improve a reproducible 110-iteration SIMP reference. Its score is the
least relative improvement in worst-load compliance across three geometries,
subject to a fixed material allowance and equilibrium checks. The baseline is not
a claimed global optimum or literature record. All seed methods and limitations
are disclosed in the [solution repository](https://github.com/matbalez/science-ladder-load-paths-solutions).

[Quiet Echoes](https://science-ladder.fly.dev/challenges/quiet-echoes-labs512)
retains reference energy 17,996 and accepted seed energies 20,604, 26,964 and
25,544. None beats its reference; no record claim was added. Both challenges use
single-host **platform verification**. Fresh confirmation on that host is not
independent physical replication.

The deployment remains `controlled-demo` with `officialAcceptance: false`.
The published computational results are real; fixed first-party conformance is
not an independent security audit. See [release gates](release-gates.md).

## Post-publication presentation

Quiet Echoes now includes **Researchers to know**, populated through the editorial
form with three source-backed profiles. The text explicitly distinguishes research
relevance from sponsorship, endorsement or confirmed interest. No outreach was
sent. Editorial metadata has its own immutable history; a live comparison confirmed
the scientific contract, submitted artifacts, scores and existing receipt history
were unchanged.

The native solver workflow was tested on macOS using Python 3.14.5 with no
third-party packages or containers: baseline energy 17,996 and all 16 checker tests
passed against the exact frozen source. Solver instructions target macOS/Linux;
Windows support is deferred. Docker is available for optional pinned-runtime
checks, and hosted verification remains authoritative.

Scaleway's console still reports **Error**, with no explanation in its displayed
provider event log (latest event: 27 August). Direct checks found the runner and
host signer active, and the runner exchanging traffic with the API. Separate OS
failures concern cloud-init's absent datasource and a firmware-daemon/library
version mismatch; neither stopped verification. The provider label's cause is
unresolved. No provider-state reset, reinstall or support message was performed.

## Persistence

Fly Managed PostgreSQL 17 holds authoritative application state and receipt order.
A private Tigris content-addressed bucket holds immutable artifacts; snapshot
support is enabled. No authoritative state depends on a Fly Machine's ephemeral
disk. A complete application-level restore and audit-reconciliation drill is not
claimed; backups and snapshot configuration alone do not establish recovery.
See [persistence and recovery](persistence.md).

## Verification continuity

Host authorization now renews automatically on startup and six hours before each
24-hour lease expires. Temporary API failures retry without stopping a valid
lease; the worker recovers automatically if authorization has expired. The
previous daily all-job maintenance cutoff is removed. Existing challenge
verification continues after the original advisory snapshot expires.

Fresh advisory evidence remains required for admitting new checkers. The original
snapshot's preflight admission deadline is 5 September 2026 at 22:16:26 UTC;
this does not block submitted solutions to published challenges. TLS certificates
still require rotation before 3 December 2026. See the
[authorization and advisory runbook](runner-renewal.md).

## Interface simplification

The website now uses concise headings and removes promotional slogans, decorative
protocol/version labels and repeated explanations. Participate and the educational
challenge material remain visible. Creation separates researching an idea from
importing a candidate; the import view links to `/docs/candidate` with the exact
YAML requirements, complete Quiet Echoes examples and repository setup guidance.
A misplaced challenge manifest receives a specific error before import. Source
contracts and verification behavior are unchanged.
