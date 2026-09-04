# Science Ladder — Technical Architecture and Build Plan

> Historical specification. The approved [implementation decisions](../decisions.md) supersede its hosting, licensing, and mandatory cross-host verification recommendations. New MVP challenges default to single-host platform verification; independent replication is a separate status and optional locked policy. Existing immutable contracts retain their original meaning.

**Version:** 0.2

**Date:** September 4, 2026

**Status:** Proposed for founder approval

**Companion document:** *Science Ladder — Product Requirements Document v0.2*

---

## 1. Executive recommendation

Build Science Ladder as two things from the beginning:

1. an open, portable protocol for defining, running, scoring, exporting, and independently verifying scientific challenges, with a versioned extension point for future rewards; and
2. a hosted reference implementation that makes the protocol easy to use.

The first release should be a deliberately compact system:

- a **Go modular monolith** for the API, background jobs, protocol logic, CLI, and local runner;
- a **Next.js/TypeScript web application** for challenge discovery, creation, submission status, and editorial operations;
- **PostgreSQL** as the transactional source of truth and job queue;
- an **S3-compatible content-addressed object store** for challenge snapshots, submissions, suites, receipts, and logs;
- an **OCI registry** for locked validator images;
- a physically isolated **Firecracker validation plane** for executing creator-supplied validators;
- free, subject-bound **validation grants** for invitation-only capacity control; and
- signed, exportable protocol receipts for every consequential transition.

The payment-free MVP should support **`artifact-checker-v1` only**. Solvers submit data artifacts—proofs, schedules, molecules, datasets, parameter files, circuits, or other structured files—and Science Ladder never intentionally launches them as programs. The whole validator guest is still treated as hostile because a buggy or malicious checker can interpret those bytes or contain parser vulnerabilities. Executing submitted solver programs while keeping hidden tests secret requires two isolated machines and a constrained broker; that is a public-beta feature, not a safe MVP shortcut.

The architecture is intentionally not a fleet of microservices. Most MVP behavior belongs in one deployable control plane with clear internal modules. Separate MVP processes exist only where the hostile-code boundary warrants them. The future payment design preserves additional resolver, relay, and wallet-secret boundaries, but none is deployed or credentialed in the MVP.

The recommended delivery target is a **12–14 week code-complete, security-approved, payment-free MVP candidate with four experienced engineers**, followed by an outcome-driven invitation-only pilot. A two-engineer team should expect roughly 20–26 weeks for the same integrity bar. A controlled hosted demonstration should arrive in weeks 6–8. This moves the hardened candidate forward by roughly four to six weeks versus the payment-inclusive plan; the secure validation runner, not checkout or wallet work, remains the critical path. After the hardened candidate exists, Bitcoin reward engineering may begin as a 5–7 week fast-follow and Stripe/Link billing as an independently schedulable 2–3 week fast-follow. Both can proceed alongside the invitation pilot, but no monetary capability activates before pilot exit and its own interoperability, security, legal, and processor gates pass.

The critical path is:

```text
protocol contracts
→ canonical artifacts
→ locked validator build
→ isolated official execution
→ ordered adjudication
→ immutable milestone claim
→ public frontier publication
```

Discovery pages, the Challenge Scout, creator UX, and editorial tools can be built in parallel, but none should obscure this core proof.

---

## 2. Architectural goals and non-goals

### Goals

The architecture must make the following properties true by construction:

1. **Challenge immutability:** the scientific and competition contract cannot change after publication.
2. **Objective outcomes:** milestone decisions derive only from a locked validator, fixed inputs, typed results, and deterministic comparison rules.
3. **Fair ordering:** the earliest fully accepted qualifying submission claims a milestone, independent of worker speed or queue order.
4. **Unique milestones:** every milestone can be claimed once and only by the earliest qualifying receipt.
5. **Explicit economics:** every MVP lock and receipt says `economicMode: none`; runtime configuration cannot create a retroactive monetary obligation.
6. **Exactly-once competition outcome:** retries, crashes, and duplicate jobs cannot create a second claim or frontier event.
7. **Hostile-code isolation:** no creator validator or solver-controlled byte can reach platform credentials, cloud metadata, another run, or the public network.
8. **Private competition:** pending and non-winning submissions can remain private; milestone-winning public-frontier artifacts become public immediately under their predeclared license.
9. **Independent auditability:** a third party can verify signatures, receipt ordering, milestone decisions, and frontier state, and can recompute scores whenever the artifact and suite are public or later revealed. Private or hidden bytes necessarily limit immediate recomputation.
10. **Permissionless creation:** a new eligible challenge can be published without a Science Ladder engineer changing application code.
11. **Extensibility:** the same candidate and challenge contracts can later support automated literature discovery, independent validators, and real-world oracle challenges.

### Non-goals for the payment-free MVP

- Trustless consensus or on-chain adjudication.
- Arbitrary submitted programs.
- GPUs, stochastic metrics, wall-clock performance metrics, or heterogeneous architectures.
- LLM-as-judge competition decisions.
- Arbitrary creator Dockerfiles or unrestricted build scripts.
- Reward amounts, monetary entitlements, payout destinations, NWC wallet connections, payout execution, creator defaults, or payment-reliability scores.
- Stripe/Link checkout, Bitcoin service billing, refunds, chargebacks, or paid validation.
- Team payout splitting, pooled funders, transferable platform credits, escrow, custody, or exchange services.
- Kubernetes, Kafka, a service mesh, a separate search cluster, or event sourcing.
- Fully automatic publication of agent-generated challenges.
- Wet-lab or other real-world claims.

The MVP is **auditable, not trustless**. The hosted platform remains trusted to assign receipt order and operate official runners. Content addressing, signed receipts, reproducible packages, and append-only checkpoints make that trust inspectable and create a path to independent validators later.

---

## 3. Architecture decisions

| ID | Decision | Rationale |
|---|---|---|
| ADR-001 | Open protocol plus hosted reference implementation | Prevents platform lock-in while preserving a polished product. |
| ADR-002 | Go for the control plane, workers, CLI, protocol library, and runner | One strongly typed implementation can be reused across local and official execution, receipt verification, and backend services. Static binaries simplify hardened deployment. A future NWC executor chooses its implementation separately after an interoperability spike. |
| ADR-003 | Next.js/TypeScript for the web application | Fast product iteration, strong ecosystem for account and dashboard UX, and no need to force browser-facing work into Go. |
| ADR-004 | Modular monolith for the control plane | Preserves transactional integrity and development speed; module boundaries can become services only when load or trust requires it. |
| ADR-005 | PostgreSQL is the state authority and initial job queue | Milestone claims, receipt order, validation grants, and outbox events need strong transactions. A Postgres-native job system avoids premature Kafka/Redis/Temporal infrastructure. |
| ADR-006 | Content-addressed storage and OCI digests | Every decision binds immutable bytes rather than mutable branches, tags, or URLs. |
| ADR-007 | REST/OpenAPI plus server-sent events | Simple public API and CLI generation; SSE covers status updates without WebSocket state machinery. |
| ADR-008 | `artifact-checker-v1` is the only MVP profile | Science Ladder never intentionally executes the submitted artifact, and one disposable microVM contains the jointly hostile validator/artifact interaction and hidden suite. |
| ADR-009 | Firecracker microVM per official run | Stronger boundary than an ordinary container for hostile challenge code, with explicit jailer, seccomp, cgroup, and KVM controls. |
| ADR-010 | Receipt sequence, not completion time, decides winners | Removes worker speed, retries, and queue scheduling from competitive ordering. |
| ADR-011 | Scores become integer ticks before adjudication | Binary floats, NaN, infinities, and ambiguous tolerance must never affect competition outcomes. |
| ADR-012 | Milestone adjudication emits a stable, provider-neutral extension event | Future reward programs may consume a final scientific outcome, but payment or billing code can never reevaluate it. |
| ADR-013 | Signed protocol envelopes and an append-only audit log | Allows offline verification and makes silent mutation or deletion detectable. |
| ADR-014 | AWS-first hosted deployment, Docker Compose locally, no Kubernetes initially | One well-understood production topology is enough for the MVP; the protocol remains cloud-neutral. |
| ADR-015 | One Linux `amd64` execution profile in MVP | Removes architecture and CPU variation from competitive results. New hardware becomes a new signed profile. |
| ADR-016 | MVP validation is free and controlled by subject-bound grants | Preserves resource admission and the future checkout seam without building a billing system. |

### Recommended implementation stack

| Concern | Choice |
|---|---|
| Control plane and workers | Go, standard library-first, explicit domain packages |
| Web | Next.js, React, TypeScript |
| API contract | OpenAPI 3.1 generated from or checked against hand-reviewed definitions |
| Object schemas | JSON Schema 2020-12; strict YAML only as the authoring representation |
| Database | PostgreSQL with `NUMERIC`/integer score ticks, row locks, unique constraints, and transactional outbox |
| Jobs | River or an equivalent Postgres-native Go queue; job payloads carry IDs, not authoritative state |
| Object storage | S3 in production; MinIO-compatible storage in development |
| Validator images | OCI images pinned by digest; ECR or another compatible registry |
| Official isolation | Firecracker, Jailer, KVM, cgroups, seccomp, immutable block devices, no guest NIC |
| Signing | Cloud KMS asymmetric keys; DSSE-wrapped in-toto statements |
| Authentication | GitHub App user identity and installation permissions; short-lived web sessions and scoped CLI tokens |
| Milestones | Go adjudication package; immutable `MilestoneTier` and `MilestoneClaim` records |
| MVP resource access | Free subject-bound `ValidationGrant` quotas; no checkout provider |
| Post-MVP rewards, not deployed | NWC-08 app-initiated connections; NWC-321 `pay`; BIP-321 instructions; BOLT 12 offers; BIP-353 names after its provenance gap is resolved; Lightning-address/BOLT 11 compatibility |
| Post-MVP service billing, not deployed | Stripe Checkout/Link producing the same subject-bound `ValidationGrant`; no Bitcoin billing provider selected |
| Observability | OpenTelemetry traces/metrics, structured logs, immutable audit events, alerting |
| Infrastructure | OpenTofu, AWS ECS/Fargate for the control plane, dedicated KVM-capable hosts for runners |

Dependencies should be pinned in lockfiles and updated deliberately. Protocol behavior belongs in Science Ladder code and conformance tests, not in framework defaults.

---

## 4. System topology

```text
                                    PUBLIC / TRUSTED CLIENTS
             ┌──────────────────────┐                 ┌──────────────────────┐
             │ Next.js web client   │                 │ Open CLI / agent     │
             │ creator + solver UX  │                 │ Git-native workflow  │
             └──────────┬───────────┘                 └──────────┬───────────┘
                        │ HTTPS                                      │ HTTPS
                        └──────────────────┬─────────────────────────┘
                                           ▼
                                ┌─────────────────────┐
                                │ CDN / WAF / API edge│
                                └──────────┬──────────┘
                                           ▼
     GitHub ◄───────────────┌────────────────────────────────────────────────┐
     LLM adapter ◄──────────│          CONTROL PLANE (trusted)               │
     paper sources ◄────────│  web · API · worker · editor · protocol logic │
                            └──────┬────────────┬─────────────┬──────────────┘
                                   │            │             │
                                   ▼            ▼             ▼
                             PostgreSQL    Object store    OCI registry
                           state + jobs    CAS + receipts  validator images
                                   │
                    signed jobs / │ narrow result API / mTLS, outbound only
                                   ▼
        ┌──────────────────────────────────────────────────────────────────┐
        │ VALIDATION ACCOUNT / NETWORK                                    │
        │ runner gateway → dedicated host → runnerd → one Firecracker VM │
        │ no control DB · no public guest network · no external secrets   │
        └──────────────────────────────────────────────────────────────────┘
```

The MVP deployment contains no payment processor, wallet connection, payment destination resolver, relay proxy, payout worker, or payment credential. Section 11 defines the post-MVP overlay and its separate trust boundaries.

### The three MVP security zones

1. **Public edge:** browsers, CLI clients, GitHub webhooks, papers, and repositories are untrusted input.
2. **Control plane:** owns identities, challenge state, receipt sequences, review decisions, frontier and milestone state, validation grants, and audit events. It never executes challenge code.
3. **Validation plane:** can read one signed job and its content-addressed inputs, then return one signed typed result. It has no database or general platform-secret access.

### Deployable units

| Unit | Responsibilities | Explicitly cannot do |
|---|---|---|
| `web` | Server-rendered discovery and account UI; authenticated forms | Decide scores, milestones, or frontier outcomes |
| `api` | REST API, auth, GitHub webhooks, transactions, signed public reads | Run validators |
| `worker` | Review orchestration, source fetch, state transitions, outbox/jobs, publication | Execute hostile code directly |
| `runner-gateway` | Pull signed validation jobs, stage verified inputs, select hosts | Access wallets or mutate adjudication state |
| `runnerd` | Create and destroy microVMs, enforce resources, sign run receipts | Make milestone or frontier decisions |

`api` and `worker` are separate processes built from the same modular monolith. This improves scaling and failure isolation without creating distributed ownership of domain state.

