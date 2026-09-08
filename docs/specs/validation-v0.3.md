# Validation framework extension requirements

7 September 2026. Companion to the [Yukon compatibility audit](../research/yukon-validation-audit-2026-09-07.md) and [scientific metric policy](../scientific-metric-policy.md).

**Status:** implementation in progress. The source now includes explicitly versioned v2 measurement contracts, exact achievement predicates, scientific-metric review gates and paired-timing arithmetic, with regression tests. Native execution and trusted-timing integration are still being built; the deployed API remains unchanged, with `artifact-checker-v1` as its only execution profile. The profile families below remain acceptance targets until their hosted conformance tests pass. Do not weaken the existing profile or change published locks to simulate broader support.

## Objective

Accommodate all validation designs observed in the public Yukon registry while requiring scores to support meaningful scientific or applied-science claims. Preserve the existing control plane, immutable inputs, isolated execution, exact arithmetic, signed receipts and ordered publication. Separate three decisions:

1. Can the platform safely execute and check this contract?
2. Does the measurement reliably support its stated score?
3. Does that score support the stated scientific contribution?

A positive answer to one does not answer the others.

## Validation profiles

| Target capability | Input and evaluation | Yukon coverage | Current state |
| --- | --- | --- | --- |
| Data artifact checking | Immutable data plus a reviewed deterministic checker | Quiet Echoes; a bounded census-only Heesch adaptation | Existing profile; adaptations still need their own checker and review |
| Proof checking | Proof source/certificate, fixed statement, pinned elaborator and trusted proof checker; separately derived objective | Full Heesch, Proximity, MODEXP and RIPEMD-160 | Needs a new reviewed profile and toolchains |
| Program evaluation | Build submitted source in isolation; feed cases to an untrusted program; independently validate its outputs and compute objective | ECDSA circuit construction, sparse ordering, all eight OpenFrontierCS tasks | Needs a new profile and execution boundary |
| Measured performance | Program evaluation plus trusted timing, matched hardware, repeat schedule, workload/quality gates and statistical decision rule | Flock x86/Apple, Lighter, current Gemma MLX and retired MLX/Qwen contracts | Needs a new measurement policy and hardware-specific executors |

A proof-checked performance challenge composes capabilities; it must not choose between correctness and speed. Language is a toolchain property, not a scientific category. Do not require every challenge to be rewritten into Python.

## Frozen evaluation pipeline

The next protocol must bind an ordered, bounded evaluation plan into the challenge lock:

- **Inputs:** submitted paths and payload kind; source commit; suite/dataset commitments; model weights; baseline implementation; toolchain and evaluator digests; legal availability and size limits.
- **Stages:** preparation/build, candidate execution, output capture, independent correctness/proof checking, measurement, aggregation, then adjudication. Declare allowed inputs, outputs, privileges, resource limits and ownership for each stage.
- **Authorities:** untrusted candidate code may generate results but cannot choose the cases, modify the reference checker, control a trusted timer, validate its own proof, or write the authoritative score. Freeze trusted code separately even when the candidate repository contains files with the same names.
- **Binding:** a proof, benchmark output and score must all refer to the identical captured candidate bytes. Bind theorem parameters, circuit statements, public outputs, model/checkpoint identity and workload version where applicable. Reject cross-track or stale-version artifacts.
- **Failures:** distinguish invalid candidate, disproved claim, inconclusive proof check, exhausted resource budget, unavailable executor and unstable measurement. None is a fabricated valid score. A declared per-case penalty may contribute to an aggregate, but must retain the failure outcome and cannot earn a universal-correctness milestone.

Treat builds as execution: Rust build scripts/procedural macros, Lean elaboration, package hooks and compiler plugins belong inside the untrusted boundary. Dependency acquisition and compilation are separate bounded phases; candidate build steps receive no platform secrets. Candidate programs and proof elaborators do not run in the API or in the trusted evaluator’s process.

The program profile needs isolated candidate and evaluator domains with a narrow, bounded I/O broker. They may be separate guests on one physical server; this requirement does not mandate a second physical machine. A hidden input deliberately sent to a solver is visible to that solver, while answer keys, unreleased future cases and score authority remain protected.

## Metrics, claims and comparison

Replace the single opaque scalar result with a versioned, schema-bound measurement record. Retain:

- named measurements with units and roles;
- exact integers/rationals where appropriate, or explicitly rounded decimals;
- case/cohort/series identity, raw objective and reference values;
- actual achieved properties and proof/correctness outcomes;
- uncertainty and accepted/rejected measurement counts;
- the reproducible derivation of the ranking key and milestone decision.

Keep integer ticks for scalar ranking. Add an explicit policy for any lexicographic ordering, justified composite or Pareto display. All weights, transforms, bounds, rounding directions and tie rules are frozen; diagnostics cannot silently become ranking inputs. Opposing Proximity tracks require separate directions and claims. Gemma batch and single-stream regimes require separate comparison identities. Sharing a repository or display unit does not make two scores comparable.

Store large evidence in immutable objects and reference its digest in the signed receipt. Hidden-suite records need a separately defined disclosure schema; adding a generic diagnostic JSON field must not expose private cases or create an arbitrary output channel.

