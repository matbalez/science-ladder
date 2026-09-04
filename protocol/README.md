# Science Ladder protocol v1

All first-party protocol code, schemas, prompts and fixtures are MIT licensed.
External papers, datasets and submitted artifacts retain their declared licenses.

Semantic JSON uses RFC 8785. YAML is a strict authoring format and is never hashed
directly. Schemas reject extra fields; the Go semantic validator additionally checks
ordered milestone arithmetic, evidence, safety classification, argv, fixture coverage,
limits and exact score behavior. Decimal scores and all integer ticks are strings.

Receipts use DSSE PAE with `application/vnd.science-ladder.v1+json`, SHA-256 and ECDSA
P-256 DER signatures. The signer accepts `crypto.Signer` so a managed KMS implementation
can keep private material out of the service. A receipt signature alone is insufficient:
verify trusted key role/validity/delegation, object schema, expected immutable inputs,
job/host/fencing bindings and checkpoint history before treating it as authoritative.
Local development keys and local reports are never production authority.

Artifact identity is SHA-256 of `science-ladder-artifact-v1\0` plus the RFC 8785
`ScienceLadderArtifactTree` manifest. Entries are sorted NFC UTF-8 relative paths,
regular files, normalized mode `0644`, byte lengths and SHA-256 digests. The parser
never extracts archive paths to the host. Links, traversal, executable modes, active
content, unresolved LFS pointers, duplicate/case-colliding paths and resource bombs fail.

The hosted control plane independently resolves exact GitHub commits and constructs
the artifact. The CLI's digest is a preview, not an accepted submission. Competition
state must only use the immutable challenge lock digest and server acceptance sequence.

New manifests default to `verificationPolicy: platform`; new locks resolve and
freeze that value explicitly. Fresh-VM repeats on one enrolled physical host may
support `platform_verified`. `independently_replicated` is separate evidence and
must never be inferred from repeats on that same host. Authors may choose the
stricter `independent` policy. Historical locks or jobs without the field retain
their earlier independent-host requirement; parsers preserve omitted fields to
avoid changing old canonical digests. A raw execution receipt alone does not award
either assurance status. All policies keep the advisory, isolation and review gates.

Scout prompt 1.1.0 documents this distinction and source publication metadata.
Candidates produced under 1.0.0 remain readable with their original provenance.

The only economic mode is `none`; no wallet, reward, payout or billing protocol is shipped.
See the versioned [Scout prompt](../prompts/challenge-scout-v1.md) and `sl --help`.