---

## 5. Trust model and invariants

| Component | Trusted for | Not trusted for |
|---|---|---|
| Control plane and PostgreSQL | Acceptance order, challenge state, adjudication, public records | Executing challenge code |
| Creator | Scientific question, licenses, validator semantics, and milestone rationale | Platform security or unbiased challenge design |
| Creator validator | Producing measurements under its locked version | Claims about its own safety or determinism |
| Solver artifact | Nothing; treated as hostile bytes | Claimed score, type, path, or metadata |
| Official runner host | Correct isolated execution and hidden-suite handling | Frontier or milestone authority |
| Local runner | Developer feedback | Official scores or milestone claims |
| Milestone engine | Deterministic threshold claims and frontier transitions | Reinterpreting validation or making network calls |

### Non-negotiable invariants

These are encoded as database constraints, property tests, and—in the concurrency-sensitive cases—a small TLA+/PlusCal model.

- A challenge version has exactly one immutable lock digest.
- An artifact digest is accepted at most once per challenge version.
- Receipt sequences are monotonic and gap-free for accepted submissions.
- A later receipt cannot adjudicate ahead of an unresolved earlier receipt.
- A milestone has at most one claim.
- Only one intake-open competition season for a challenge accepts submissions in the MVP.
- Every milestone claim belongs to the earliest qualifying receipt.
- One submission may atomically claim every still-open milestone it crosses.
- Every MVP challenge lock and acceptance receipt binds `economicMode: none`.
- No runtime flag can create a monetary obligation for an earlier payment-free receipt.
- An official result always binds the exact challenge lock, artifact, suite, semantic execution profile, validator image/disk, runner policy, and runner implementation epoch digests.
- Only an independently confirmed deterministic result can create a milestone claim or advance the public frontier.
- Milestone-winning public-frontier solution bytes become public; non-winning solution bytes remain private unless the solver elects to publish.

---

## 6. Open protocol and immutable objects

The protocol is the product's durable core. The hosted application is one implementation.

### 6.1 Versioned protocol objects

| Object | Purpose |
|---|---|
| `ChallengeCandidate` | Evidence-linked draft emitted by the Challenge Scout or, later, the scientific-web crawler |
| `ChallengeManifest` | Human-authored scientific, task, evaluation, milestone, economic-mode, and submission contract |
| `MachineConformanceReceipt` | Signed preflight build, fixture, determinism, and hostile-corpus result before scientific/editorial review |
| `ChallengeLockReceipt` | Signed immutable package after remote preflight |
| `SubmissionBundle` | Canonical solver artifact plus Git provenance and declared attribution/license |
| `SubmissionAcceptanceReceipt` | Signed competitive sequence assignment and frozen evaluation context |
| `ValidationJob` | Signed, narrow instructions to the isolated runner |
| `ValidationRunReceipt` | Signed measurements, gate outcomes, resource usage, and environment bindings |
| `AdjudicationReceipt` | Ordered frontier and milestone decision |
| `MilestoneClaim` | Irrevocable first-to-threshold scientific claim with no monetary meaning in the MVP |
| `AuditEvent` | Append-only state transition referencing the relevant object digests |

All objects carry `apiVersion`, `kind`, stable ID, creation time, producer identity, and content digest. Unknown required fields fail closed. Minor additive fields can be ignored only when the schema marks them as non-semantic. The MVP reserves stable extension points and the future object-kind names `RewardProgram`, `FundingAuthorization`, `PayoutDestination`, `RewardEntitlement`, `PaymentCommand`, `PaymentReceipt`, and `CreatorDefault`. Their schemas, tables, routes, and services ship only in the payment fast-follow.

### 6.2 Authoring and canonicalization

Creators author `science-ladder.yaml`, but YAML is never hashed directly. The parser must:

- disable aliases, anchors, custom tags, duplicate keys, implicit timestamps, and non-finite numbers;
- impose depth, key-count, string-size, and total-document limits;
- validate against the exact JSON Schema version; and
- convert to a typed JSON value.

Semantic objects use RFC 8785 canonical JSON before hashing. Signed receipts use DSSE envelopes carrying in-toto statements so a signature also binds the payload type. Every digest is algorithm-qualified, for example `sha256:<hex>`.

The lock process distinguishes:

- `manifest_digest`: the creator-authored semantic contract;
- `source_snapshot_digest`: the immutable repository tree;
- `validator_image_digest`: the built OCI artifact;
- `validator_disk_digest`: the final read-only runtime filesystem produced in quarantine;
- `suite_digest` or hidden-suite commitment;
- `execution_profile_digest`: guest-visible architecture, kernel, rootfs, CPU features, runtime configuration, limits, and policy; and
- `challenge_lock_digest`: the canonical digest over all of the above plus `economicMode`, the milestone schedule, and immutable review-report digests.

Mutable labels and operations—Featured, Human-reviewed, moderation status, and creator reputation—are excluded from the lock. They are separately signed events that reference the unchanged lock digest. Future funding and payout state is also operational and never changes the scientific contract.

Only `challenge_lock_digest` is used for official submission acceptance and milestone adjudication.

### 6.3 Canonical solver artifacts

GitHub is provenance and the required MVP source, but a Git commit hash alone is not the scored object. The solver pushes a clean full SHA to a public or private GitHub repository on which the Science Ladder App has read access. The server independently fetches that exact commit and constructs a canonical tree from only the challenge's allowed paths; the CLI performs the same process locally as a preview:

- sorted UTF-8 NFC paths;
- normalized file modes, timestamps, owners, and archive headers;
- explicit MIME/type contract;
- streaming compressed and expanded size limits;
- file-count, path-length, and decompression-ratio limits;
- no symlinks, hardlinks, device files, path traversal, submodules, case-colliding names, or unresolved Git LFS pointers; and
- an RFC 8785 manifest with `kind: ScienceLadderArtifactTree`, `version: 1`, and entries containing each normalized path, entry type, mode, byte length, and per-file SHA-256 digest; and
- an artifact digest computed as `SHA-256("science-ladder-artifact-v1\0" || canonical_manifest_bytes)`.

The submission records the verified GitHub repository/database ID, full remotely resolved commit SHA, parent public-frontier digest, author identity, and tool version. Official identity is the server-produced canonical artifact digest. A quarantine worker also constructs a platform-normalized read-only submission filesystem and records `submission_disk_digest`; this derived disk is what the official guest mounts, while competition identity remains the semantic artifact digest. A solver does not have to expose a losing branch publicly, but self-attested or unpushed commits are not accepted in the MVP.

### 6.4 Score representation

The validator writes a finite canonical decimal string matching `[+-]?(0|[1-9][0-9]*)(\.[0-9]+)?`; exponent notation, negative zero, leading zeroes, NaN, and infinities are forbidden. Version 1 allows at most 100 significant digits and 50 fractional digits, and the manifest declares a strictly positive exactly representable quantum within those bounds. The trusted normalizer parses with arbitrary-precision decimal arithmetic and converts it to integer ticks using mathematical floor/ceiling, including for negative values:

- maximize: round down, then compare `score_ticks >= threshold_ticks`;
- minimize: round up, then compare `score_ticks <= threshold_ticks`.

This rounds ambiguity against the solver. Thresholds, baselines, minimum meaningful deltas, tolerance, records, and milestone decisions use integer ticks only. Binary floating-point is never used in adjudication. Missing gates, unknown gates, duplicate keys, values outside the declared domain, NaN, infinity, or overflow fail closed.

The MVP stores no reward amount. A future `RewardProgram` stores bounded integer Bitcoin base-unit amounts separately from score ticks and binds them to milestone IDs. Its NWC-321 adapter derives the protocol wire amount with divisibility and overflow checks; routing fees are tracked separately and never deducted from a published reward. User-facing Bitcoin amounts use the ₿-prefixed integer convention.

### 6.5 Signing hierarchy and public audit checkpoints

Protocol receipts use DSSE with ECDSA P-256/SHA-256, a signature suite available in the selected cloud KMS and broadly verifiable offline. Future Nostr/NWC integration uses its protocol-required secp256k1 keys; those remain unrelated to the Science Ladder receipt trust root.

The hierarchy is:

1. a manually controlled KMS-backed platform root signs only key-delegation, revocation, and key-history objects;
2. a separate online KMS receipt key countersigns committed control-plane receipts within a narrow policy;
3. each runner host has a revocable hardware- or workload-bound host key and root-signed delegation limited to validation-run statements; and
4. host-signed run receipts become authoritative only after the control plane matches them to an accepted job and adds its countersignature.

`/.well-known/science-ladder-keys.json` is a content-addressed, root-signed history containing algorithms, public keys, roles, validity intervals, delegation limits, rotations, and revocations. It is mirrored in tagged GitHub releases and the public artifact archive. Revocation has an explicit effective time; the verifier rejects out-of-window signatures without rewriting valid history from before a compromise.

The append-only audit sequence is hash-chained and periodically summarized into a signed Merkle checkpoint containing the covered sequence interval, prior checkpoint digest, tree root, and timestamp. Science Ladder ships a small open witness binary and recruits three independently administered operators; production requires a 2-of-3 countersignature quorum. Checkpoints are emitted at a fixed event/time cadence. A missed quorum is public immediately; after the configured one-hour MVP grace period, new competitive acceptance pauses while existing receipts continue to resolution and the history remains exportable. Witness recovery signs the same checkpoint chain—never a replacement fork.

The root fingerprint, genesis checkpoint, witness public keys, quorum, and outage policy ship in the signed protocol release and key-history object. Verifiers reject gaps, broken prior-root links, invalid delegation, inadequate witness quorum, and two different roots for the same sequence interval. Clients cache/gossip recent checkpoints so an operator cannot quietly show inconsistent histories to different audiences. A root stored only beside the same database is not treated as an external witness.

---

## 7. GitHub and identity architecture

### 7.1 Identity

- GitHub is the required public identity provider for creators and solvers in the MVP.
- The GitHub App requests only the repository metadata/content permissions needed for installed challenge repositories.
- Web sessions use secure, HTTP-only, same-site cookies with rotation and short expiry.
- The CLI opens a one-time browser authorization flow and receives a revocable, scoped token stored in the operating system credential store.
- Organization affiliation is informational unless separately verified.
- Editor and operator roles are explicit platform grants, not inferred from GitHub organization membership.

### 7.2 Challenge repositories

Every public challenge version resolves to a public GitHub repository and full commit SHA. The platform archives a normalized source snapshot immediately; deleting or force-pushing the repository cannot change a locked version.

GitHub webhook deliveries are authenticated with `X-Hub-Signature-256`, compared in constant time, recorded by delivery ID, and processed idempotently. A repository change creates a draft of a new version; it never mutates a live version.

### 7.3 Private solver submissions

The initial solver workflow is GitHub-based without requiring a public fork:

```text
git commit and push to a selected public or private GitHub repository
→ grant the Science Ladder GitHub App read access to that repository
→ science-ladder submit --challenge <slug> --ref HEAD
→ API resolves the full remote SHA with its installation token
→ server fetches and canonicalizes only allowed paths into private storage
→ CLI preview digest is compared with the server digest
→ free subject-bound ValidationGrant and capacity are reserved
→ sequence receipt is assigned
```

Private challenge work should use a normal private repository rather than a public-repository fork, whose visibility rules may expose it. The App installation is scoped to the selected repository, and its short-lived token is discarded after the snapshot. GitHub API archive responses are still untrusted and pass through the canonical streaming parser.

If the submission claims a milestone and advances the public frontier, the platform publishes the canonical artifact bundle and attribution immediately under the submission's predeclared license, optionally mirroring it to a platform-owned GitHub solution repository. The solver's source repository itself need not become public. Non-winning bundles remain private by default. A solver may publish one voluntarily, in which case it can become the public frontier.

This produces two related records:

- **verified best:** the strongest valid official score, which may refer to a private non-winning artifact; and
- **public frontier:** the strongest published artifact that other solvers can actually inspect and build on.

Every milestone-winning result is public, so its verified best also advances the public frontier. The challenge page must not imply that a private artifact is available as a starting point.

---

## 8. Challenge creation and preflight

### 8.1 Challenge Scout flow

The portable Challenge Scout prompt and its output schema live in the open repository and are versioned protocol assets.

```text
topic / question / paper / dataset / repository
→ prefilled prompt for the user's own agent
→ science-ladder-candidate.yaml
→ local schema lint
→ server-side source resolution and duplicate checks
→ creator review and adoption
→ scaffolded challenge repository
```

Agent output is always a draft. It cannot publish a challenge, invent a verified citation, waive safety review, or certify a validator. The candidate records prompt version, model-provided provenance if available, source URLs/identifiers, quoted evidence locations, uncertainty, and unresolved work.

The future continuous crawler writes the exact same `ChallengeCandidate` envelope. It gains scheduling, source ingestion, deduplication, and ranking; it does not gain a path around named creator adoption and preflight.

### 8.2 MVP validator profile

`artifact-checker-v1` has a deliberately narrow contract:

