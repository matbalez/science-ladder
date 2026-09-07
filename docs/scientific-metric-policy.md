# Scientific meaning of scores

Requirement added 7 September 2026. Applies to the design and review of new challenges and new versions, including applied science and research engineering. It does not amend an existing locked challenge. This is an editorial requirement; the current API does not mechanically establish scientific relevance. Passing a checker establishes the checker’s bounded claim, not novelty or scientific importance.

## Publication rule

Every ranked metric must have an explicit, defensible connection to a stated scientific or applied-science objective. The creator must explain what an improvement establishes, provide evidence for that connection, and identify ways the metric could improve without useful progress. Correct arithmetic, a reproducible benchmark and an impressive leaderboard are insufficient on their own.

Useful progress may be a better construction, a tighter bound, an improved algorithm, a more accurate prediction, a more efficient implementation at preserved quality, or evidence against a hypothesis. Discovering a weakness can advance knowledge even when it moves away from a hoped-for practical outcome. A world record, a commercially deployed result, a recent publication date, or a second physical verification server is not required.

Use [the metric rationale template](templates/metric-rationale.md) in `challenge-brief.md`, which the existing scientific-review evidence selector already includes. The creator and scientific reviewer must address it before approving a new challenge. A later machine-readable policy must enforce completeness and record the review decision; it cannot automatically prove the scientific argument.

## Required argument

1. **Objective and scope.** State the question, object or population, assumptions, and intended contribution. Distinguish the exact mathematical problem from possible applications. A broad application story cannot substitute for a defined research question.
2. **Meaning of each metric.** Give the definition, units, direction, measurement procedure, and role: ranking, validity constraint, or diagnostic. Classify its interpretation as a direct objective, a proved bound, an empirically supported proxy, or a local search diagnostic. Declare the complete aggregation, including weights, denominators, clipping, missing cases and numeric precision. Explain why any trade-off is appropriate.
3. **Evidence linking improvement to progress.** Supply a mathematical argument, a documented engineering cost relationship, or empirical validation on the relevant task/population. A proxy need not correlate perfectly with the final goal, but its connection must withstand scrutiny. For new or uncertain proxies, provide supporting experiments or retain the score as a diagnostic until that case is made.
4. **Preserved conditions.** Identify what must not deteriorate: correctness, proof assumptions, predictive quality, coverage, resource budget, workload difficulty, or an explicit permitted trade-off. Validate those conditions separately. A speed improvement obtained by doing less useful work is not automatically progress.
5. **Baseline and meaningful change.** Explain the reference, raw values and smallest defensible improvement. For exact mathematics, one new feasible unit or a tighter bound can be meaningful. For measurements, justify the threshold against observed variation and practical effect; decimal precision and two close repeat scores are not statistical evidence. Preserve both the scientific reference and any launch or same-session calibration baseline.
6. **Counterexamples and generalization.** Attempt at least one concrete shortcut that raises the score without advancing the stated objective. Explain how the design rejects it, measures it, or narrows its claim. Consider memorization, test leakage, solver-selected cases, denominator changes, quality regressions, omitted failures and resource inflation. Generalization evidence is necessary when the claim is about an algorithm or population; it is not necessary for an explicitly fixed-instance construction claim.
7. **Permitted conclusion.** Write the strongest sentence a qualifying result would justify, and the stronger claims it would not. Separate “valid artifact,” “better benchmark result,” “scientific milestone,” and “new literature record.” A novelty check is needed for the last label, not for publishing a challenge inviting the community to improve a reference.

## Search gradients and multiple metrics

Intermediate scores are useful, but the platform must not misrepresent them as completed scientific achievements. Partial coverage, a surrogate loss, or a reduced residual can guide a search without proving that the next theorem, feasible design, or experiment is close. A local-search score may rank only a clearly stated, scientifically useful subproblem with a defensible rationale; otherwise show it as a diagnostic. Always retain the actual achieved property alongside it.

For multiple metrics, prefer a declared primary metric with separate hard constraints and visible diagnostics. Use a fixed composite only when its trade-offs are justified. If different contributions are inherently incomparable, use separate tracks or a Pareto view rather than inventing an “overall science” score. A monotone transformation of each case before aggregation can change the ordering across a test suite; treat that as part of the scientific contract, not merely chart styling.

Scores expressed as percentages must identify the denominator. Percentage improvement over a baseline is not percentage progress toward a discovery. Reaching a display cap is not proof of optimality. Avoid silently dropping negative results, difficult cases or outliers.

## Review outcomes

- **Accept the rationale:** the metric supports the bounded claim, its gates preserve necessary conditions, and its limitations are explicit. Other publication requirements still apply.
- **Needs work:** missing rationale, weak proxy evidence, unjustified trade-offs, or inadequate measurement design can be repaired. Require the repair before competitive publication.
- **Do not rank as scientific progress:** the number can be gamed without a defensible contribution, or the link to the stated objective is absent. The material may still be educational or useful as a noncompetitive diagnostic.

Do not resolve missing scientific evidence by substituting a model’s confidence, a creator’s enthusiasm or a generic “impact” paragraph. Automated review can identify omissions and contradictions; accountable review remains necessary where the claim is unresolved.

## Examples for reviewers

- **Quiet Echoes:** lower autocorrelation energy is a direct improvement to the defined length-512 optimization problem. It does not itself establish a better deployed radar or a world record. No changes to its existing lock or results are implied by this policy.
- **A partial tiling witness:** greater verified coverage is a narrower construction result; it is not a fractional proof that a shape has another complete corona.
- **A faster prover:** require the same statement, soundness assumptions, workload and successful proof verification. Hash-compression throughput is not automatically end-to-end transaction throughput.
- **Faster inference:** disclose permitted output differences and establish that the claimed quality is preserved. A token-difference budget alone is not evidence that application quality is unchanged.
- **Sparse ordering:** predicted factorization operations can support an algorithmic cost claim. End-to-end speed, memory and numerical stability need their own evidence.
- **Puzzle construction:** a verifiable score for clue patterns is not sufficient by itself. State the mathematical or algorithmic question and why the score advances it; otherwise present the puzzle as educational.

See the [Yukon audit](research/yukon-validation-audit-2026-09-07.md) for the source-backed comparisons motivating these requirements.
