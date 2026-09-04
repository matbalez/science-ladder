# Deployment status

Status recorded **4 September 2026**, while the creator's human scientific-review
decision is pending. Public site: [science-ladder.fly.dev](https://science-ladder.fly.dev).
Public MIT source: [matbalez/science-ladder](https://github.com/matbalez/science-ladder).

| Component or evidence | Current state |
| --- | --- |
| Web deployment | Revision `2c87` deployed |
| API deployment | Revision `49c0143` deployed; expired platform-policy leases can retry on the available host with fresh fencing |
| Dedicated verifier | Running; signed hardware isolation checks and the reviewed runtime advisory scan passed |
| Quiet Echoes hosted preflight 2 | Signed machine checks passed, including repeated fixture execution in fresh microVMs |
| Scientific review 2 | Requires an actual human decision; awaiting the creator's response |
| Initial candidate attempts | Three real prepared attempts are ready; none has been accepted yet |

Publication and candidate acceptance remain pending that review decision. These
are single-host **platform verification** results. Independent replication requires
a different physical host group. The deployment remains `controlled-demo` with
`officialAcceptance: false`; successful mechanical checks do not complete the
external security, key-custody and production release gates. See
[host evidence](security/host-commissioning-2026-09-04.md) and
[release gates](release-gates.md).

## Persistence

Fly Managed PostgreSQL 17 holds authoritative application state and receipt order.
A private Tigris content-addressed bucket holds immutable artifacts; snapshot
support is enabled. No authoritative state depends on a Fly Machine's ephemeral
disk. A complete application-level restore and audit-reconciliation drill is not
claimed; backups and snapshot configuration alone do not establish recovery.
See [persistence and recovery](persistence.md).

## Next operator deadline

The runner will stop new claims on **5 September 2026 at 15:16:26 PDT
(22:16:26 UTC)**, twenty minutes before its earliest signed-trust expiry. It will
finish existing result delivery and enter maintenance cleanly. Renewal is manual:
refresh and review advisory evidence, renew the signed host/config binding, and
drain/restart using the [renewal runbook](runner-renewal.md). No trust lifetime was
extended. The website and existing records remain available during this pause.