- one platform-supported Linux `amd64` checker runtime, initially Python-oriented;
- a platform-owned, declarative build recipe and digest-pinned base image;
- dependency lockfile with hashes;
- fixed argv entrypoint, never a shell-valued production command;
- one canonical data-artifact tree whose executable bits and active-content types are forbidden;
- public fixtures and optional precommitted hidden suite;
- one typed result document; and
- fixed resource classes.

Each guest is composed from a platform-owned kernel, read-only root filesystem, and minimal PID 1/init agent plus a **separately mounted** creator-validator filesystem produced during preflight. The init agent is protected from the unprivileged validator process, applies the guest environment/cgroup controls, launches the locked argv directly, waits for termination, reads one result file, closes the validator, and emits one bounded frame to the host. The official filesystem interface is:

Version 1 derived inputs use a read-only SquashFS profile built only by pinned platform tooling: sorted canonical paths, normalized ownership/modes, zeroed timestamps, no xattrs, fixed compression settings, and single-threaded image creation. The filesystem-format/tool digest is part of the execution profile. Validator, submission, and suite images are produced before a production job; the gateway only verifies and attaches them.

```text
/sl/challenge     read-only locked challenge files
/sl/submission    read-only canonical solver artifact
/sl/suite         read-only official suite
/sl/work          bounded scratch
/sl/output        bounded typed-result channel
```

The validator must write exactly one small schema-valid `/sl/output/result.json`. The platform-owned guest agent—not the creator validator—frames that file over vsock/serial after the validator exits. Standard output is never interpreted as a score. The entire guest is treated as jointly hostile even though Science Ladder does not intentionally invoke submission bytes.

### 8.3 Remote preflight pipeline

1. Resolve the exact Git commit and construct the canonical source snapshot.
2. Validate schemas, citations, evidence locations, licenses, dependency locks, paths, sizes, fixture declarations, deadlines, and safety classification.
3. Validate milestone arithmetic: thresholds are strictly ordered, exactly representable in ticks, beyond the reproduced baseline in the correct direction by at least the meaningful delta, and individually claimable once; require `economicMode: none`.
4. Scan for credentials, malware indicators, prohibited file types, vulnerable dependencies, and incompatible licenses.
5. Build in a disposable quarantine microVM. Required dependencies pass through an allowlisted caching proxy and must match exact hashes.
6. Rebuild offline in a second clean environment and require the same canonical `validator_disk_digest`, which is the execution-reproducibility criterion. Each OCI digest remains provenance unless/until the builder also canonicalizes config timestamps, history, layer metadata, and compression; OCI-byte equality is not assumed implicitly.
7. Inside quarantine, compose the final read-only creator-validator filesystem/block artifact; sign its descriptor so production never unpacks OCI layers or creator archives.
8. Generate an SBOM and archive the build recipe, dependency set, logs, OCI digest, and validator-disk digest.
9. Run the baseline plus known-valid, known-invalid, malformed, empty, oversized, timeout, and numeric-boundary fixtures.
10. Run the malicious conformance corpus: archive traversal, Unicode collisions, parser fuzz cases, decompression bombs, fork/memory/disk/output bombs, and mutated artifacts.
11. Repeat the baseline across clean hosts and times to expose clock, randomness, architecture, thread-count, and environment dependence.
12. Build and seal the one MVP hidden-suite disk before publication, reproduce its baseline, and publish its commitment.
13. Simulate out-of-order submissions and every milestone-threshold transition.
14. Issue a signed `MachineConformanceReceipt` over build and executable results.
15. Run the structured LLM legibility/proxy review. Its output may flag or route; it cannot certify executable safety.
16. Produce the remaining scientific-legibility, rights, safety, and competition-contract review reports.
17. Issue the final signed `ChallengeLockReceipt` over every semantic input, machine receipt, and immutable review-report digest.

Outcomes are `pass`, `fail_with_actionable_findings`, or `human_review_required`. An editor can mark a passing challenge Human-reviewed and/or Featured, but cannot silently change its contract.

---

## 9. Official validation plane

### 9.1 Job handoff

The control plane commits a `ValidationJob` and outbox record in the same database transaction. A runner gateway in the validation account makes an outbound mTLS request for work. It receives a signed envelope containing only:

- job and attempt IDs;
- challenge lock and submission acceptance receipt digests;
- content-addressed locations and expected sizes for the prebuilt validator, submission, and suite disks plus their semantic digests;
- execution-profile and runner-policy digests;
- resource limits and deadline; and
- a one-use result-upload capability.

The gateway verifies the platform signature before downloading anything. It streams each already-final object into a job workspace, checks size and digest, and attaches the **prebuilt** read-only validator, submission, and suite disks produced by trusted platform tooling in quarantine. The production gateway never unpacks an OCI layer, repository archive, submission bundle, or creator-controlled filesystem format. It then assigns a compatible host. Validation hosts never connect directly to the control database.

Queue delivery is at least once. Attempt IDs, signed inputs, unique database constraints, and idempotent result ingestion make duplicates harmless. Queue order has no competitive meaning.

### 9.2 One disposable microVM per run

`runnerd` controls Firecracker directly on dedicated KVM-capable Linux hosts. Every run gets:

- a current production Firecracker binary with default seccomp filters and Jailer;
- a unique unprivileged UID/GID, namespace, cgroup, workspace, and Unix socket;
- a patched host implementation plus the lock-pinned guest kernel, root filesystem, CPU feature mask, and runtime configuration;
- **no virtual network device**, rather than merely a guest firewall;
- no metadata service, shared host filesystem, container socket, platform credential, or secret;
- immutable root, challenge, submission, and suite block devices;
- a capped writable scratch disk;
- host-enforced VM vCPU, memory, disk, I/O, output, and wall-time limits, plus guest-agent cgroup/PID limits as defense in depth;
- one tiny length-framed vsock or serial result channel with a hard byte cap; and
- destruction of the VM and cleanup of all scratch state after one workload.

Host swap is disabled or encrypted and scrubbed according to the runner policy. **Every official competitive run**, including its first run, uses the strict MVP tenancy class because milestone/frontier relevance is unknowable until after scoring: one microVM/job at a time per physical host, SMT and kernel same-page merging disabled, vCPUs pinned to physical cores, and no shared writable backing. Confirmation runs use different hosts and failure domains. Dense execution is limited to local development, preflight, or explicitly noncompetitive runs whose receipts cannot affect frontier or milestone state. Hosts rotate out for patching.

For hidden-suite jobs, every guest-writable or host-captured byte—`/sl/work`, `/sl/output`, swap if any, raw stdout/stderr, crash data, and temporary result framing—lives on RAM-backed storage or under the same per-job ephemeral encryption boundary. No plaintext snapshot/cache is permitted, and teardown crypto-erases every associated key before the cleanup attestation is signed.

The lock-bound `execution_profile_digest` covers reusable guest-visible or score-visible platform semantics: guest kernel/rootfs, CPU feature mask, vCPU/thread count, locale, timezone, umask, filesystem/fixture ordering, environment allowlist, fixed-time policy, deterministic seed, `PYTHONHASHSEED`, BLAS/OpenMP thread settings, and numeric-library set. The challenge lock binds that profile and its challenge-specific validator-disk digest separately. A `runner_implementation_epoch` records host kernel, microcode, Firecracker/Jailer, and host policy. Host/VMM security patches may create a new epoch after equivalence tests; changing any lock-bound semantic input requires a new challenge lock/version and an explicit migration of still-open milestones.

### 9.3 Run result

The guest reports only the declared result schema. The trusted host controller records:

- raw reported decimal string and normalized integer score;
- every required hard-gate result;
- termination reason;
- CPU, memory, disk, I/O, and wall-time usage;
- encrypted raw-log digest and platform-generated public diagnostic-code list;
- all semantic-input, derived-disk, image, kernel, rootfs, profile, and policy digests;
- worker and physical-host identity; and
- start/end times, nonce, and runner implementation epoch.

Each runner host has a revocable per-host identity/key certified by the platform; it never receives the platform root receipt key or unrestricted KMS signing authority. It host-signs a job-bound `ValidationRunReceipt` and uploads it through the one-use capability. The control plane validates the certified host identity, signature, expected job binding, inventory-backed host/failure-domain identity, schema, and uniqueness, then adds a platform countersignature. Scheduler anti-affinity relies on that control-plane inventory, not host self-report.

Raw guest output is hostile and may leak hidden-suite content. It is encrypted, capped, retained briefly for restricted incident investigation, and never rendered as HTML or SVG. A hidden official run exposes only platform-generated error categories and manifest-enumerated diagnostic identifiers—never creator-controlled strings, stdout fragments, tracebacks, filenames, or per-case output. Public-suite local runs may show escaped diagnostics under a separately declared policy.

### 9.4 Confirmation and nondeterminism

A result capable of claiming an open milestone **or advancing the public frontier** is executed again on a distinct physical host. Scheduler anti-affinity is mandatory.

- If both runs pass and their normalized scores are within the declared integer-tick tolerance, adjudication uses the worse confirmed score: lower for maximize, higher for minimize.
- If they disagree beyond tolerance or differ on any hard gate, the result is `nondeterministic`, claims no milestone, and is routed for review.
- Input outside the declared contract, a typed validator rejection, or guest resource exhaustion receives a signed solver/input terminal category.
- A repeatable checker crash on an in-contract artifact is a `challenge_fault`, not silently a solver loss or infrastructure fault.
- A host/VMM/storage/control failure is `infrastructure_fault`, retries on a clean worker, and does not consume the solver's validation grant.
- A guest compromise signal is a security incident that quarantines the host and pauses the challenge.
- Repeated challenge faults pause the version. If it cannot score the accepted artifact under its immutable contract, the version is marked compromised, all unadjudicated receipts end as `challenge_unscorable`, affected validation grants are restored, and no later receipt skips ahead. Relaunch requires a new version.

Security patching overrides historical host preservation. A new runner implementation epoch is equivalence-tested against baselines and public frontiers before use. Any guest-visible semantic change creates a new challenge version rather than an equivalence attestation against the old lock.

### 9.5 Hidden suites

Hidden evaluation is optional and receives a lower transparency classification until revealed. The MVP permits exactly one hidden suite per immutable challenge lock; “suite epoch” is therefore fixed for the life of that version. Any replacement requires a newly committed and locked challenge version before its first accepted submission. When used:

- bytes are canonicalized and uploaded before challenge publication;
- the platform records separate private plaintext and encrypted-object digests, generates a random 32-byte salt, and publishes only `SHA-256("science-ladder-hidden-suite-v1\0" || salt || plaintext_digest_bytes)`;
- the salt and suite remain encrypted and undisclosed until reveal;
- a per-suite data key encrypts the at-rest object, and each job receives a newly wrapped ephemeral key;
- each submission acceptance receipt pins the suite epoch;
- before lock, only the quarantine preflight worker may materialize it to reproduce the baseline; after lock, only the runner gateway may do so, always on a RAM-backed or encrypted ephemeral volume with no plaintext cache or snapshot, attached read-only;
- teardown destroys the job key and records an attested cleanup outcome; crypto-erasure, not unreliable SSD overwriting, is the boundary;
- raw per-case results and arbitrary validator evidence are never returned; and
- retired suites and salts are revealed when rights permit.

A malicious creator can still encode hidden data into a score or write a collusive validator. Isolation cannot prove scientific honesty. Public validator source, feedback quantization, immutable suite commitments, validation quotas/rate limits, adversarial preflight, flags, and creator reputation are the appropriate defenses.

### 9.6 Local parity

The CLI and official infrastructure share one open runner-contract library for:

- canonical tree construction;
- job-spec generation;
- filesystem paths and mount semantics;
- result parsing and score normalization;
- public fixture execution; and
- receipt verification.

Creators use `science-ladder challenge test` before lock. It applies the candidate platform build recipe, runs the baseline and fixture suite, and emits a non-authoritative candidate report; it cannot predict the eventual OCI/disk digest. Solvers use `science-ladder validate --local` after publication to run the exact locked validator disk/image, public suite, official directory layout, network-disabled mode, and equivalent declared limits. It emits an unsigned, clearly non-authoritative local report.

On Apple Silicon, local emulation is allowed with a prominent execution-profile mismatch warning. Official MVP outcomes always use Linux `amd64`. A public golden corpus ensures the local and official implementations agree on all semantic cases; isolation mechanisms need not be identical on a developer laptop.

---

## 10. Submission ordering, frontier, and milestone adjudication

### 10.1 Admission and competitive receipt

A sequence number is assigned only after all of the following are true:

1. the complete canonical artifact is privately stored;
2. the server has independently verified its digest, structure, size, declared type, inactive-content policy, and malware scan;
3. the deterministic read-only submission disk is built in quarantine, stored, and independently verified against `submission_disk_digest`;
4. a free platform `ValidationGrant` bound to this exact artifact, challenge, resource class, and primary-plus-confirmation allowance is available;
5. the challenge is open and before its acceptance deadline;
6. per-user limits and worst-case primary-plus-confirmation capacity admission pass; and
7. the exact challenge lock, `economicMode: none`, suite epoch, execution profile, and validation-grant reference are known.

