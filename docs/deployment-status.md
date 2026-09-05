# Deployment status

Status recorded **4 September 2026**, after the creator's authenticated human
scientific-review approval and challenge publication.
Public site: [science-ladder.fly.dev](https://science-ladder.fly.dev).
Public MIT source: [matbalez/science-ladder](https://github.com/matbalez/science-ladder).

| Component or evidence | Current state |
| --- | --- |
| Web deployment | Published on Fly.io with prominent Participate instructions and the Quiet Echoes educational explorer |
| API deployment | Automatic unchanged-host authorization renewal and purpose-filtered claims; preserved-history researcher context; expired platform-policy leases retry with fresh fencing |
| Dedicated verifier | Running and connected to the API at the latest check; signed hardware isolation checks and the reviewed runtime advisory scan passed |
| Quiet Echoes hosted preflight 2 | Signed machine checks passed, including repeated fixture execution in fresh microVMs |
| Publication review | Actual human approval recorded separately from the preserved automated reviews; challenge version locked and published |
| Initial candidate attempts | Three public platform-verified attempts: energies 20,604; 26,964; and 25,544. Each has a primary and fresh-confirmation run |
| Submission queue | Six logical units configured for three outstanding submissions; each reserves primary and fresh-confirmation capacity |

[Quiet Echoes](https://science-ladder.fly.dev/challenges/quiet-echoes-labs512) is
published. All three actual seeded attempts are finalized and public; none beats
the published reference of 17,996, and no milestone was claimed. The deployment
uses single-host **platform verification**.
The protocol's independent-replication status requires a different physical host
group; physical redundancy alone does not establish scientific novelty. The deployment remains `controlled-demo` with
`officialAcceptance: false`; successful mechanical checks do not complete the
external security, key-custody and production release gates. See
[host evidence](security/host-commissioning-2026-09-04.md) and
[release gates](release-gates.md).

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
