# Local-first frontier admission

Solver search and correctness checks run on the solver's computer. Only candidates
claiming a meaningful improvement over the current public frontier enter hosted
preparation and verification. The frozen reference is the lower bound on this
comparison: a weaker public result never makes admission easier. Metric direction,
quantum, bounds and minimum delta come from the immutable manifest; comparisons use
exact integer ticks. Private results are not consulted by admission.

## Solver flow

1. Reproduce the reference and run the documented local search loop.
2. Run `sl claim` against the final artifact with an explicit local checker command.
   It fetches the frozen manifest and frontier metadata, compares the local manifest,
   hashes the artifact before and after execution and rejects changes. The checker
   must succeed and emit the exact `ValidatorResult`, or a current CLI
   `LocalValidationReport` containing it. Every hard gate must pass. A baseline,
   weaker result or insufficient improvement produces no claim file.
3. Publish the artifact-only repository at an exact Git commit. `sl submit` requires
   `--claim`, `--artifact` and the frozen `--manifest`; it recomputes the artifact digest.
4. The CLI posts small claim metadata to `/v1/frontier-claims`. The server validates
   the report structure, typed measurements and gates, comparison identity, lock
   binding and claimed improvement. It issues a 15-minute ticket bound to the account,
   version, repository, commit and artifact digest. No repository is fetched here.
5. `/v1/submission-intents` requires the ticket and `previewDigest`. The transaction
   rechecks the public frontier, consumes the ticket once, reserves preparation and
   enqueues the source fetch. Failure rolls back all reservations. Independently
   reconstructed GitHub bytes must match the claim before storage or isolated disk
   preparation. Existing capacity and validation-grant checks still apply at acceptance.
6. Official isolated checking and fresh confirmation decide correctness and score.
   The local claim never supplies an authoritative result. The acceptance receipt
   records the claim digest, admission mode and public frontier snapshot.

Example, from a challenge checkout with a native `local.py` driver:

```sh
mkdir -p .local
SL_FINAL_RUN="$(mktemp -d "$PWD/.local/frontier.XXXXXX")"
sl claim --api https://science-ladder.fly.dev --version VERSION_ID \
  --manifest science-ladder.yaml --artifact ../candidate-artifact \
  --out "$SL_FINAL_RUN/claim.json" --report "$SL_FINAL_RUN/result.json" \
  -- python3 local.py --solver ../candidate-artifact/solver.py \
  --output "$SL_FINAL_RUN/result.json"
# Only after the claim succeeds, publish the same candidate and use its real SHA.
sl submit --api https://science-ladder.fly.dev --version VERSION_ID \
  --repository OWNER/REPO --commit FULL_COMMIT_SHA --license MIT \
  --manifest science-ladder.yaml --artifact ../candidate-artifact \
  --claim "$SL_FINAL_RUN/claim.json"
```

Use the actual checker documented for the pinned challenge. `sl claim` executes the
explicit command with the user's local permissions, without a shell unless the user
explicitly selects one. It permits at most 30 minutes and 64 KiB of report data.
Legacy native drivers that raise on scientific invalidity and write only
`score` and `measurements` can use `--checked-gates name1,name2,...`. List exactly
the frozen gates actually checked by that driver. After successful execution the
CLI supplies version/comparison metadata and those explicit gate assertions, then
applies the same full report validation. This is a local attestation, not a server
certification. The pinned One Less Multiply and Smallest Triangle Participate
instructions include their corresponding checked gates. New creators should emit
the complete ValidatorResult directly.

Report and claim paths must be fresh and outside the artifact. To capture a container
check, omit `--report` and supply `-- sl validate --local --unsafe-local ...`; its JSON
stdout includes the complete result. Docker Desktop is not required for native checks.

## Hidden tests and hardware measurements

The server derives `qualification` mode only from a hidden suite or performance
contract. A client cannot opt an ordinary public artifact, proof or program challenge
into it. Run the documented public correctness/qualification command with `sl claim`,
supplying `--estimated-score-ticks` and `--measurement-reason`. The bounded report's
digest, successful exit, estimate and rationale become the claim. The estimate must
be in-domain and frontier-potential. It is explicitly untrusted; it neither asserts
hidden-test success nor substitutes client timings for controlled paired measurement.
Scientific acceptance still requires the full frozen suite on the designated executor.
This preserves architectural support for hardware-specific challenges without claiming
that an unavailable hardware profile is commissioned.

## Abuse limits and retries

- Invited accounts and remaining validation quota are required. At most 5 new tickets
  per account per rolling 24 hours, including at most 2 qualification tickets.
- At most 3 live unconsumed tickets per account, and no more than remaining quota.
- At most 100 new tickets globally per rolling 24 hours. Issuance is serialized in
  PostgreSQL across API machines. Expired tickets still count toward the daily budget.
- Existing preparation budgets, active submission limits, bounded isolated execution,
  duplicate artifact checks and two-run capacity reservations remain enforced.
- Repeated identical live claims return the same ticket. An identical consumed claim
  can only resume its existing intent. Changed commits cannot reuse a ticket.
- Use `sl resume --api URL --intent ID` after interruption; it polls and accepts the
  same intent, without requesting more compute. Failed preparations retain their
  recorded findings; operational retries use existing jobs and leases.
- Once consumed, admission uses its snapshot through preparation/acceptance. A later
  racing frontier improvement does not cancel already admitted work or imply abuse.
  Final adjudication still compares verified results in the established order.

A dishonest client can fabricate a report or estimate. Hashes bind bytes and reports;
they cannot prove that a client executed the checker. This design reduces wasted work
and bounds each invited account's ability to consume compute; it is not a cryptographic
proof of computational work or an absolute defense against malicious accounts. Admission
limits are currently conservative platform constants. Request floods that fail before
ticket issuance still need the hosting layer's normal traffic controls.

## Rollout and compatibility

New solver intents require tickets. Existing intents are allowed to finish, and all
previous submissions, scores, locks and signed receipts retain their meaning. Older
CLIs receive `frontier_claim_required`; upgrade using the Participate instructions.
Creator adoption, baseline fixtures, conformance preflight and operational health
checks keep their separate budgets and do not require improving the frontier.
The claim is a separate admission protocol; it does not change frozen v1/v2 scientific
contracts or require a second physical verification server.