All mutable preconditions are checked again inside one serializable transaction. The API locks the challenge-version admission row, creates a durable capacity reservation for worst-case primary and confirmation units, enforces the active-submission quota under constraint, verifies open/deadline state, reserves—but does not yet consume—the exact `ValidationGrant`, increments `next_receipt_sequence`, inserts the submission, and writes the final canonical acceptance-receipt payload/digest plus outbox event.

The capacity reservation carries a monotonic fencing token. Creating the primary job atomically converts its units into a primary attempt lease while holding the confirmation units. A nonqualifying/non-frontier terminal result releases the held units; a potential milestone or public-frontier result converts them into an anti-affined confirmation lease. Terminal completion releases each lease. A scheduler may recover an abandoned lease only after its fencing token is superseded and result storage is reconciled; a stale worker cannot commit afterward. Capacity failure after acceptance is a platform incident and never invalidates the receipt.

KMS signing occurs idempotently **after commit**, never inside the database transaction. If the API dies between commit and response, a signer job completes the same persisted payload and the client retrieves the original receipt. The database commit establishes acceptance time and sequence; no signed but uncommitted receipt can exist.

An incomplete upload, exhausted validation grant, failed structural/safety check, duplicate artifact, or unavailable capacity holds no place in line. A duplicate is rejected before sequence assignment; the same authenticated owner receives the original receipt, while another caller learns no private details. The challenge deadline applies to committed acceptance time, not worker completion time or later signature completion.

### 10.2 Parallel execution, serial adjudication

Workers may finish in any order. Each challenge version owns an `adjudication_watermark`, initially zero. The adjudicator locks that row and processes only the next contiguous final receipt.

```text
receipt 41 final ─┐
receipt 42 running├─► watermark remains 41; receipt 43 is provisional
receipt 43 final ─┘

receipt 42 becomes final
→ process 42
→ process 43 in the same or next transaction
→ watermark becomes 43
```

For each receipt, one transaction:

1. confirms that it is exactly `watermark + 1`;
2. reads the independently confirmed score and hard gates;
3. compares it with the prior verified best and all unclaimed milestone thresholds;
4. advances `verified_best` if appropriate;
5. flips the already-stored, pre-scanned artifact's database authorization to public and inserts a frontier event if the artifact is or becomes public;
6. atomically creates a `MilestoneClaim` for every crossed milestone using unique constraints;
7. advances the watermark; and
8. inserts Git-mirroring, notification, future-extension, and audit outbox events.

This transaction contains no network calls. The versioned future-extension event is inert in the MVP; it is the provider-neutral handoff that a later active `RewardProgram` may consume for new payment-enabled seasons.

### 10.3 Terminal classification

Receipt-ordered fairness requires every earlier receipt to resolve:

- `valid_non_record`, `valid_record`, `hard_gate_failed`, `invalid_output`, `resource_limit`, `declared_timeout`, `nondeterministic`, `malicious`, or version-wide `challenge_unscorable` are terminal;
- `infrastructure_fault` is not silently converted into a competitive loss and retries on clean hosts;
- repeatable `challenge_fault` pauses the version and follows the compromise/resolution protocol in section 9.4 rather than skipping the receipt;
- resolution produces a signed record, even when the outcome is failure.

The platform never invents a timeout merely to let a later promising answer win.

### 10.4 Publication and lineage

The system stores both `verified_best_submission_id` and `public_frontier_submission_id`.

- Every accepted artifact is already in its final pre-scanned content-addressed location. For a milestone-winning public-frontier artifact, the adjudication transaction atomically sets `public_at` and access authorization alongside claim creation; asynchronous Git mirroring may lag without making publication ambiguous.
- A non-winning record remains private unless the solver opted into publication.
- New solver prompts and starter branches point only at the public frontier.
- Every submission records the public frontier it used as its parent, but scoring does not assume a linear Git history.
- A platform-hosted solution bundle and signed receipts are canonical; Git mirrors are convenience views.

Full acceptance and validation receipts for private submissions remain private. The public audit log exposes only sequence, terminal-status class, and a randomly salted commitment—not the raw artifact digest, which could reveal a low-entropy artifact by enumeration. When the solver publishes or a suite is revealed, the salt and complete receipt can be disclosed for recomputation. Until then, outsiders can verify the platform's signed ordering/adjudication claim but cannot independently rerun unavailable bytes.

This retains compounding scientific progress without exposing losing work or creating merge-order races.

### 10.5 Challenge-version and milestone migration

The MVP permits only one intake-open competition season of a challenge at a time. Every scientific milestone has a globally unique immutable `milestone_id`; a version supplies that milestone's threshold under its locked score contract. Claims consume the global milestone ID, not merely a version-local row.

A security-required version migration follows one ordered protocol:

1. pause old-version intake and keep the new version unable to accept submissions;
2. resolve every already accepted old-version receipt through its final watermark, or complete the signed version-wide `challenge_unscorable` procedure;
3. build/review/lock the new version and reproduce its baseline and last public frontier;
4. in one serializable transaction, assert `old_watermark == old_last_sequence`, close the old version, transfer only still-unclaimed global milestones to their new-version threshold mappings, write a signed migration payload, and open the new version only after the transfer commits.

Claimed milestones never transfer or reopen. The new version cannot accept a receipt before the transfer commits, so there is no undefined cross-version race or duplicate claim. Ordinary creator changes may instead close the old season and create an entirely new ladder with new milestone IDs. A future payment-enabled season is always new and prospective; this migration path cannot attach money to old claims.

---

## 11. Milestone boundary and post-MVP payment architecture

### 11.1 Payment-free MVP boundary

The MVP implements one economic-neutral ledger: immutable `MilestoneTier` definitions and first-to-threshold `MilestoneClaim` records. It stores no reward amount, wallet connection, solver destination, monetary entitlement, payment command, payment receipt, creator default, service order, or billing event. No NWC, Nostr-relay, LNURL, DNS-payment, or Stripe credential is provisioned in an MVP environment.

Adjudication emits a versioned `milestone.claimed.v1` outbox event containing the challenge lock, milestone IDs, submission and acceptance receipt, confirmed score, and public artifact digest. The event has no payment semantics when `economicMode: none`. A future reward consumer may act only when a `RewardProgram` was locked for a new payment-enabled season before that submission's acceptance; a feature flag alone is never authority.

The eventual monetary system has three distinct domains:

1. **Reward entitlement:** deterministic creator obligations produced from a pre-existing reward program and final milestone result.
2. **Payment execution:** destination resolution, NWC commands, reconciliation, receipts, and creator-default incidents.
3. **Service billing:** money paid to Science Ladder for one defined validation service, producing the same subject-bound `ValidationGrant` used by the MVP.

Billing cannot set a score, milestone, or reward amount. Reward code cannot call Stripe, resolve a destination, or decrypt a wallet. Payment code receives an already-final entitlement and cannot reevaluate science or ordering.

### 11.2 Activating a future reward program

A creator activates rewards only by publishing a new immutable competition season with `economicMode: bitcoin-reward`. Its `RewardProgram` binds the challenge lock, opening receipt sequence, deadline, global reward-obligation IDs, milestone mappings, bounded integer amounts, total maximum liability, publication policy, and settlement policy. A signed `FundingAuthorization` must exist before intake opens.

For a payment-enabled season, the existing serial adjudication transaction still chooses the earliest receipt and claims every crossed milestone. In the same transaction it claims each corresponding global reward obligation, creates one `RewardEntitlement` with checked line items, and emits a payout request. The entitlement begins `owed`; transport state is separate. Old payment-free claims, accepted receipts, and closed seasons are never converted retroactively.

### 11.3 Solver payment destinations and BIP-321 normalization

The target fast-follow schema reserves a versioned tagged union so identical-looking strings cannot be misclassified:

| Type | Planned input | Normalized payment instruction |
|---|---|---|
| `lightning_address` | Legacy Lightning address | Resolve LNURL-pay to an exact BOLT 11 invoice, then wrap as `bitcoin:?lightning=…` |
| `bip353_name` | Reserved-disabled pending the provenance gate; later `user@domain` or displayed `₿user@domain` | DNSSEC-validate the BIP-353 record and retain its BIP-321 URI plus proof provenance |
| `bolt12_offer` | Direct `lno1…` offer | Validate/canonicalize and wrap as `bitcoin:?lno=…` |

The resolver accepts only the declared type, verifies network and fixed-amount compatibility, rejects mixed-case or malformed BOLT 12 strings, and rejects BIP-321 `pop` or `req-pop` callbacks for unattended payout. A reward-eligible acceptance receipt snapshots a destination version. Each payment command additionally binds the exact resolved-instruction digest so retry cannot silently re-resolve to another recipient.

BIP-353 resolution validates DNSSEC locally to the root, follows only validated CNAME/DNAME chains, rejects ambiguous records, honors signed TTL bounds, and persists the proof digest. The first release accepts ASCII names only to reduce homograph risk and requires the record to yield one eligible BOLT 12 `lno` instruction, avoiding unpredictable wallet selection among multiple instructions. It never treats a recursive resolver's validation assertion as sufficient.

There is a current interoperability blocker: BOLT 12 requires an invoice request derived through BIP-353 to include `invreq_bip_353_name`, but NWC-321 currently accepts only the resolved BIP-321 URI and has no provenance field. Science Ladder keeps `bip353_name` in the target schema but does not enable it until NWC standardizes that provenance, the wallet resolves the name itself, or a reviewed interoperable extension exists. `payer_note` cannot substitute for the required field.

### 11.4 App-initiated NWC authorization

The creator starts wallet connection from Science Ladder using NWC-08. The isolated connection service—not browser JavaScript—generates and retains a fresh client keypair plus a single-use high-entropy `state`. The browser opens the wallet's HTTP confirmation surface or `nostr+walletauth://` deep link. Public keys, state, relay URLs, requested permissions, and other non-secret authorization parameters cross the boundary; the private client key does not.

The service requests `pay` and `get_info`, plus only the minimum wallet-supported method needed for authoritative transaction reconciliation. It requests an isolated, expiring, non-renewing spending authority bounded to maximum remaining liability plus routing allowance when the wallet supports those constraints. Completion requires exact state matching, expected wallet-origin and identity checks, a valid signed info/grant event, NIP-44 v2, an approved relay set, the correct network, and confirmation of the actual granted capabilities. State becomes terminal and non-reusable after success, decline, or timeout.

NWC-321 `pay` was merged on August 2, 2026 but remains explicitly `draft` and `optional`. NWC-08 remains an open `draft` and `optional` proposal. The fast-follow pins audited revisions behind provider-neutral adapters and ships only against a tested wallet compatibility matrix. It does not commit the MVP to a particular SDK or wallet implementation.

The encrypted client key is decryptable only by the payout executor. That executor has no public egress: it sends opaque signed/encrypted events through an SSRF-hardened relay proxy. The public destination resolver has no wallet key. Neither process can access validation objects or reevaluate entitlements.

Successful setup produces a signed `FundingAuthorization` bound to the reward program and challenge lock. It records the connection fingerprint, network, wallet-service key, relays, negotiated extension revisions, granted methods, expiry, effective spend policy, observed coverage, routing allowance, and compatibility receipt. Its public view shows coverage and policy without disclosing unrelated wallet balance.

### 11.5 NWC-321 payment command and reconciliation

The payout executor calls NWC-321 `pay` with one canonical BIP-321 URI. When the selected BOLT 12 offer is amountless, it supplies the exact reward in the request's wire-level `amount`; it may supply `max_fee`, but treats absent `fees_paid` as evidence that the wallet may not have enforced that cap. For BOLT 12 the wallet performs invoice-request and invoice retrieval.

```text
entitlement: owed ─────────────────────────────────────────────► paid

execution: queued
→ instruction_resolving
→ instruction_ready
→ request_committed
→ pending | settled | failed

exception branches:
solver_action_required | creator_action_required
payment_unknown | retryable_failure
```

Before relay publication, the executor persists the exact signed and encrypted NWC request event, event ID, instruction digest, exact amount, entitlement ID, expected wallet-service key, expiry, and correlation fields. Every response is rejected unless its signature, author, kind, NIP-44 mode, request reference, result type, amount, selected instruction type, and command identity match.

NWC-321 returns a required wallet-scoped `transaction_id`, `state`, selected `instruction_type`, and paid amount. Payment hash, preimage, fees, and BOLT 12 payer proof are optional; none can be a universal uniqueness or settlement invariant. The certified wallet must support authenticated correlation after a lost first response using the exact request-event ID or another tested wallet-defined key; `transaction_id` becomes the durable wallet-record reference once known.

After an ambiguous response, reconciliation may replay only the exact persisted Nostr event and looks up wallet history using the known `transaction_id`, exact request-event ID, or other certified wallet-defined correlation key. It never creates a fresh command, re-resolves the destination, replaces the instruction, or switches wallets until authenticated wallet evidence proves the previous command terminal and unpaid. A new request event is persisted before every authorized send. Distributed delivery is not magically exactly once; the system guarantees one entitlement, one active logical command, persist-before-send, authenticated correlation, and reconciliation before replacement.

