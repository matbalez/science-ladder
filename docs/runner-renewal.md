# Runner authorization and advisory renewal

The worker automatically renews permission to execute on its unchanged approved
host. Existing locked challenges do not stop every day when an advisory snapshot
expires. Fresh advisory evidence remains required when admitting a new checker.
This deployment uses the existing platform signer and single-host platform
verification; renewal is not a new hardware or vulnerability assessment.

## Automatic authorization

An operator enrolls the exact commissioned `HostAttestation` template and its
configuration digest in `runner_authorization_enrollments`, with an approval
reason. The template is immutable; an operator can disable it. The worker cannot
supply new physical-tenancy, egress, profile or inventory claims.

On startup, and thereafter when six hours or less remain, the daemon calls
`POST /internal/v1/runner/authorization/renew` over the dedicated mTLS connection.
The API checks the active host, certificate delegation, exact configuration
and enabled enrollment. It signs the unchanged template with a new 24-hour
expiry and records append-only issuance evidence. The daemon verifies the
signature, unchanged claims and pinned files, then adopts the lease in memory.
The platform private key stays off the verification server. On-disk commissioned
evidence and advisory timestamps remain unchanged.

Before each claim, at least twenty minutes of host authorization must remain:
a fifteen-minute job lease plus a five-minute delivery margin. Failed renewal
retains a still-valid lease and retries at most once a minute. If authorization
expires, the daemon waits without consuming jobs and recovers automatically when
renewal succeeds. Already signed results are replayed before new claims. Actual
host controls and pins are checked on startup and renewal and independently
before every execution. Invalid controls or signatures still prevent execution.

Revoking a host prevents authenticated claims; disabling its renewal enrollment
prevents new leases. Disabling only renewal does not retract an already issued
lease. For immediate removal, disable the host as well.

## Advisory freshness applies to new checkers

The worker requests only eligible purposes. `artifact_prepare`, `submission`
and `confirmation` remain eligible under valid host authorization after the
advisory expires. `preflight` is omitted once less than twenty minutes remain on
the signed advisory. The API intersects requested purposes with the host's
existing permissions; a request can never broaden enrollment. The preflight
scanner independently rejects stale or incomplete evidence.

Existing challenges retain their exact locked checker, runtime and scientific
contract. Historical advisory evidence is verified at its recorded generation
time; this is not a claim that it covers newly discovered vulnerabilities.
A new relevant security finding requires operator assessment and, if needed,
revocation or a reviewed runtime migration. New checker admission requires the
fresh-source procedure below; automatic source collection is not implemented.

Expiry does not erase or rewrite historical receipts. Expired platform-policy
job leases may retry on the same host with fresh fencing, subject to the existing
eight-attempt limit. Explicit infrastructure-fault exclusions and independent
verification policies retain their restrictions; those faults can require
operator reconciliation. Never delete receipts or relabel failed work successful.

## Other credential lifetimes

The original signed advisory expires on **5 September 2026 at 22:36:26 UTC**.
With its admission margin, new checker preflights need refreshed evidence after
22:16:26 UTC. This does not pause solutions for published challenges. The original
on-disk host attestation expires at 22:49:34 UTC that day; the daemon now obtains
its own renewable authorization instead of stopping at that date.

The commissioned runner client and API server TLS certificates expire on
**3 December 2026 at 22:11:10 UTC**, and their CA on **4 September 2027 at
22:11:10 UTC**. TLS rotation is separate and not automated by this change. Read
current certificates and the latest `runner_authorization_renewals` rows for
current operational status; these dates document the original commissioning.

## Installing fresh advisory evidence or a changed configuration

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
   to a fresh bounded expiry. Enroll that exact approved template in
   `runner_authorization_enrollments` and disable the superseded enrollment.
   Keep the platform private signing key off the runner.
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
