# Runner trust renewal

The currently commissioned verifier needs operator maintenance before its first
24-hour trust window expires. Renewal is not yet scheduled or automatic. The
public website and stored results remain available during a verification pause.
This deployment remains `controlled-demo`; renewal does not certify production
readiness or turn one physical host into independent replication.

## Observed deadlines

These dates were read from the deployed signed configuration and advisory on
4 September 2026. Certificate dates are from the provisioned public certificates.
Read the current files again before every operation; this table is an audit record,
not a live status display.

| Authority | Expiry in UTC | Vancouver time | Effect |
| --- | --- | --- | --- |
| Signed advisory snapshot | 2026-09-05 22:36:26.017165 | 5 September, 15:36:26 PDT | Fresh preflight scans fail as stale/unknown |
| Signed host attestation | 2026-09-05 22:49:34.304148 | 5 September, 15:49:34 PDT | New runs and preparations fail host authorization |
| Runner client and API server TLS certificates | 2026-12-03 22:11:10 | 3 December, 14:11:10 PST | New mTLS connections fail |
| Private runner TLS CA certificate | 2027-09-04 22:11:10 | 4 September, 15:11:10 PDT | Certificate-chain validation fails |

The worker stops accepting new work **20 minutes before** either signed trust
deadline: a 15-minute lease plus a five-minute transport/delivery margin. For the
recorded configuration this is **5 September, 22:16:26 UTC / 15:16:26 PDT**.
Start renewal before that admission deadline, allowing review and rollback time.
The current daemon
loads its configuration, trust maps and TLS client once: replacing files alone
does not renew the running process.

Advisory expiry is checked by the offline scanner during preflight. Already locked
submissions do not rerun that scanner on each submission; host attestation remains
an admission check for each new run. Expiry does not erase or retroactively change
historical signed receipts. A run admitted before host expiry can finish within
its separate job deadline.

## Current failure and recovery behavior

The daemon verifies and caches its signed host/config and advisory trust window,
then checks that window before every claim request. Near expiry it first finishes
the current attempt and result delivery, then exits successfully into maintenance
without claiming another job. The service's `Restart=on-failure` does not restart
this intentional stop. A manual restart with expired signed trust can replay
already signed results but cannot claim or execute new work. Signature, pin,
inventory and policy failures still fail closed; the guard grants no execution
authority and does not replace full host checks in Run/Prepare.

Before this guard, claiming work with expired host authorization could leave an
uncompleted lease. Lease recovery remains fenced and excludes the failing host;
other infrastructure faults can therefore still leave single-host work without
an eligible host. The API does not currently store either trust expiry. Renewal
and explicit infrastructure-failure recovery remain operator responsibilities.

There is no operator drain or recovery HTTP endpoint yet. Do not delete receipts,
clear exclusions indiscriminately, or relabel failed work as successful. A failed
preflight can use a new authorized preflight request after repair. An expired
leased submission needs explicit operator reconciliation with the backend's
fencing and acceptance-order rules before a fresh authorized attempt.

## Safe renewal sequence

1. Record the current signed config, advisory, public keys, file digests, host
   identity, profile, epoch and inventory row in the operator audit. Preserve the
   preceding records. Refresh primary advisory sources for the exact installed
   runtime and every approved vendored wheel, keeping fetched times, URLs and raw
   content hashes. Reapply exact binary/source-version matching and the reviewed
   applicability evidence. New, changed, unresolved, high or critical findings
   require review or remediation; do not inherit a previous clean verdict.

2. Validate the renewed snapshot with the real scanner, then have the authorized
   platform advisory signer sign its exact canonical document. The scanner accepts
   at most seven days between generation and expiry; sources must have been fetched
   no earlier than 24 hours before generation. The current operating window is
   24 hours. Neither a fresh signature nor a changed timestamp refreshes old source
   evidence. A review script that reuses cached source bytes is not a collector.
   `runnerd advisory-check --runtime-inventory FILE --snapshot UNSIGNED_FILE --out NEW_FILE`
   checks schema, exact coverage, provenance and policy; its output explicitly
   remains unsigned. Verify the resulting signed envelope separately with
   `sl receipt verify --receipt SIGNED_FILE --keys ADVISORY_PUBLIC_KEYS`.