### 11.6 Creator default after payment activation

If the creator revokes authority, removes funds, or otherwise blocks an earned payment, the entitlement remains owed. The cure clock starts only after a creator-specific authenticated failure while the resolver, relay, and platform are healthy. A unique immutable `CreatorDefault` records opening, deadline, default, and eventual cure. It suspends new payment-enabled receipts for the creator, removes delinquent challenges from featuring, lowers public payment reliability, and can lead to removal. Restoring authority pays the original obligation but never erases the incident.

### 11.7 Stripe validation billing as an independent fast-follow

The MVP issues free `ValidationGrant` records from invitation quotas. Each grant is bound to one source snapshot or submission artifact, challenge lock, resource class, and primary-plus-confirmation allowance. It is reserved during competitive acceptance and consumed when the first official job is durably created; platform failure restores the same grant.

The billing fast-follow adds `ServiceOrder → Stripe Checkout/Link → verified webhook and authoritative reconciliation → ValidationGrant`. It does not change the validation or adjudication interface. Provider event IDs deduplicate transport; provider object plus semantic state transition deduplicates effects; checkout and refunds use stable idempotency keys. A later chargeback creates billing debt but never changes receipt order, a scientific result, milestone claim, reward entitlement, or creator payment.

Stripe remains disabled until written approval for the complete use case and jurisdictional review. It is used only for Science Ladder's outcome-independent validation service and never for creator rewards. No Bitcoin service-billing provider is selected in this architecture.

---

## 12. Data architecture

### 12.1 Sources of truth

| Data | Authority | Notes |
|---|---|---|
| Domain state and ordering | PostgreSQL | Normalized relational state with constraints and transactions |
| Large immutable bytes | Object store | Content-addressed keys, encryption, retention policy, malware/quarantine states |
| Validator runtime | OCI registry | Image referenced only by immutable digest |
| Work delivery | PostgreSQL job tables | Delivery mechanism only; workers reload authoritative rows |
| Public audit history | Append-only audit table plus signed checkpoints | Exportable; periodically anchored by a Merkle root |
| Search | PostgreSQL full-text/trigram initially | Avoid a separate search system until demonstrated need |

Database backups use point-in-time recovery. Immutable public objects and receipts are separately replicated and can be regenerated into an independent verifier. Deleting an account may remove private/profile data where legally required, but cannot falsify public scientific, milestone, frontier, or incident history; privacy-sensitive references use stable pseudonymous IDs.

### 12.2 Core tables and constraints

| Domain | Principal tables | Critical constraints |
|---|---|---|
| Identity | `users`, `github_identities`, `organizations`, `memberships`, `api_tokens` | Unique GitHub identity; hashed tokens; explicit roles |
| Candidate discovery | `prompt_versions`, `challenge_candidates`, `candidate_sources`, `candidate_imports` | Candidate digest unique per version; source resolution status retained |
| Challenges | `challenges`, `challenge_versions`, `challenge_locks`, `citations`, `review_runs`, `editorial_decisions` | Live version immutable; one lock per version; repository+commit archived |
| Evaluation contract | `execution_profiles`, `validator_images`, `suite_epochs`, `baselines` | All addressed by digest; hidden suite sealed before publish |
| Competition | `milestone_tiers`, `milestone_version_mappings` | Globally unique milestone; strict threshold ordering; mapping changes only through the declared migration protocol |
| Resource access | `validation_grants`, `grant_reservations` | Subject-bound grant cannot be transferred or double-reserved |
| Submissions | `submission_intents`, `github_submission_snapshots`, `submissions`, `submission_receipts` | GitHub SHA independently resolved; unique artifact per challenge; sequence unique and monotonic |
| Validation | `capacity_reservations`, `attempt_leases`, `validation_jobs`, `validation_attempts`, `validation_runs`, `confirmation_pairs` | Worst-case units reserved before receipt; monotonic fencing; one accepted signed result per attempt; host anti-affinity for pair |
| Adjudication | `challenge_adjudication`, `frontier_events`, `milestone_claims` | Contiguous watermark; unique milestone claim; one atomic all-crossed claim set |
| Governance | `flags`, `moderation_actions`, `challenge_incidents` | Append-only action history; no destructive state rewrite |
| Operations | `outbox_events`, `jobs`, `audit_events`, `audit_checkpoints` | Unique idempotency keys; signed checkpoint sequence |

Payment and billing tables are deliberately absent from MVP migrations. The post-MVP migration adds `reward_programs`, `reward_obligations`, `reward_entitlements`, `reward_entitlement_lines`, `payout_destination_versions`, `resolved_payment_instructions`, `wallet_connections`, `funding_authorizations`, `payment_commands`, `nwc_request_events`, `payment_attempts`, `payment_receipts`, `creator_defaults`, `service_orders`, and `billing_events`. Payment hash and preimage remain nullable because NWC-321 does not guarantee them; the durable correlation fields are entitlement, command, exact request-event ID, instruction digest, wallet identity, and wallet-scoped transaction ID.

### 12.3 Key state machines

Challenge facts are orthogonal rather than forced into one overloaded state:

```text
contract lifecycle:
draft → machine_preflight → review_ready → locked → published → closed | superseded
             └→ changes_required → revised_draft → machine_preflight

review outcome:
pending → automated_pass | human_review_required
human_review_required → human_approved | changes_required | rejected

intake availability:
unavailable ↔ open ↔ paused → closed

incident status:
none → investigating → resolved | compromised

editorial badges:
Human-reviewed and Featured are independent revocable labels
```

`human_review_required` therefore has approve, changes-required, and reject paths. Any changes-required result returns to a revised draft/source snapshot and reruns machine preflight and affected reviews; it never patches a prior receipt. A paused challenge can resume if the same immutable contract remains sound. Only a new version can change semantic fields after lock. Compromise closes intake and preserves the original lock/history rather than mutating the contract.

Submission facts are likewise separate:

```text
processing:
intent → github_fetch → structurally_valid → grant_reserved → accepted
→ queued → running → confirmation_running → finalized

validation outcome:
pending | valid | hard_gate_failed | invalid_output | resource_limit
| nondeterministic | malicious | challenge_unscorable

challenge pointers/events:
verified_best_submission_id: optional
public_frontier_submission_id: optional
(one submission may occupy both pointers)

publication:
private | public

milestone relation:
none | claimed

economic mode:
none
```

A submission can simultaneously be valid, a public-frontier advance, public, and milestone-winning. `infrastructure_fault` is retryable and nonterminal for competitive ordering. The acceptance receipt, not a UI label, establishes sequence. Future reward and payment state machines are separate and defined in section 11; they do not exist in the MVP database.

### 12.4 Transactions and locking

Use short, explicit transactions. Network calls and long computation never hold database locks. Important transaction boundaries are:

- competitive sequence assignment plus subject-bound validation-grant reservation and outbox insert;
- first official validation-job creation plus reserved validation-grant consumption;
- validation-result acceptance plus final-state update;
- contiguous adjudication plus frontier update, all milestone claims, and outbox events.

The adjudicator locks one `challenge_adjudication` row at a time. General workers can claim jobs with `FOR UPDATE SKIP LOCKED`, but queue locking never determines winner order.

---

## 13. API and event surface

The public contract is REST under `/v1` with an OpenAPI document. Long-running state changes return resource IDs and status URLs. Challenge and submission pages use server-sent events with polling fallback.

### Representative public endpoints

```text
POST   /v1/auth/cli-sessions
GET    /v1/prompts/challenge-scout/{version}
POST   /v1/prompts/challenge-scout/{version}/prefill
POST   /v1/candidates/validate
POST   /v1/candidates/import
GET    /v1/candidates/{id}

POST   /v1/challenges
POST   /v1/challenges/{id}/versions
POST   /v1/challenge-versions/{id}/preflights
GET    /v1/preflights/{id}
POST   /v1/challenge-versions/{id}/lock
POST   /v1/challenge-versions/{id}/publish
GET    /v1/challenges/{slug}
GET    /v1/challenge-versions/{id}/events

POST   /v1/submission-intents          # GitHub repo + exact ref
GET    /v1/submission-intents/{id}     # fetch/canonicalization status
POST   /v1/submission-intents/{id}/accept
GET    /v1/submissions/{id}
POST   /v1/submissions/{id}/publish
GET    /v1/milestone-claims/{id}

POST   /v1/flags
GET    /v1/receipts/{digest}
GET    /v1/exports/challenge-versions/{id}
```

State-changing calls require an `Idempotency-Key`. Submission intent starts a server-side GitHub fetch into quarantine. Acceptance is unavailable until the full remote SHA, structure, scan, canonical bytes, digest, validation grant, and capacity reservation have been independently verified.

The post-MVP API surface is a separate capability group—`reward-programs`, `payout-destinations`, `nwc-authorizations`, `reward-entitlements`, `payment-commands`, `payment-receipts`, and `service-orders`. Those routes are not registered, advertised in OpenAPI, or reachable in the MVP.

### Internal interfaces

Internal runner endpoints use a separate hostname with mTLS identities, narrow schemas, replay protection, request expiry, and no browser credentials. The runner never receives a general object-store credential; it receives scoped, short-lived reads. Result uploads are one use. Future payment endpoints use separate identities and networks described in section 11.

### Domain events

PostgreSQL outbox events drive side effects:

```text
candidate.imported
challenge.preflight_requested
challenge.locked
challenge.published
submission.accepted
validation.requested
validation.result_finalized
submission.adjudicated
frontier.advanced
milestone.claimed.v1
solution.publication_requested
```

Consumers are idempotent and identify work by immutable resource ID. Public audit events are a curated signed projection of consequential domain events, not a raw dump of private internal messages.

---

## 14. Security, privacy, and abuse controls

### 14.1 Secret classes

| Secret | Holder | Excluded from |
|---|---|---|
| GitHub App private key | API identity subsystem / KMS | Workers that do not call GitHub and runners |
| Platform signing key | Narrow signing service / KMS | Application memory as raw key material |
| Hidden-suite data key | Runner gateway via per-job grant | Control UI and guest output |
| Database credentials | Per-deployable role | Cross-domain table writes not required by that process |

KMS policies separate `Encrypt`, `Decrypt`, and `Sign`. Credential rotation is tested, and public receipts include signing-key IDs and a signed key-history document so old receipts remain verifiable.

NWC client keys, wallet connection data, and Stripe secrets are future secret classes. MVP infrastructure policy denies creating or mounting them; the fast-follow adds narrowly scoped KMS roles only with the corresponding services.

### 14.2 Supply-chain controls

- Pin GitHub Actions by full commit digest and minimize third-party actions.
- Require reviewed lockfile changes and automated dependency/vulnerability scans.
- Produce SBOMs and signed provenance for Science Ladder binaries and validator images.
- Protect release branches, require code review, and use short-lived workload identity instead of static cloud keys.
- Sign production containers and verify signatures at deployment.
- Separate challenge-builder caches from application build caches.
- Treat all creator build output as quarantined until preflight completes.

### 14.3 Input and content controls

- Stream uploads; never unpack untrusted archives before size and path policy is active.
- Verify MIME/type from bytes rather than trusting filenames or headers.
- Scan imported repositories and candidate documents for secrets and malicious files.
- Fetch user-supplied URLs through an SSRF-hardened proxy with DNS rebinding defenses.
- Escape all user text on render; never serve uploaded HTML/SVG from the application origin.
- Put public artifacts on a separate download origin with attachment-oriented content policy.
- Apply per-account, per-IP, per-challenge, and global quotas before expensive work.
- Require capacity admission before granting a competitive receipt.

### 14.4 Threat-control matrix

| Threat | Principal controls |
|---|---|
| Validator escape | Firecracker/KVM, Jailer, seccomp, namespaces, unique UID, patched hosts, no NIC, no secrets, one run per VM |
| Hidden-test theft | Submission bytes are never intentionally launched, whole guest is hostile, suite is encrypted/read-only, output uses platform codes only, and one suite is committed per lock |
| Malicious archive | Canonical streaming unpacker, path/type/count/ratio limits, adversarial corpus |
| Score manipulation | Typed result, exact decimal parser, hard gates, integer ticks, two-host confirmation |
| Race for a milestone | Atomic sequence receipt, ordered watermark, unique milestone claims, TLA+ invariants |
| Queue duplication/reordering | At-least-once-safe job handlers; queue order never adjudicates |
| Creator changes judge | Immutable lock digest; changes create a new version |
| Front-running | Private uploads, no public pending bytes, receipt only after complete verified upload |
| Forged GitHub event | HMAC verification, constant-time comparison, delivery-ID deduplication |
| Audit-history rewrite | Signed receipts, append-only rows, object versioning, periodic signed Merkle checkpoints |
| Cost denial-of-service | Structural checks before scarce work, free subject-bound grants, quotas, admission control, and fixed resource classes |
| Bad scientific proxy | Required evidence and rationale, adversarial LLM review, human flags, trust labels, immutable creator responsibility |

### 14.5 Privacy

