# Independent audit witnesses

The witness observes the same signed checkpoint chain as other clients, verifies every event in each interval, and persists its countersignature before returning it. Its journal is held by its own operator. The root public key and genesis checkpoint digest must come from an independently verified protocol release; downloading a trust root from the service under examination is insufficient.

## Start an observer

Build the binary with `go build -o bin/sl-witness ./cmd/witness`. Supply an externally pinned root public key and the complete ordered array of root-signed key-history envelopes. The history identifies all three witness keys and their distinct administrators, with a 2-of-3 quorum and one-hour outage policy.

```sh
bin/sl-witness \
  --root-key /etc/science-ladder/root-public.pem \
  --root-id root-v1 \
  --history /etc/science-ladder/key-history.json \
  --key-id witness-operator-a-v1 \
  --kms-key YOUR_WITNESS_KMS_KEY_ARN \
  --kms-region YOUR_AWS_REGION \
  --journal /var/lib/science-ladder-witness/journal.ndjson \
  --platform https://science-ladder.fly.dev \
  --listen 127.0.0.1:8090
```

The optional platform observer fetches one checkpoint at a time, verifies the chain against its retained predecessor, and returns a signature for the exact checkpoint. It retries its last persisted signature before moving ahead, so restarts or lost HTTP responses cannot silently lose a witness receipt. Public peer endpoints can sit behind the operator's HTTPS proxy. There are no app/database credentials on a witness.

For an explicitly local demonstration, `--mode controlled-demo --key-file PATH` permits a private mode-600 P-256 PEM. This does not qualify as independently administered KMS custody. Root-signed delegation and all chain checks still apply.

The local HTTP service exposes:

- `GET /healthz`: process liveness.
- `GET /v1/checkpoints/latest`: latest retained checkpoint and this witness's signature.
- `POST /v1/checkpoints`: an `audit.Bundle` containing a platform envelope and the complete event interval. The witness verifies the pinned chain and returns `{ "envelope": ... }` only after durable retention.

The public platform exposes `/v1/audit/events`, `/v1/audit/checkpoints`, and the matching checkpoint witness-receipt endpoint. Each checkpoint's digest addresses its canonical payload rather than a randomized ECDSA envelope.

## Failure and recovery

The journal holds an exclusive process lock. A second process cannot use the same journal. Exact retries return the already retained signature; a competing successor, sequence gap, changed event, invalid signature, expired delegation, or altered Merkle root fails closed.

A failed journal write or sync pauses signing. A partial final record prevents restart. Preserve the damaged journal, compare it with independently retained checkpoints, and recover the previously committed prefix before resuming. Never discard the journal or reset to a new genesis to work around a fork. Running an independent copy from a verified backup is a recovery operation that must preserve the same chain.

When a key rotates or is revoked, publish a root-signed successor history retaining all previous delegations and explicit revocation times. Install the verified complete history and restart the observer. Old receipts remain verifiable in their original validity window. Effective revocations cannot be withdrawn by editing history.

Tests exercise malicious event changes, dropped sequences, fork attempts, repeated signatures, multiple claimed operators sharing one key, revocation boundaries, restarts, exclusive ownership, and interrupted journals. These are internal tests; external operators and a witnessed outage/restore drill are still required for official launch.