3. Prevent new claims while retaining result delivery: in an audited operator
   transaction, save this host's allowed purposes and set `runner_hosts.purposes`
   to an empty array. Leave `enabled=true`, key, certificate fingerprint and group
   intact. The backend confirms that empty purposes prevent claim selection while
   retaining authentication for already leased results. Wait for running jobs to
   complete, all signed results to be accepted, the dedicated encrypted
   `spool/results` directory to be empty and the private work directory to be idle.
   Then stop `science-ladder-runner.service`. An ordinary stop cancels current work;
   the systemd stop timeout is not a graceful drain mechanism.

4. Recheck actual dedicated-host controls and pinned components. Update only the
   advisory file pin in the proposed host config when the guest runtime is unchanged.
   Compute the new binding with `runnerd config-digest --config PROPOSED_CONFIG`.
   The authorized platform host-attestation signer must bind that exact config,
   host identity, physical ownership/egress assertions, epoch and execution profile
   to a fresh bounded expiry. Keep the platform private signing key off the runner.
   Advisory renewal alone does not change the guest execution profile or scientific
   challenge locks. A runtime or guest-semantic change follows security migration
   rules instead of this renewal path.

5. Install the signed advisory and signed config atomically while the worker is
   stopped, with the same protected ownership and file permissions. Update the
   operator-approved `runner_hosts.advisory_snapshot_digest` to the exact signed
   envelope file digest. Keep `runtime_inventory_digest` unchanged if its bytes
   are unchanged; otherwise review the new inventory explicitly. Queued work that
   already binds the old scan-policy digest cannot silently switch: finish it
   within its valid window or create an explicitly reconciled fresh attempt.
   Independent preflight hosts must all use the same approved scan inputs.

6. Run `runnerd host-check` with the installed config and public trust map. This
   checks host authorization, actual controls and pins; it does not substitute for
   the advisory scan in step 2. Run the fixed hardware corpus under the deployed
   service restrictions after host-control or binary changes. Preserve its new
   signed receipt outside `spool/results`. Restore the saved allowed purposes and
   restart the worker to load the new config and certificates. Confirm service
   health, successful claim/result delivery and the next valid hosted preflight.

If validation fails, keep claim admission paused and report the reason. Rollback
is usable only while every restored trust record is still valid. Expired records
must not be made valid by changing local clocks, skipping checks or lengthening
their approved lifetime.

TLS renewal additionally updates the enrolled client certificate fingerprint,
restarts the runner's TLS client, and rolls the API's server certificate. Rotate
the CA with an explicit trust-overlap plan before its expiry. Retain signing-key
history separately: a certificate replacement is not permission to discard old
receipt verification keys.

## Smallest supported automation path

The existing code provides strict scanner, signature, binding, host-check,
hardware-probe and expiry-aware claim-admission primitives. It does not yet provide
a complete refresh collector, review-aware renewal command, scheduled signer or
drain API. Installing a timer that merely re-signs yesterday's snapshot is not a
proper maintenance implementation.

A bounded implementation should add:

- A reproducible primary-source collector and exact-inventory matcher that archives
  raw responses. Existing applicability decisions may be reused only while their
  exact package versions, relevant advisory assessment and supporting evidence
  remain unchanged. Changed or new security records stop automatic approval and
  produce a review request. Missing sources and stale coverage remain failures.
- A dedicated renewal signer outside the application and runner, using restricted
  managed-key authority. It may renew an unchanged approved host/config only after
  fresh authenticated control measurements. It must not allow a compromised host
  to grant itself tenancy, egress or new component authorization.
- A transactionally coordinated drain/install/inventory/restart operation with
  audit records and operator alerts before the existing admission guard stops
  claims. Existing result delivery must continue. Failure keeps admission paused
  rather than consuming and stranding jobs.

A small operator command that validates and installs an already reviewed, newly
signed snapshot/config bundle would reduce manual errors and is a reasonable next
step. It would not itself automate scientific/security review or primary-source
refresh. The admission guard is installed; automatic renewal and lifetime
extensions are not.