The default telemetry policy excludes solution content, hidden-suite content, prompts sent to external solver agents, and raw validator logs. Trace IDs may reference opaque resource IDs, never submission paths or titles that could leak work. Future payment secrets and raw payment instructions inherit the same exclusion before their services are enabled.

Private losing artifacts use per-object encryption, access logs, a documented retention/deletion policy, and no operator browsing path outside an incident workflow. Public metadata clearly states in advance whether a losing aggregate score is visible.

---

## 15. Observability and operations

### Service-level indicators

Track at least:

- API availability and latency;
- challenge preflight duration and stage failure rates;
- queue age by job class;
- validation admission wait, run duration, platform-fault rate, and resource-limit rate;
- confirmation disagreement rate by challenge and execution epoch;
- adjudication-watermark lag and oldest blocking receipt;
- validation-grant exhaustion and restoration rate;
- milestone-claim and public-frontier publication latency;
- object digest mismatch, signature failure, and denied runner-network attempts; and
- manual interventions per published challenge.

### Alerts

Page an operator for suspected sandbox escape, signing-key misuse, duplicate milestone/frontier effect, audit-log inconsistency, hidden-suite exposure, or a challenge whose ordering watermark is blocked. Queue depth or ordinary invalid submissions are ticket/notification-level unless they threaten deadlines.

### Runbooks required before the MVP

- pause one challenge without affecting others;
- drain and patch a runner host;
- rotate signing keys while preserving receipt verification;
- handle repeated platform faults blocking adjudication;
- restore a validation grant after platform non-delivery;
- mark a challenge compromised and supersede it without deleting history;
- reveal a retired hidden suite;
- restore PostgreSQL and public objects from backup; and
- respond to suspected secret exposure or sandbox escape.

No operator action may directly edit a score, receipt sequence, milestone claim, or public-frontier pointer. Exceptional corrections are new signed events through a reviewed administrative command with dual authorization.

---

## 16. Deployment topology

### 16.1 Local development

Docker Compose provides:

- PostgreSQL;
- MinIO-compatible object storage;
- a local OCI registry;
- fake GitHub and LLM adapters; and
- web, API, worker, and local validation processes.

The CLI can run public validators through a local container runtime with network disabled. Linux developers and CI can run the official Firecracker contract; macOS uses emulation and displays the profile mismatch. Local end-to-end tests stop at the signed milestone-claim/future-extension event.

One command should bootstrap the stack, seed two reference artifact challenges, create test accounts, and run the golden conformance suite.

### 16.2 Preview and staging

Every pull request gets an application preview using fake external adapters. Shared staging adds:

- a real GitHub App installation on test repositories;
- dedicated nonproduction runner hosts;
- a test signing hierarchy;
- synthetic challenge/submission traffic.

Staging receipts and signatures are visibly namespaced and can never be mistaken for production.

### 16.3 Production

Recommended initial AWS layout:

```text
Control account
  CloudFront/WAF → load balancer → ECS/Fargate web + API + workers
  RDS PostgreSQL with Multi-AZ and point-in-time recovery
  S3 buckets: quarantine, canonical artifact CAS, public mirrors, receipts, audit
  ECR validator and application registries
  KMS, Secrets Manager, metrics/log pipeline

Validation account
  runner gateway with outbound mTLS only
  exact bare-metal instance family + hardened AMI proven in Phase 0
  minimum three reserved/on-demand hosts across labeled failure domains
  no inbound public route; no control database credentials
```

The production MVP account inventory contains no payment boundary, Nostr relay access, wallet KMS role, or Stripe secret. Those resources are added as isolated stacks only during their independently reviewed fast-follows.

Bucket policies separate quarantine, canonical private origins, optional public mirrors, and audit data. Public download authorization reads the transactional `public_at` state and issues access to the already-stored canonical object; mirror copies may lag and are non-authoritative. Object versioning is enabled for receipts and audit checkpoints; object lock can be added for the public audit bucket. The runner account receives time-limited grants to exact object digests, not bucket-wide credentials.

Deploy one region first. Backups replicate to a second region, but active-active processing is deferred because cross-region ordering complicates competitive receipt semantics. Disaster recovery may temporarily pause acceptance; it must never guess order.

The Phase 0 runner ADR records the exact instance family, AMI digest, `/dev/kvm` and Jailer proof, guest density class, measured boot/run cost, and capacity-reservation strategy. Milestone/frontier confirmation never relies solely on interruptible capacity. A host enrolls with its cloud instance-identity evidence and approved AMI, obtains a short-lived mTLS certificate and per-host signing delegation, and is assigned a control-plane failure-domain label before receiving work.

### 16.4 Why no Kubernetes initially

The control plane fits standard stateless application tasks plus managed PostgreSQL and object storage. Firecracker hosts require bespoke lifecycle and security work regardless of Kubernetes. ECS/Fargate and dedicated runner hosts reduce the number of operational systems the MVP team must secure. Kubernetes becomes reasonable only if measured scale, multi-cloud hosting, or independent validator operation justifies it.

---

## 17. Repository and module layout

Use one public monorepo so protocol changes, server behavior, CLI behavior, and conformance tests evolve atomically.

```text
science-ladder/
├── protocol/
│   ├── schemas/
│   ├── prompts/
│   ├── examples/
│   ├── conformance/
│   ├── test-vectors/
│   └── go/                      # Apache-2.0 Go module
│       ├── canonical/
│       ├── receipts/
│       ├── runnercontract/
│       └── go.mod
├── cli/                         # Apache-2.0 Go module
│   ├── cmd/science-ladder/
│   ├── internal/
│   └── go.mod
├── server/                      # AGPL-3.0 Go module
│   ├── cmd/
│   │   ├── api/
│   │   └── worker/
│   ├── internal/
│   │   ├── identity/
│   │   ├── github/
│   │   ├── candidates/
│   │   ├── challenges/
│   │   ├── review/
│   │   ├── submissions/
│   │   ├── validation/
│   │   ├── adjudication/
│   │   ├── milestones/
│   │   ├── grants/
│   │   ├── audit/
│   │   └── storage/
│   ├── db/
│   │   ├── migrations/
│   │   ├── queries/
│   │   └── fixtures/
│   └── go.mod
├── runner/                      # AGPL-3.0 Go module
│   ├── cmd/
│   │   ├── runner-gateway/
│   │   └── runnerd/
│   ├── internal/
│   ├── kernels/
│   ├── rootfs/
│   ├── images/
│   ├── policies/
│   ├── security-tests/
│   └── go.mod
├── web/
│   ├── app/
│   ├── components/
│   ├── generated-api/
│   ├── tests/
│   └── package.json
├── templates/
│   ├── artifact-checker-python/
│   └── reference-challenges/
├── deploy/
│   ├── compose/
│   └── opentofu/aws/
├── models/
│   └── adjudication.tla
├── docs/
│   ├── adr/
│   ├── rfcs/
│   ├── threat-model/
│   ├── operations/
│   └── contributing/
├── go.work                      # development workspace, not a license boundary
├── LICENSE
└── LICENSES/
```

### Internal module rule

Domain modules expose commands, queries, and immutable events; they do not reach into each other's tables ad hoc. The adjudication package is the only writer of frontier and milestone-claim state. The Apache protocol module imports no AGPL code, and the distributed CLI may import only the Apache module and permissively licensed dependencies. CI checks the complete CLI dependency/license graph and forbidden module edges on every change. Future payment and billing modules are added as separate deployables after their own ADRs; empty production-shaped services are not scaffolded in the MVP.

### Open-source split

- Server/web reference implementation: AGPL-3.0.
- Protocol schemas, prompts, CLI, SDK, receipt verifier, and conformance corpus: Apache-2.0.
- Challenge, dataset, and solution artifacts retain their declared compatible licenses.

The repository is public from the first protocol commit. Security-sensitive deployment values are configuration, not a private code fork. Vulnerability reporting has a private channel and coordinated-disclosure policy.

---

## 18. Build strategy

### 18.1 Team assumption

The recommended plan assumes:

- **Engineer A — protocol/control plane:** schemas, API, PostgreSQL, ordering, milestone engine;
- **Engineer B — execution security:** canonical artifacts, preflight builder, Firecracker runner, conformance;
- **Engineer C — product/full stack:** Next.js, creator/solver flows, GitHub identity, editorial tools;
- **Engineer D — platform/infrastructure:** cloud/KMS, runner operations, capacity admission, observability, recovery, and security hardening;
- **Founder/product lead:** protocol decisions, challenge recruitment, UX acceptance; and
- part-time scientific editor plus external runner/security review. Payment/legal specialists join before the monetary fast-follows.

The workstreams overlap, but security-sensitive code receives cross-review. Engineer A does not unilaterally ship adjudication, and Engineer D does not unilaterally ship runner policy or key-management changes.

### 18.2 Delivery principles

1. Build a thin vertical slice before broad marketplace UI.
2. Keep one reference challenge in CI from the first week.
3. Develop deterministic fakes for GitHub, LLM, storage, and runner boundaries before live dependencies.
4. Treat schemas, receipts, and invariants as code, not prose.
5. Land dangerous capabilities behind feature flags and environment allowlists.
6. Do not accept public competitive validators before external execution review.
7. Convert every repeated manual MVP intervention into a protocol check, template, or runbook.

---

## 19. Phased build plan

The weeks below are elapsed calendar time for a four-engineer team and intentionally overlap.

### Phase 0 — Ratify contracts and threats (weeks 1–2)

**Build**

- Publish monorepo, contribution policy, licenses, CI, local Compose skeleton, and ADR template.
- Publish candidate versions of `artifact-checker-v1`, canonical tree rules, candidate schema, manifest schema, typed result schema, and initial resource profile; do not freeze them before real challenge and guest spikes.
- Define the signed receipt suite and offline verifier behavior.
- Specify score ticks, tolerance, milestone tiers, acceptance sequence, ordered watermark, and claim state machines. Freeze `economicMode: none` and the inert `milestone.claimed.v1` future-extension envelope.
- Write the initial threat model and data classification.
- Model milestone/adjudication invariants in TLA+/PlusCal.
- Create fake GitHub, LLM, storage, and runner adapters.
- Run an AWS infrastructure spike on the exact proposed bare-metal instance family: prove `/dev/kvm`, the production Firecracker/Jailer build, signed prebuilt-disk attachment, per-host identity, failure-domain labels, and capacity for three anti-affined hosts; record measured startup/cost and reserve dependable MVP capacity.
- Book the independent runner review now, with time reserved for remediation before launch.
- Assign Engineer A to the open audit-witness protocol/binary and the founder to recruit three independent operators; freeze the 2-of-3 quorum, bootstrap material, cadence, and outage/pause policy.

**Exit gate**

- The same challenge, artifact, manifest, and receipt test vectors produce identical digests in CLI and server tests.
- Model checking finds no double milestone claim or out-of-order winner across the bounded state space.
- The runner spike names a viable instance family/AMI and proves the three-host topology; otherwise the schedule is replanned before product work depends on it.
- The team has approved the MVP cuts listed in section 23.

### Phase 1 — Open local protocol loop (weeks 1–4)

**Build**

- CLI commands: `scout-prompt`, `candidate lint`, `challenge init`, `challenge lint`, `challenge test`, `validate --local`, `submit --dry-run`, and `receipt verify`.
- Python artifact-checker template with locked dependencies and fixed result contract.
- Canonical artifact packer/unpacker and malicious archive corpus.
- Public fixture runner, exact score normalizer, milestone-ladder simulator, and unsigned local report.
- Two artifact reference challenges: one certificate/proof-style and one structured optimization artifact.
- Versioned Challenge Scout prompt and candidate examples, including truthful abstention cases.

**Exit gate**

- A developer unfamiliar with the codebase can generate a candidate, scaffold a challenge, reproduce its baseline, test a solver artifact, simulate milestones, and verify a receipt using only public documentation.
- Linux CI and macOS emulation agree on all protocol semantics and visibly disclose the execution-profile mismatch.
- Both reference challenges and the platform-owned guest/init skeleton pass; only then is the MVP profile ratified and frozen.

### Phase 2 — Hosted control-plane slice (weeks 2–6, parallel)

**Build**

- PostgreSQL migrations, typed queries, outbox, job system, and audit projection.
- GitHub sign-in, CLI authorization, GitHub App installation, webhook verification, and immutable repository snapshotting.
- Signed/versioned Challenge Scout prompt retrieval, Create-page prefill/copy flow, candidate import, creator draft, challenge detail, submission status, and minimal editor UI.
- S3-compatible CAS, private GitHub snapshot ingestion, digest verification, and OCI registry integration.
- OpenAPI contract and generated CLI/web clients where useful.

**Exit gate**

- One reference repository can become a hosted draft and render a complete challenge page from immutable stored state.
- Duplicate webhooks, API retries, and worker crashes produce no duplicate domain transitions.

### Phase 3 — Secure preflight and official runner (weeks 3–9)

**Build**