A mathematical achievement predicate is separate from the search ranking key: for example, completed corona count plus valid non-tiling evidence determines a Heesch milestone, not a rounded fractional gradient. A timeout cannot be treated as a non-existence proof. A circuit passing sampled tests cannot be relabelled universally proved.

Every ranked metric must satisfy the scientific metric policy. New contracts must carry a structured rationale and a bound review decision. Initial completeness checks can require the argument and evidence fields; publication still requires resolution of substantive scientific findings.

## Measurement policies

Exact evaluation and noisy performance measurements need different confirmation policies. The current “two scores within tolerance; keep the conservative one” rule remains appropriate only within its declared deterministic/numeric scope. Increasing that tolerance does not implement statistical validation.

A measured-performance contract must predeclare:

- baseline/candidate pairing, order randomization or alternation, warm-up, measured repetitions, seeds and stopping rules;
- timer authority and boundaries, including synchronization, serialization, preprocessing and output capture;
- required correctness/quality/workload gates before any valid ranking;
- supported hardware, OS, accelerator, memory, threads, compiler and relevant operating-state controls;
- estimator, uncertainty procedure, practical improvement threshold and treatment of missing/failed trials;
- an explicit inconclusive result and bounded retest policy, including repeated-submission selection effects;
- exact population and operating regime to which the speed or cost claim applies.

Use measured calibration and workload knowledge to select the rule. Do not impose one universal percentage or copy Yukon's timing constants without validating them on the target host. Normalize to a matched baseline when justified, and preserve the raw measurements and regression constraints so aggregate wins cannot hide unacceptable losses.

## Runtime and resource capabilities

**Current delivery scope (user decision, 7 September 2026):** Implement and exercise Linux evaluation first. Defer the specific MLX benchmark, Apple executor implementation and Mac provisioning. Keep the execution interface independent of Linux/Firecracker so a later macOS/Metal adapter does not require changing challenge semantics, score authority, evidence bindings or comparison identities. A matching Apple executor is an optional future implementation, not a launch prerequisite. This scope change does not defer the Linux proof, submitted-program, measurement or scientific-metric gaps.

Executors advertise and attest reviewed capabilities. Scheduling must match the locked contract to those capabilities; an unavailable profile must be reported before reserving execution capacity or admitting a competitive challenge. CPU/GPU architecture, OS, device/driver, toolchains and memory are distinct from the verification policy and from scientific quality.

The existing Linux/amd64 Firecracker host can be a base for additional Linux profiles after implementation and conformance testing. It cannot supply macOS/Metal behavior. Hosting an Apple-specific benchmark may require Apple hardware; that is a workload requirement, not independent replication. Provision no additional hardware merely to satisfy a generic redundancy rule.

Resource envelopes must support separately budgeted build, proof, case and session stages, streamed large proofs, immutable pre-provisioned weights/corpora and per-object size controls. The present 64 MiB upload, 4-vCPU, 8-GiB, 600-second envelope does not cover the entire observed portfolio. Increasing limits is a reviewed profile change, not a creator-controlled bypass. Long computations need resumable orchestration and explicit evidence of completed checks, never assumed success.

Hidden suites retain commitments and an auditable release/access policy. Extend the current single canonical-JSON suite mechanism to versioned immutable bundles and challenge-derived fresh workloads. Bind randomness after candidate commitment when fresh challenges are necessary; make the seed provenance reproducible for authorized audit. Control feedback, query budgets and cumulative leakage. Encryption at rest does not itself protect tests from code that is allowed to read them.

## Compatibility and rollout

Preserve all v1 locks, scores, receipts and export verification. New fields and signatures require an explicitly versioned contract; old clients must reject unknown execution capabilities rather than downgrade. Do not add native extensions to the current allowlist and call the work complete. A renamed source file is still executable content when interpreted.

Existing users keep single-host `platform` verification. The optional `independent` policy retains its distinct meaning. Independent checker implementations or proof-kernel replay can run on the same host and are not evidence of physical replication.

Implementation order:

1. Structured metric rationale, typed result evidence and explicit comparison/achievement policies, with backward-compatible reads of existing records.
2. Native proof profile: statement/axiom/byte binding, Lean and certificate adapters, streaming proof budgets. This covers full Heesch and the two formal-proof families.
3. Isolated submitted-program profile: Rust/C++ builds, test-case broker and trusted output checking. This covers ECDSA, matrices and OpenFrontierCS.
4. Hardware-specific performance profiles and statistical adjudication. Add Apple execution only when an Apple challenge is to be hosted.

## Acceptance criteria before claiming support

A row in the audit becomes supported only when its actual validation design has a runnable adapter on an enrolled matching executor and passes both a representative valid case and adversarial cases. Importing metadata or parsing a manifest is not support.

Required conformance cases include: forged proof and added axioms; proof/byte mismatch; missing certificate and truncated large proof; wrong circuit/output; hidden-case precomputation; evaluator/timer/score-file writes; solver build hooks; invalid per-case output and non-finite measurements; unacceptable quality regression; wrong hardware or series; baseline drift and unstable timing; changed weights or aggregation; stale versions; and a higher-scoring but scientifically useless shortcut. Check exact raw-to-ranked arithmetic and actual milestone predicates.

Until these implementation and execution checks exist, describe the capability as planned. This document broadens the architecture requirements; it does not certify universal compatibility or Yukon's infrastructure.
