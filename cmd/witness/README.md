# Independent checkpoint witness

The witness starts from an externally pinned root public key and the complete
root-signed key-history chain. Its local append-only journal is the durable fork
boundary. Retain that journal across restarts, host replacement, and key rotation.
Never delete or truncate it to recover from an error; reconcile with independently
retained history first.

Startup synchronizes the journal and its directory chain before allowing
observations. Each accepted record is synchronized before a signature is returned.
Storage that cannot honor these synchronization operations fails closed. A partial
journal tail or failed write/synchronization pauses signing.

A replacement key can catch up from the pinned genesis. The witness verifies the
platform signature, event chain, checkpoint predecessor, and Merkle root for each
historical checkpoint. If its key was not eligible at that checkpoint's issue time,
it retains an explicit unsigned historical observation. This advances its durable
cursor without claiming a retroactive vote. Once it reaches an eligible checkpoint,
it persists and returns its own signature. The HTTP observation endpoint responds
with `202 {"retained":true,"signed":false}` for unsigned catch-up, or a signed
`envelope` for an eligible vote. The latest-checkpoint endpoint has an empty
`witnesses` array for unsigned historical observations.

The continuous observer skips vote publication for unsigned history and retries
the exact persisted signature after a lost acknowledgement. Checkpoint-time
witness membership is reconstructed from the complete signed history so rotating
a witness key does not erase old quorums. Quorum requires two active, distinct
operator signatures out of three registered operators; an inactive third key
does not invalidate the other two votes.

Official mode requires a managed nonexportable signing key. Local PEM keys remain
restricted to explicit `controlled-demo` mode.