- Quarantine builder with controlled dependency proxy, offline rebuild, SBOM, license/security scans, and image locking.
- Automated fixture matrix, fuzz/adversarial cases, hidden-suite sealing, baseline reproducibility, final validator-disk construction, and `MachineConformanceReceipt`.
- Dedicated runner account, host images, Jailer/seccomp/cgroup policy, no-NIC guests, immutable disks, result framing, and signed run receipts.
- Runner gateway with signed jobs, exact-object grants, mTLS, one-use result capabilities, and host anti-affinity.
- Runner implementation epoch metadata and equivalence corpus.

**Exit gate**

- A competitive run executes on a disposable microVM with no network route or platform secret.
- The hostile corpus terminates in typed bounded outcomes; no path traversal, host mount, credential read, or unbounded output succeeds.
- Golden artifacts reproduce across at least three official hosts.
- A `MachineConformanceReceipt` can be independently verified from exported build and executable objects.

### Phase 4 — Challenge review and publication (weeks 5–10)

**Build**

- Source resolution and evidence capture for supported paper identifiers/URLs.
- Structured LLM legibility/proxy reviewer behind a replaceable provider adapter.
- Separate executable, scientific, rights, safety, and competition-contract review results.
- Final `ChallengeLockReceipt` assembly only after the machine receipt and all immutable review reports exist.
- Human-review queue, reviewer notes, Human-reviewed/Featured labels, flagging, pause, compromise, and supersession.
- Free, quota-issued `ValidationGrant` for the immutable source snapshot; no payment boundary or service order.
- Immutable publication transaction and public export bundle.

**Exit gate**

- An external creator can import a Scout candidate, correct it, publish a qualifying challenge without a Science Ladder code change, and receive a signed lock receipt.
- The system rejects or routes underspecified, unsafe, nondeterministic, rights-incompatible, and non-machine-verifiable candidates instead of forcing them live.

### Phase 5 — Official submission and ordered milestones (weeks 6–11)

**Build**

- Private GitHub commit resolution, canonical snapshot ingestion, trusted quarantine construction of the read-only submission disk, structural validation, duplicate detection, and parent-frontier capture.
- Free subject-bound validation grants plus exact capacity admission.
- Atomic acceptance/sequence receipt and concurrent official scheduling.
- Two-host confirmation, score normalization, terminal failure taxonomy, ordered watermark, verified-best/public-frontier split, and immediate winning-artifact publication.
- Atomic all-crossed milestone claims, adjudication receipt, inert future-extension event, and public contribution record.
- Guest-semantic version migration logic for still-open milestones; full operational migration drill in Phase 6.

**Exit gate**

- Randomized test runs that duplicate and arbitrarily reorder queue messages always select the same earliest qualifying receipt.
- Killing API, worker, runner, or adjudicator processes at every persistence boundary cannot duplicate a receipt, frontier event, or milestone claim.
- A private loser remains private; a milestone-winning public-frontier artifact becomes public with reproducible receipts.

### Phase 6 — Product completion and MVP hardening (weeks 9–14)

**Build**

- Challenge discovery, readable benchmark pages, copyable solver-agent prompt, leaderboard distinctions, profiles, milestone status, and contribution lineage.
- Editor/operator consoles that expose state without exposing secrets or private artifacts.
- Free validation-grant issuance, rate limits, quotas, admission control, retention jobs, backups, recovery, dashboards, alerts, and required runbooks.
- Audit checkpoints, key rotation, public exports, and independent reproduction workflow.
- Deploy three independent audit witnesses and exercise quorum loss, fork detection, bootstrap, and recovery.
- Accessibility, browser support, error recovery, support workflows, and analytics that respect the no-trace default.
- External execution/security review and remediation.
- Guest/profile migration, restore, key-rotation, challenge-pause, validation-grant restoration, and hidden-suite incident drills.
- Infrastructure assertion that no wallet, Nostr-relay, payment-destination, payout, or Stripe secret/service exists in the MVP deployment.

**Exit gate**

- No open critical/high runner or platform-security finding.
- Every required operational and failure drill has succeeded.
- The checkpoint chain has three independent operators, reaches 2-of-3 quorum, and automatically pauses new acceptance after the tested outage grace period.
- Operators can run the pilot from documented procedures without direct database edits.
- The payment-free software is code-complete and security-approved for the deliberately capped cohort.

### Phase 7 — Invitation-only payment-free pilot (outcome-driven duration)

**Operate and learn**

- Invite three to five external creators and a capped solver cohort.
- Run real scientific challenges with platform-funded validation grants and no monetary reward or submission fee.
- Measure creator autonomy, manual interventions, validation delivery, confirmation agreement, milestone claims, frontier advances, and reproduction.
- Convert recurring staff work into protocol rules, tooling, templates, or documented editorial judgment.

**Exit gate**

- All PRD invitation-pilot exit criteria pass, including external challenge creation, submission volume, milestone/frontier advances, and third-party reproduction.
- The exit date is determined by evidence, not forced into the software calendar.

### Fast-follow A — Bitcoin reward settlement (5–7 additional weeks)

Engineering may begin after M5, in parallel with the payment-free invitation pilot. No monetary capability activates before M6 and the rail-specific interoperability, security, and legal gates below.

**Build**

- New payment-enabled seasons, immutable reward programs, funding authorizations, atomic reward entitlements, and public payout/default history.
- Versioned `lightning_address` and `bolt12_offer` destinations, a reserved-disabled `bip353_name` type, and the isolated BIP-321 payment-instruction resolver.
- App-initiated wallet connection using a pinned NWC-08 revision, encrypted client-key storage, approved relay proxy, and narrow payout executor.
- NWC-321 `pay`, persisted exact request events, wallet transaction-ID reconciliation, ambiguous-state recovery, and creator-default enforcement.
- Wallet conformance matrix, payment threat model, legal review, and independent security review.

**Exit gate**

- NWC-08 is merged or an audited pinned revision is explicitly accepted, and the chosen wallets pass required capability and authorization tests.
- Direct BOLT 12 and Lightning-address paths complete the certified wallet crash/replay matrix without an observed duplicate settlement, and every ambiguous attempt reconciles before replacement; BIP-353 remains disabled until its provenance gap has an interoperable solution.
- Optional payment hash, preimage, fee, and payer-proof fields are handled correctly; settlement never depends on one being present.
- Revoked authority creates the specified owed/default/cure history without reopening a funded milestone.

### Fast-follow B — Stripe validation billing (2–3 additional weeks, independently schedulable)

Engineering may begin after M5 and run alongside either the invitation pilot or the Bitcoin work. Production activation waits for M6 plus the processor and legal gates below.

**Build**

- Subject-bound service orders, Stripe Checkout/Link, verified webhooks, provider reconciliation, refunds, disputes, and grant issuance.
- Processor/legal approval, billing observability, and isolation from reward and adjudication state.

**Exit gate**

- A reconciled service order issues exactly one correctly bound `ValidationGrant`.
- Duplicate, delayed, and out-of-order events cannot duplicate service access, and disputes cannot rewrite scientific or reward records.
- Written approval covers the complete use case before production activation.

---

## 20. Milestones and dependency graph

| Milestone | Target | Demonstration | Depends on |
|---|---:|---|---|
| M0 — Protocol candidate | End week 2 | Candidate and challenge schemas, test vectors, threat model, checked receipt-ordering model | Founder decisions |
| M1 — Local scientific loop | End week 4 | Two artifact/checker reference challenges scaffold, lint, simulate, and validate locally | M0 |
| M2 — Hosted challenge preview | Weeks 5–6 | GitHub repository becomes an immutable staging page with automated review output | M1; control-plane slice |
| D1 — Controlled hosted demo | Weeks 6–8 | An invited creator imports a candidate and a solver submits to a pre-reviewed reference challenge through the visible staging flow | M2; early runner slice |
| M3 — Official isolated score | Weeks 8–9 | A privately submitted artifact produces a signed result in the fixed Firecracker profile | M1; runner track |
| M4 — Deterministic milestone claim | Weeks 10–11 | Arbitrarily reordered parallel runs always assign each crossed milestone to the correct receipt | M3; adjudication |
| M5 — Security-approved payment-free MVP | Weeks 12–14 | Product, operations, recovery, and runner security gates pass for the capped cohort | M2–M4; hardening |
| M6 — Invitation-pilot exit | Outcome-driven | External creators and solvers satisfy the PRD invitation-pilot exit criteria | M5; live evidence |

Bitcoin settlement and Stripe billing have their own post-MVP milestones. Neither is a dependency of M0–M6.

### Work that can run in parallel

- Product UI and GitHub identity can proceed once core schemas stabilize.
- Firecracker infrastructure can proceed against signed fixture jobs.
- Challenge Scout evaluation can proceed against the candidate schema.
- Editorial and challenge pages can use reference fixtures before the official runner is complete.
- Observability, runbooks, and hostile-corpus development can begin alongside the runner.

### Work that cannot be safely skipped or reordered

- Canonicalization before content-addressed receipts.
- Execution contract before production runner images.
- Capacity reservation and acceptance ordering before competitive submissions.
- Deterministic confirmation before milestone claims.
- Failure injection and external runner review before the invitation pilot.
- A prospective `RewardProgram` before any future monetary entitlement.
- NWC conformance and reconciliation tests before any real wallet authority.

---

## 21. Verification plan

### 21.1 MVP test layers

**Protocol conformance**

- Golden valid and invalid candidate, manifest, validator-result, and receipt fixtures.
- Cross-process stable hashes and signatures.
- Schema migration and forward/backward compatibility tests.
- Offline verification after signing-key rotation.
- Proof that an `economicMode: none` receipt cannot acquire reward semantics through configuration or replay.

**Property and model tests**

- Canonical tree invariance and rejection properties.
- Arbitrary-precision score and tolerance edge cases.
- Strict receipt ordering and all-crossed milestone arithmetic.
- Milestone migration only after the old version's accepted-receipt set is fully drained; rejection of every old/new race.
- TLA+/PlusCal model for the adjudication watermark and milestone uniqueness.

**Integration tests**

- GitHub webhook replay and signature failures.
- Upload interrupted at every chunk and seal stage.
- Transactional-outbox crash and replay.
- Validation-grant reservation, exhaustion, restoration after platform failure, and rejection after consumption.
- Source snapshot changes between intent, fetch, and acceptance.

**Runner security tests**

- Path traversal, link/device files, Unicode collisions, and decompression bombs.
- Fork, memory, CPU, disk, file-count, and output floods.
- Metadata, DNS, Internet, host-mount, socket, credential, and cross-run access attempts.
- Invalid UTF-8, duplicate JSON keys, huge exponents, non-finite values, and unknown fields.
- Guest crash, kernel panic, host termination, stale job, and duplicate result upload.
- Hidden bytes placed as canaries and searched across all solver-visible outputs.

**Concurrency and chaos tests**

- Random queue duplication and reordering.
- Worker/process termination before and after every durable write.
- Abandoned capacity/attempt lease recovery and rejection of every stale fencing token.
- Two adjudicators racing one challenge.
- Database failover during receipt assignment and milestone claim.
- Confirmation hosts failing or disagreeing.
- Runner-epoch migration with submissions in flight.

**End-to-end and usability**

- An external creator completes Scout-candidate import through publication without staff code changes.
- An external solver copies the challenge prompt, works locally, submits, and understands each status.
- A third party reproduces a milestone-winning public-frontier result from exported material.
- An editor flags, pauses, features, compromises, and supersedes without changing history.
- The deployed MVP exposes no wallet, payment-destination, payout, relay, checkout, or billing capability.

### 21.2 MVP launch gates

Before the invitation-only payment-free pilot:

- external review of the runner host, guest contract, artifact parser, and hidden-suite path;
- no unresolved critical or high security findings;
- verified backups and restore;
- receipt-key rotation and offline-validation drill;
- sandbox-escape response drill;
- challenge-pause, runner-epoch migration, and validation-grant restoration drills;
- explicit caps on creators, challenges, concurrent runs, validation grants, and resource classes; and
- an infrastructure assertion that payment services, routes, roles, and credentials are absent.

If schedule pressure arises, cut product breadth or defer restricted-program work. Do not cut isolation, deterministic arithmetic, receipt ordering, confirmation, failure recovery, or signed audit history.

### 21.3 Bitcoin fast-follow tests and gates

These tests are not MVP launch gates. They become mandatory before a payment-enabled season can open:

- conformance to pinned NWC-08 and NWC-321 revisions, NIP-44 v2 vectors, and the negotiated wallet capabilities;
- exact state, origin, wallet identity, signature, relay, expiry, network, and granted-method checks for app-initiated connection;
- BIP-321 instruction parsing with explicit rejection of `pop` and `req-pop` callbacks;
- direct BOLT 12 and Lightning-address success, fixed/amountless offer handling, expiry, wrong network, and wrong amount;
- local DNSSEC validation, CNAME/DNAME handling, proof retention, ambiguity, expiry, and rebinding tests for BIP-353;
- production disablement of BIP-353-over-NWC until the original-name provenance requirement is interoperably solved;
- spoofed, wrongly signed, wrong-author, wrong-kind, wrong-reference, expired, and miscorrelated NWC responses;
- loss of the first relay response after wallet execution, replay of the exact persisted event, and authenticated correlation by request-event ID or another certified wallet-defined key before `transaction_id` is known and before any replacement;
- correct behavior when payment hash, preimage, fees, or payer proof is absent;
- revoked, expired, underfunded, and method-restricted creator authority;
- connection replacement during `payment_unknown` and destination change after acceptance;
- secret-canary scans across logs, traces, crash dumps, analytics, and support surfaces;
- external review of entitlement creation, resolver egress, wallet-key isolation, payment reconciliation, and default handling; and
- legal and jurisdictional approval for the reward program.

### 21.4 Stripe fast-follow tests and gates

These are independently schedulable and do not block Bitcoin settlement:

- written processor approval for the complete validation-service and reward context;
- verified webhook signatures and authoritative provider-state reconciliation;
- duplicate, delayed, and out-of-order event handling at both provider-event and semantic-transition levels;
- stable checkout, fulfillment, refund, and retry idempotency keys;
- exactly one correctly bound `ValidationGrant` per fulfilled `ServiceOrder`;
- platform non-delivery, refund, dispute, and chargeback tests; and
- proof that billing state cannot change a score, receipt sequence, milestone claim, reward entitlement, or payout.

---

## 22. First two sprints

### Sprint 1

1. Create the public monorepo, CI, license split, ADR index, and contribution/security policies.
2. Land `ChallengeCandidate`, `ChallengeManifest`, typed validator-result, milestone, and canonical-digest schemas.
3. Implement canonical JSON and canonical artifact-tree libraries with test vectors.
4. Implement exact decimal-to-tick normalization and milestone simulation.
5. Draft the machine-conformance, challenge-lock, acceptance, validation, adjudication, milestone-claim, and audit receipt envelopes.
6. Write the initial TLA+/PlusCal receipt-ordering model.
7. Scaffold the web app, Go API, PostgreSQL migrations, outbox, and fixture adapters.
8. Choose the first two reference scientific challenges and verify that both fit artifact/checker semantics.
9. Prove the intended AWS bare-metal runner family/AMI and three-host anti-affinity topology.
10. Add an architecture test asserting that MVP packages and infrastructure contain no payment, wallet, relay, or billing dependencies.

**Sprint 1 review:** demonstrate identical digests in CLI and server, a model-checked earliest-milestone example, two viable reference contracts, and a viable runner-host spike.

### Sprint 2

1. Ship CLI scaffold, lint, `challenge test`, local-validate, milestone-simulate, and receipt-verify commands.
2. Ship the Python artifact-checker template and platform-owned build recipe.
3. Implement safe GitHub snapshot canonicalization and local public-suite execution.
4. Add GitHub sign-in/App installation and immutable source snapshotting.
5. Render the first challenge page from a real repository snapshot.
6. Run the Challenge Scout against the curated viable, unsuitable, and adversarial paper set.
7. Stand up a Linux Firecracker development host and execute the first fixed guest contract.
8. Implement free subject-bound validation grants and an early quota/admission test harness.

**Sprint 2 review:** an outside developer takes one paper-grounded candidate through a complete local creator and solver loop, while the official-runner skeleton scores the same golden artifact on Linux.

---

## 23. Explicit MVP cuts

These cuts are architectural protections, not missing polish:

- Artifact/checker challenges only; submitted programs move to `restricted-program-v1`.
- One platform-owned Python-oriented validator build profile; no arbitrary Dockerfile or shell setup command.
- Linux `amd64`, CPU-only, fixed thread count; no speed-based competition metric.
- Public challenge repositories without submodules, unresolved LFS, symlinks, or special files.
- Exact commits fetched from App-authorized public or private GitHub repositories; losing branches need not become public.
- Fixed resource classes and declared maximum artifact sizes.
- One LLM review-provider adapter, with model and version recorded.
- Free, capped, subject-bound validation grants; no checkout or transferable credit.
- No reward amount, monetary entitlement, payout destination, wallet connection, relay egress, creator-default state, or payment-reliability score.
- No Stripe/Link, Bitcoin service billing, refund, dispute, or chargeback subsystem.
- One cloud and one active region.
- PostgreSQL jobs; no Kafka, Redis, or workflow engine.
- REST/SSE; no GraphQL or WebSockets.
- Signed audit events; no blockchain token or on-chain registry.
- No teams, funder pools, validator marketplace, or appeals that alter deterministic results.
- Challenge Scout creates drafts only; no continuous crawler and no automatic publication.

---

## 24. Principal risks and responses

### MVP risks

| Risk | Consequence | Design response |
|---|---|---|
| Challenge validators are harder to secure than expected | Pilot delay or host compromise | Artifact-only profile, controlled builder, Firecracker, hostile corpus, external review, narrow initial language profile |
| “Deterministic” science workloads vary across machines | False winner or blocked adjudication | One execution profile, exact ticks, anti-affined confirmation, conservative score rule, fail closed, exclude GPUs and performance timing |
| Hidden suites become an extraction target | Benchmark gaming or scientific invalidity | Precommitment, encryption, no solver code, coarse output, quota controls, eventual reveal, trust label |
| Early receipt blocks later results after a platform fault | Challenge appears stuck | Capacity admission, bounded retries on distinct hosts, incident pause/runbook; never sacrifice ordering fairness |
| Free hosted validation is abused | Runaway cost or degraded service | Invitation-only cohort, subject-bound grants, per-account/challenge/resource quotas, concurrency caps, duplicate detection, global kill switch |
| Permissionless challenge quality is poor | Marketplace floods with weak proxies | Mandatory evidence, objective-contract checks, structured LLM review, flags, trust labels, and featuring rather than universal editorial approval |
| Manual review becomes the bottleneck | Fails the permissionless thesis | Instrument interventions, turn repeated cases into schemas/templates/tests, reserve humans for borderline judgment |
| Open source is easy to fork before network effects form | Commercial competition | Make protocol credibility, reputation, discovery, reliable execution, and contribution history the moat—not code secrecy |
| Four-engineer schedule is too aggressive | Unsafe shortcuts | Maintain feature gates and explicit cuts; extend the timeline rather than weakening the runner, ordering, or audit invariants |

### Future payment-activation risks

| Risk | Consequence | Design response |
|---|---|---|
| Creator wallet cannot pay an earned entitlement | Solver distrust | Preserve the obligation, reconcile automatically, publish immutable default/cure history, suspend and reputation-penalize the creator |
| Draft or uneven NWC support | Ambiguous payments or narrow compatibility | Pin reviewed revisions, capability-gate, certify a small wallet matrix, persist requests before send, reconcile before replacement |
| BIP-353 provenance is lost at the NWC boundary | Invalid BOLT 12 invoice request | Keep the destination type in the schema but disable this route until an interoperable solution exists |
| Stripe rejects or restricts the validation-service model | Card rail unavailable | Treat cards as an independent optional capability and require written approval before activation |

---

## 25. Post-MVP architecture

### 25.1 `restricted-program-v1`

Do not place solver code and the hidden judge in one guest. Use two sibling microVMs:

```text
solver VM (submitted program, no suite)
          ⇅
bounded host broker
          ⇅
judge VM (creator validator + hidden suite)
```

The broker exposes a length-prefixed protocol with message-size, message-count, test-count, CPU, memory, and wall-time limits. Neither VM has a NIC or shared filesystem. The judge decides what query/response information leaves its boundary. This profile needs its own threat review, conformance corpus, execution digest, and public template.

### 25.2 Independent validators and federation

Signed jobs, content-addressed inputs, portable receipts, and public conformance tests let third parties reproduce a run or operate a compatible validator later. The next trust step should be optional independent-reproduction receipts, followed by quorum policies for selected challenge classes. Do not design a token or consensus system before real disagreements reveal what governance is needed.

### 25.3 Continuous challenge discovery

The crawler is another `ChallengeCandidate` producer:

```text
source adapters
→ rights-aware document store
→ open-question extraction
→ citation/evidence resolver
→ duplicate/topic clustering
→ impact, evidence, tractability, safety, and validation-readiness axes
→ ChallengeCandidate drafts
→ named creator adoption
```

Ranking axes remain separately visible; avoid one opaque importance score. The crawler never publishes a live challenge or activates a reward program.

### 25.4 Web-lab and real-world science

Real-world work is a new `oracle` profile, not a looser container validator. It will require preregistered methods, ethics/safety review, sample identity and chain of custody, signed observations, independent labs, replication thresholds, delayed finality windows, conflicts policy, and stronger assurance for any optional reward program. The current protocol's versioned manifests, evidence, milestone claims, receipts, and audit objects remain reusable.

---

## 26. Founder decisions to ratify

The architecture recommends approving these as a set:

1. The MVP is payment-free and supports `artifact-checker-v1` challenges only.
2. Control plane, CLI, runner, and protocol library use Go; the web app uses Next.js/TypeScript.
3. The MVP control plane is a modular monolith with separate builder and runner processes only where hostile-code isolation requires them.
4. PostgreSQL is the transactional authority and initial job queue.
5. Official MVP execution is Firecracker on one Linux `amd64` profile with no guest NIC.
6. Solver submissions are exact commits independently fetched from App-authorized public or private GitHub repositories; losing work remains private by default.
7. `verified best` and `public frontier` are distinct; every milestone-winning public-frontier artifact is published immediately.
8. First-to-threshold adjudication uses atomic receipt sequences and integer score ticks; the MVP records `MilestoneClaim`, not `RewardEntitlement`.
9. The MVP uses free, capped, subject-bound `ValidationGrant` records and deploys no wallet, payout, relay, checkout, or billing component.
10. AWS is the first hosted environment, in one region and without Kubernetes.
11. The four-engineer target is a controlled hosted demo in weeks 6–8 and a hardened payment-free candidate in weeks 12–14, followed by an outcome-driven pilot.
12. Bitcoin rewards are a 5–7 week fast-follow using app-initiated NWC-08 and NWC-321 `pay` with BIP-321 instructions. Engineering may begin after M5; activation waits for M6 plus compatibility, security, and legal gates.
13. Solver payout destinations in that fast-follow are tagged Lightning address or direct BOLT 12 offer, with a reserved-disabled BIP-353 name type until its provenance gap is resolved.
14. Stripe Checkout/Link billing is an independent 2–3 week fast-follow. Engineering may begin after M5; activation waits for M6 and written processor approval. No Bitcoin service-billing provider is selected.

If these decisions are accepted, implementation can begin with section 22. Payment work is deliberately absent from those sprints and from every MVP deployment.

---

## 27. Primary technical references

### MVP platform

- [Firecracker design](https://github.com/firecracker-microvm/firecracker/blob/main/docs/design.md)
- [Firecracker production host setup](https://github.com/firecracker-microvm/firecracker/blob/main/docs/prod-host-setup.md)
- [Firecracker Jailer](https://github.com/firecracker-microvm/firecracker/blob/main/docs/jailer.md)
- [Firecracker Go SDK](https://github.com/firecracker-microvm/firecracker-go-sdk)
- [OCI image specification](https://github.com/opencontainers/image-spec/blob/main/spec.md)
- [JSON Schema specification](https://json-schema.org/specification)
- [RFC 8785: JSON Canonicalization Scheme](https://www.rfc-editor.org/rfc/rfc8785.html)
- [in-toto attestation envelope specification](https://github.com/in-toto/attestation/blob/main/spec/v1/envelope.md)
- [AWS KMS asymmetric signing and signature encoding](https://docs.aws.amazon.com/kms/latest/APIReference/API_Sign.html)
- [PostgreSQL `SELECT` locking and `SKIP LOCKED`](https://www.postgresql.org/docs/current/sql-select.html)
- [River transactional PostgreSQL job queue](https://github.com/riverqueue/river)
- [GitHub App permissions](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/choosing-permissions-for-a-github-app)
- [GitHub webhook delivery validation](https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries)

### Post-MVP rewards and billing

- [NIP-47: Nostr Wallet Connect](https://github.com/nostr-protocol/nips/blob/master/47.md)
- [NIP-44: encrypted payloads](https://github.com/nostr-protocol/nips/blob/master/44.md)
- [NWC-321 BIP-321 payment-method specification](https://github.com/nostr-wallet-connect/nwc/blob/main/321.md)
- [NWC-321 merge discussion](https://github.com/nostr-wallet-connect/nwc/pull/2)
- [NWC-08 client-initiated connection proposal](https://github.com/nostr-wallet-connect/nwc/pull/3)
- [BIP-321 payment URI scheme](https://github.com/bitcoin/bips/blob/master/bip-0321.mediawiki)
- [BIP-353 DNS payment instructions](https://github.com/bitcoin/bips/blob/master/bip-0353.mediawiki)
- [BOLT 12 offers and invoice requests](https://github.com/lightning/bolts/blob/master/12-offer-encoding.md)
- [LNURL-pay specification](https://github.com/lnurl/luds/blob/luds/06.md)
- [BOLT 11 invoice encoding](https://github.com/lightning/bolts/blob/master/11-payment-encoding.md)
- [Stripe Checkout fulfillment](https://docs.stripe.com/checkout/fulfillment)
- [Stripe webhook handling](https://docs.stripe.com/webhooks)
